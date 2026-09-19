package plot

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/radiopath/radiopath/internal/coverage"
	"github.com/radiopath/radiopath/internal/dem"
	"github.com/radiopath/radiopath/internal/geo"
	"github.com/radiopath/radiopath/internal/itm"
	"github.com/radiopath/radiopath/internal/link"
)

func TestRealSaentis(t *testing.T) {
	dir := os.Getenv("RADIOPATH_DEM_DIR")
	if dir == "" {
		t.Skip("RADIOPATH_DEM_DIR not set")
	}
	c := coverage.Calculator{DEM: dem.NewDirSource(dir, 8)}
	for _, rng := range []float64{30000, 60000} {
		in := coverage.Input{
			TX:        link.Site{Pos: geo.Point{Lat: 47.2494, Lon: 9.3432}, AntennaHeightM: 10},
			Radio:     link.Radio{FreqMHz: 145, TxPowerDBm: 40, RxSensitivityDBm: -120, Polarization: itm.Vertical},
			RxHeightM: 2, RangeM: rng, ResolutionM: 100,
		}
		start := time.Now()
		r, err := c.Compute(context.Background(), in)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("range %.0f km: %dx%d px, %d radials, %d truncated, %v", rng/1000, r.W, r.H, r.Radials, r.Truncated, time.Since(start).Round(time.Millisecond))
		if out := os.Getenv("RADIOPATH_TEST_OUT"); out != "" {
			os.WriteFile(fmt.Sprintf("%s/saentis_%.0fkm.png", out, rng/1000), CoveragePNG(r, DefaultLegend()), 0o644)
		}
	}
}
