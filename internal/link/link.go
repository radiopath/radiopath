package link

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/radiopath/radiopath/internal/antenna"
	"github.com/radiopath/radiopath/internal/dem"
	"github.com/radiopath/radiopath/internal/geo"
	"github.com/radiopath/radiopath/internal/itm"
	"github.com/radiopath/radiopath/internal/metrics"
)

const (
	minStepM     = 10.0
	maxSamples   = 4000
	kFactor      = 4.0 / 3.0
	maxDistM     = 2000e3
	speedOfLight = 299792458.0
)

const (
	Climate = itm.ContinentalTemperate
	N0      = 301.0
	Epsilon = 15.0
	Sigma   = 0.005
	TimePct = 50.0
	LocPct  = 50.0
	SitPct  = 50.0

	MdvarFixed  = 11 // both ends fixed: location variability off
	MdvarMobile = 12

	ReliabilityPct = 99.0
)

type Site struct {
	Name           string
	Pos            geo.Point
	AntennaHeightM float64
}

type Radio struct {
	FreqMHz          float64
	TxPowerDBm       float64
	TxGainDBi        float64
	RxGainDBi        float64
	TxLineLossDB     float64
	RxLineLossDB     float64
	RxSensitivityDBm float64
	Polarization     itm.Polarization

	TxPattern    antenna.Lobe
	TxAzimuthDeg float64
	TxTiltDeg    float64
	RxPattern    antenna.Lobe
	RxAzimuthDeg float64
	RxTiltDeg    float64
}

type Input struct {
	A, B          Site
	Radio         Radio
	ClutterLossDB float64
}

type Sample struct {
	DistM    float64
	TerrainM float64
	BulgeM   float64
	RayM     float64
	FresnelM float64
	CanopyM  float64
}

type Result struct {
	DistanceM  float64
	BearingDeg float64
	TerrainAM  float64
	TerrainBM  float64

	FreeSpaceLossDB float64
	PathLossDB      float64
	PathLossRelDB   float64
	Mode            itm.Mode
	Warnings        []string

	EIRPdBm       float64
	RxLevelDBm    float64
	RxLevelRelDBm float64
	MarginDB      float64
	MarginRelDB   float64

	TxTakeoffDeg      float64
	RxTakeoffDeg      float64
	TxOffBoresightDeg float64
	RxOffBoresightDeg float64
	TxPatternLossDB   float64
	RxPatternLossDB   float64
	ClutterLossDB     float64
	HasCanopy         bool
	CanopyPathAM      float64
	CanopyPathBM      float64
	VegetationLossDB  float64

	WorstClearancePct   float64
	WorstClearanceDistM float64

	Profile []Sample
}

type Analyzer interface {
	Analyze(ctx context.Context, in Input) (Result, error)
}

type Calculator struct {
	DEM    dem.Source
	Canopy dem.Canopy
}

func (c Calculator) Analyze(ctx context.Context, in Input) (Result, error) {
	start := time.Now()
	r, err := c.analyze(ctx, in)
	metrics.LinkDuration.Observe(time.Since(start).Seconds())
	result := "ok"
	if err != nil {
		result = "error"
	}
	metrics.LinkAnalyses.WithLabelValues(result).Inc()
	return r, err
}

func (c Calculator) analyze(ctx context.Context, in Input) (Result, error) {
	var r Result
	r.DistanceM = geo.Distance(in.A.Pos, in.B.Pos)
	r.BearingDeg = geo.InitialBearing(in.A.Pos, in.B.Pos)
	if r.DistanceM <= 0 {
		return r, fmt.Errorf("link: sites are at the same position")
	}
	if r.DistanceM > maxDistM {
		return r, fmt.Errorf("link: distance %.0f km exceeds model limit of %.0f km", r.DistanceM/1000, maxDistM/1000)
	}

	if _, err := c.DEM.Elevation(in.A.Pos.Lat, in.A.Pos.Lon); err != nil {
		return r, fmt.Errorf("link: %s (%.5f, %.5f): %w", in.A.Name, in.A.Pos.Lat, in.A.Pos.Lon, err)
	}
	n := int(math.Ceil(r.DistanceM/max(c.DEM.ResolutionM(), minStepM))) + 1
	n = min(n, maxSamples)
	step := r.DistanceM / float64(n-1)
	elev := make([]float64, n)
	var canopy []float64
	if c.Canopy != nil {
		canopy = make([]float64, n)
	}
	for i := range elev {
		if err := ctx.Err(); err != nil {
			return r, err
		}
		p := geo.PointAt(in.A.Pos, in.B.Pos, float64(i)/float64(n-1))
		h, err := c.DEM.Elevation(p.Lat, p.Lon)
		if err != nil {
			return r, fmt.Errorf("link: %.0f m from %s (%.5f, %.5f): %w", float64(i)*step, in.A.Name, p.Lat, p.Lon, err)
		}
		elev[i] = h
		if canopy != nil {
			if canopy[i], err = c.Canopy.HeightM(p.Lat, p.Lon); err != nil {
				return r, fmt.Errorf("link: canopy %.0f m from %s (%.5f, %.5f): %w", float64(i)*step, in.A.Name, p.Lat, p.Lon, err)
			}
		}
	}
	r.TerrainAM, r.TerrainBM = elev[0], elev[n-1]

	params := itm.Params{
		HTxM: in.A.AntennaHeightM, HRxM: in.B.AntennaHeightM,
		FreqMHz: in.Radio.FreqMHz, Polarization: in.Radio.Polarization,
		Epsilon: Epsilon, Sigma: Sigma, N0: N0, Climate: Climate, Mdvar: MdvarFixed,
		Time: TimePct, Location: LocPct, Situation: SitPct,
	}
	res, err := itm.PointToPoint(elev, step, params)
	if err != nil {
		return r, err
	}
	r.PathLossDB = res.LossDB
	r.FreeSpaceLossDB = res.AFsDB
	r.Mode = res.Mode
	r.Warnings = res.Warnings.Messages()

	params.Time = ReliabilityPct
	rel, err := itm.PointToPoint(elev, step, params)
	if err != nil {
		return r, err
	}
	r.PathLossRelDB = rel.LossDB

	r.TxTakeoffDeg = res.TakeoffRad[0] * 180 / math.Pi
	r.RxTakeoffDeg = res.TakeoffRad[1] * 180 / math.Pi
	r.TxOffBoresightDeg = r.TxTakeoffDeg + in.Radio.TxTiltDeg
	r.RxOffBoresightDeg = r.RxTakeoffDeg + in.Radio.RxTiltDeg
	r.TxPatternLossDB = in.Radio.TxPattern.AttenuationDB(r.BearingDeg-in.Radio.TxAzimuthDeg, -r.TxOffBoresightDeg)
	r.RxPatternLossDB = in.Radio.RxPattern.AttenuationDB(r.BearingDeg+180-in.Radio.RxAzimuthDeg, -r.RxOffBoresightDeg)

	// ITM ran on bare terrain; ITU-R P.833 canopy loss at the ends comes on top
	r.Profile, r.WorstClearancePct, r.WorstClearanceDistM = profile(elev, canopy, step, in)
	if canopy != nil {
		r.HasCanopy = true
		r.CanopyPathAM, r.CanopyPathBM = canopyPath(r.Profile)
		r.VegetationLossDB = excessDB(r.CanopyPathAM, in.Radio.FreqMHz, in.Radio.Polarization) +
			excessDB(r.CanopyPathBM, in.Radio.FreqMHz, in.Radio.Polarization)
	}

	r.EIRPdBm = in.Radio.TxPowerDBm + in.Radio.TxGainDBi - r.TxPatternLossDB - in.Radio.TxLineLossDB
	r.ClutterLossDB = in.ClutterLossDB
	rxSide := in.Radio.RxGainDBi - r.RxPatternLossDB - in.Radio.RxLineLossDB - r.ClutterLossDB - r.VegetationLossDB
	r.RxLevelDBm = r.EIRPdBm - r.PathLossDB + rxSide
	r.RxLevelRelDBm = r.EIRPdBm - r.PathLossRelDB + rxSide
	r.MarginDB = r.RxLevelDBm - in.Radio.RxSensitivityDBm
	r.MarginRelDB = r.RxLevelRelDBm - in.Radio.RxSensitivityDBm
	return r, nil
}

func profile(elev, canopy []float64, step float64, in Input) ([]Sample, float64, float64) {
	n := len(elev)
	d := float64(step * float64(n-1))
	hA := elev[0] + in.A.AntennaHeightM
	hB := elev[n-1] + in.B.AntennaHeightM
	lambda := speedOfLight / (in.Radio.FreqMHz * 1e6)
	re := kFactor * geo.EarthRadius

	samples := make([]Sample, n)
	worst, worstDist := math.Inf(1), 0.0
	for i := range samples {
		d1 := float64(i) * step
		if i == n-1 {
			d1 = d
		}
		d2 := d - d1
		s := Sample{
			DistM:    d1,
			TerrainM: elev[i],
			BulgeM:   d1 * d2 / (2 * re),
			RayM:     hA + (hB-hA)*d1/d,
			FresnelM: math.Sqrt(lambda * d1 * d2 / d),
		}
		if canopy != nil {
			s.CanopyM = canopy[i]
		}
		samples[i] = s
		if i == 0 || i == n-1 {
			continue
		}
		pct := (s.RayM - s.TerrainM - s.BulgeM - s.CanopyM) / s.FresnelM * 100
		if pct < worst {
			worst, worstDist = pct, d1
		}
	}
	if math.IsInf(worst, 1) {
		worst = 0
	}
	return samples, worst, worstDist
}

func OffBoresight(deg float64) string {
	switch {
	case math.Abs(deg) < 0.05:
		return "on boresight"
	case deg > 0:
		return fmt.Sprintf("%.1f deg above boresight", deg)
	default:
		return fmt.Sprintf("%.1f deg below boresight", -deg)
	}
}
