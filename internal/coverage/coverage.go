package coverage

import (
	"context"
	"fmt"
	"maps"
	"math"
	"runtime"
	"slices"
	"sync"

	"github.com/radiopath/radiopath/internal/dem"
	"github.com/radiopath/radiopath/internal/geo"
	"github.com/radiopath/radiopath/internal/itm"
	"github.com/radiopath/radiopath/internal/link"
)

const (
	minRadials = 360
	maxRadials = 3600
	maxPixels  = 2000

	MinRangeM      = 500.0
	MaxRangeM      = 300e3
	MinResolutionM = 10.0
	MaxResolutionM = 1000.0
)

type Input struct {
	TX          link.Site
	Radio       link.Radio
	RxHeightM   float64
	RangeM      float64
	ResolutionM float64
}

type Result struct {
	W, H                     int
	Margin                   []float64
	North, South, East, West float64
	Radials, Truncated       int
	Errors                   []string
}

func (r Result) At(x, y int) float64 { return r.Margin[y*r.W+x] }

type Computer interface {
	Compute(ctx context.Context, in Input) (Result, error)
}

type Calculator struct {
	DEM dem.Source
}

func radialCount(in Input) int {
	n := int(math.Ceil(2 * math.Pi * in.RangeM / in.ResolutionM))
	return min(max(n, minRadials), maxRadials)
}

func (c Calculator) Compute(ctx context.Context, in Input) (Result, error) {
	var r Result
	if in.RangeM < MinRangeM || in.RangeM > MaxRangeM {
		return r, fmt.Errorf("coverage: range must be between %.0f and %.0f m", MinRangeM, MaxRangeM)
	}
	if in.ResolutionM < MinResolutionM || in.ResolutionM > MaxResolutionM {
		return r, fmt.Errorf("coverage: resolution must be between %.0f and %.0f m", MinResolutionM, MaxResolutionM)
	}
	side := int(math.Ceil(2 * in.RangeM / in.ResolutionM))
	if side > maxPixels {
		return r, fmt.Errorf("coverage: range/resolution gives %d pixels per side, max %d", side, maxPixels)
	}

	h0, err := c.DEM.Elevation(in.TX.Pos.Lat, in.TX.Pos.Lon)
	if err != nil {
		return r, fmt.Errorf("coverage: transmitter site: %w", err)
	}

	nSamples := int(math.Ceil(in.RangeM / in.ResolutionM))
	radials := radialCount(in)
	r.Radials = radials
	margins := make([][]float64, radials)
	errs := make([]error, radials)

	work := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < runtime.GOMAXPROCS(0); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range work {
				margins[i], errs[i] = c.radial(ctx, in, h0, float64(i)*360/float64(radials), nSamples)
			}
		}()
	}
	for i := 0; i < radials && ctx.Err() == nil; i++ {
		work <- i
	}
	close(work)
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return r, err
	}
	distinct := map[string]struct{}{}
	for _, e := range errs {
		if e != nil {
			r.Truncated++
			distinct[e.Error()] = struct{}{}
		}
	}
	r.Errors = slices.Sorted(maps.Keys(distinct))

	r.W, r.H = side, side
	dLat := in.RangeM / geo.EarthRadius * 180 / math.Pi
	dLon := dLat / math.Cos(in.TX.Pos.Lat*math.Pi/180)
	r.North, r.South = in.TX.Pos.Lat+dLat, in.TX.Pos.Lat-dLat
	r.West, r.East = in.TX.Pos.Lon-dLon, in.TX.Pos.Lon+dLon
	r.Margin = make([]float64, side*side)
	for y := 0; y < side; y++ {
		lat := r.North - (float64(y)+0.5)*2*dLat/float64(side)
		for x := 0; x < side; x++ {
			lon := r.West + (float64(x)+0.5)*2*dLon/float64(side)
			p := geo.Point{Lat: lat, Lon: lon}
			d := geo.Distance(in.TX.Pos, p)
			k := int(d/in.ResolutionM + 0.5)
			if k > nSamples {
				r.Margin[y*side+x] = math.NaN()
				continue
			}
			if k == 0 {
				k = 1
			}
			ri := int(geo.InitialBearing(in.TX.Pos, p)/(360/float64(radials))+0.5) % radials
			r.Margin[y*side+x] = margins[ri][k]
		}
	}
	return r, nil
}

func (c Calculator) radial(ctx context.Context, in Input, h0, bearing float64, n int) (out []float64, err error) {
	out = make([]float64, n+1)
	for i := range out {
		out[i] = math.NaN()
	}
	elev := make([]float64, 0, n+1)
	elev = append(elev, h0)
	params := itm.Params{
		HTxM: in.TX.AntennaHeightM, HRxM: in.RxHeightM,
		FreqMHz: in.Radio.FreqMHz, Polarization: in.Radio.Polarization,
		Epsilon: link.Epsilon, Sigma: link.Sigma, N0: link.N0, Climate: link.Climate, Mdvar: link.MdvarMobile,
		Time: link.TimePct, Location: link.LocPct, Situation: link.SitPct,
	}
	eirp := in.Radio.TxPowerDBm + in.Radio.TxGainDBi - in.Radio.TxLineLossDB
	azRel := bearing - in.Radio.TxAzimuthDeg
	for k := 1; k <= n; k++ {
		if ctx.Err() != nil {
			return out, nil
		}
		p := geo.Destination(in.TX.Pos, bearing, float64(k)*in.ResolutionM)
		h, err := c.DEM.Elevation(p.Lat, p.Lon)
		if err != nil {
			return out, err
		}
		elev = append(elev, h)
		res, err := itm.PointToPoint(elev, in.ResolutionM, params)
		if err != nil {
			continue
		}
		pattern := in.Radio.TxPattern.AttenuationDB(azRel, -res.TakeoffRad[0]*180/math.Pi-in.Radio.TxTiltDeg)
		out[k] = eirp - pattern - res.LossDB + in.Radio.RxGainDBi - in.Radio.RxLineLossDB - in.Radio.RxSensitivityDBm
	}
	return out, nil
}
