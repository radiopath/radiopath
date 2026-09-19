package report

import (
	"math"
	"testing"
	"time"

	"github.com/radiopath/radiopath/internal/coverage"
	"github.com/radiopath/radiopath/internal/plot"
	"github.com/radiopath/radiopath/internal/store"
	"github.com/radiopath/radiopath/internal/tiles"
)

func TestCoverageRaster(t *testing.T) {
	r := coverage.Result{W: 60, H: 40, Margin: make([]float64, 2400)}
	for i := range r.Margin {
		x, y := float64(i%60)-30, float64(i/60)-20
		d := math.Hypot(x, y)
		r.Margin[i] = math.NaN()
		if d < 18 {
			r.Margin[i] = 40 - 2*d
		}
	}
	now := time.Date(2026, 9, 6, 17, 28, 0, 0, time.UTC)
	c := store.CoverageWithSite{
		Coverage: store.Coverage{Name: "b34 > ghpsa", FreqMHz: 5720, TxPowerDBm: 30, TxGainDBi: 27, RxHeightM: 2, RxSensitivityDBm: -120, Polarization: 1, RangeM: 30000, ResolutionM: 100,
			ComputedAt: &now, ComputeMs: 3088, Bounds: &store.Bounds{North: 47.59234, South: 47.05274, East: 9.83317, West: 9.03715}},
		Site: store.Site{Name: "b34", Lat: 47.32254, Lon: 9.43516, AntennaHeightM: 3},
	}
	in := Coverage{Coverage: c, TxAntenna: "Mikrotik LHG 5 XL ac", User: "hb9hil", Generated: now,
		View:   tiles.Fit(c.Bounds.South, c.Bounds.West, c.Bounds.North, c.Bounds.East, CoverageMapPx, CoverageMapPx, MapPad).Trim(c.Bounds.South, c.Bounds.West, c.Bounds.North, c.Bounds.East, MapPad),
		Raster: plot.CoveragePNG(r, plot.DefaultLegend())}
	in.Map = flatPNG(in.View.W, in.View.H, colorGrey)
	check(t, "coverage_raster", buildCoverage(in))
}
