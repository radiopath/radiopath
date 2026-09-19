package coverage

import (
	"context"
	"math"
	"testing"

	"github.com/radiopath/radiopath/internal/antenna"
	"github.com/radiopath/radiopath/internal/dem"
	"github.com/radiopath/radiopath/internal/geo"
	"github.com/radiopath/radiopath/internal/itm"
	"github.com/radiopath/radiopath/internal/link"
)

type fakeDEM func(lat, lon float64) float64

func (f fakeDEM) Elevation(lat, lon float64) (float64, error) { return f(lat, lon), nil }
func (f fakeDEM) ResolutionM() float64                        { return 30 }

type halfDEM struct{}

func (halfDEM) ResolutionM() float64 { return 30 }

func (halfDEM) Elevation(_, lon float64) (float64, error) {
	if lon < 9.0 {
		return 0, dem.ErrNoTile
	}
	return 500, nil
}

var in = Input{
	TX:          link.Site{Name: "TX", Pos: geo.Point{Lat: 47.0, Lon: 9.0}, AntennaHeightM: 30},
	Radio:       link.Radio{FreqMHz: 145, TxPowerDBm: 40, RxSensitivityDBm: -120, Polarization: itm.Vertical},
	RxHeightM:   2,
	RangeM:      5000,
	ResolutionM: 250,
}

func TestFlat(t *testing.T) {
	c := Calculator{DEM: fakeDEM(func(_, _ float64) float64 { return 500 })}
	r, err := c.Compute(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if r.W != 40 || r.H != 40 || len(r.Margin) != 1600 {
		t.Fatalf("raster %dx%d, %d cells", r.W, r.H, len(r.Margin))
	}
	if !(r.North > 47.04 && r.North < 47.05 && r.South < 46.96 && r.West < 8.94 && r.East > 9.06) {
		t.Errorf("bounds N%.4f S%.4f W%.4f E%.4f", r.North, r.South, r.West, r.East)
	}
	if !math.IsNaN(r.At(0, 0)) || !math.IsNaN(r.At(39, 39)) {
		t.Error("corners should be NaN")
	}
	centre := r.At(20, 20)
	if math.IsNaN(centre) || centre < 40 {
		t.Errorf("centre margin %.1f", centre)
	}
	prev := math.Inf(1)
	for x := 21; x < 39; x++ {
		m := r.At(x, 20)
		if math.IsNaN(m) {
			continue
		}
		if m > prev+0.5 {
			t.Errorf("margin rises at x=%d: %.1f > %.1f", x, m, prev)
		}
		prev = m
	}
}

func TestRidgeShadow(t *testing.T) {
	ridge := fakeDEM(func(lat, _ float64) float64 {
		if lat > 47.01 && lat < 47.012 {
			return 900
		}
		return 500
	})
	r, err := Calculator{DEM: ridge}.Compute(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	north, south := r.At(20, 5), r.At(20, 35)
	if math.IsNaN(north) || math.IsNaN(south) || north > south-20 {
		t.Errorf("north %.1f should be well below south %.1f", north, south)
	}
}

func TestTruncated(t *testing.T) {
	r, err := Calculator{DEM: halfDEM{}}.Compute(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if r.Radials != 360 || r.Truncated < 170 || r.Truncated > 190 {
		t.Errorf("radials %d, truncated %d, want 360 and about 180", r.Radials, r.Truncated)
	}
	if !math.IsNaN(r.At(5, 20)) || math.IsNaN(r.At(35, 20)) {
		t.Error("west should be NaN, east covered")
	}
	if len(r.Errors) != 1 || r.Errors[0] != dem.ErrNoTile.Error() {
		t.Errorf("errors %q, want just ErrNoTile", r.Errors)
	}
}

func TestRadialCount(t *testing.T) {
	cases := []struct {
		rangeM, resM float64
		want         int
	}{
		{5000, 250, 360},
		{30000, 100, 1885},
		{100000, 100, 3600},
	}
	for _, c := range cases {
		if got := radialCount(Input{RangeM: c.rangeM, ResolutionM: c.resM}); got != c.want {
			t.Errorf("radialCount(%v, %v) = %d, want %d", c.rangeM, c.resM, got, c.want)
		}
	}
}

func TestValidation(t *testing.T) {
	c := Calculator{DEM: fakeDEM(func(_, _ float64) float64 { return 0 })}
	bad := in
	bad.RangeM = 100
	if _, err := c.Compute(context.Background(), bad); err == nil {
		t.Error("range 100 m accepted")
	}
	bad = in
	bad.ResolutionM = 10
	bad.RangeM = 50000
	if _, err := c.Compute(context.Background(), bad); err == nil {
		t.Error("oversized raster accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Compute(ctx, in); err == nil {
		t.Error("cancelled context accepted")
	}
}

func TestTilt(t *testing.T) {
	c := Calculator{DEM: fakeDEM(func(_, _ float64) float64 { return 500 })}
	flat, err := c.Compute(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	narrow := in
	narrow.Radio.TxPattern = antenna.Lobe{V: antenna.Sector(10, 30)}
	r, err := c.Compute(context.Background(), narrow)
	if err != nil {
		t.Fatal(err)
	}
	if d := flat.At(35, 20) - r.At(35, 20); d < 0 || d > 0.2 {
		t.Errorf("untilted narrow lobe costs %.2f dB at 3.9 km", d)
	}
	if d := flat.At(22, 20) - r.At(22, 20); d < 0.5 {
		t.Errorf("untilted narrow lobe costs only %.2f dB at 600 m", d)
	}

	tilted := narrow
	tilted.Radio.TxTiltDeg = 30
	r, err = c.Compute(context.Background(), tilted)
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range []int{21, 30, 38} {
		if d := flat.At(x, 20) - r.At(x, 20); math.Abs(d-30) > 1e-6 {
			t.Errorf("tilted 30 deg at x=%d: %.2f dB lost, want 30", x, d)
		}
	}
}
