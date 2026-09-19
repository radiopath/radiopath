package link

import (
	"context"
	"math"
	"strings"
	"testing"

	"github.com/radiopath/radiopath/internal/antenna"
	"github.com/radiopath/radiopath/internal/geo"
	"github.com/radiopath/radiopath/internal/itm"
)

type fakeDEM func(lat, lon float64) float64

func (f fakeDEM) Elevation(lat, lon float64) (float64, error) { return f(lat, lon), nil }
func (f fakeDEM) ResolutionM() float64                        { return 30 }

var radio = Radio{FreqMHz: 145, TxPowerDBm: 40, RxSensitivityDBm: -120, Polarization: itm.Vertical}

var (
	siteA = Site{Name: "A", Pos: geo.Point{Lat: 47.0, Lon: 9.0}, AntennaHeightM: 10}
	siteB = Site{Name: "B", Pos: geo.Point{Lat: 47.09, Lon: 9.0}, AntennaHeightM: 10}
)

func TestFlat(t *testing.T) {
	c := Calculator{DEM: fakeDEM(func(_, _ float64) float64 { return 500 })}
	r, err := c.Analyze(context.Background(), Input{A: siteA, B: siteB, Radio: radio})
	if err != nil {
		t.Fatal(err)
	}
	if r.Mode != itm.ModeLineOfSight {
		t.Errorf("mode = %s", r.Mode)
	}
	if r.PathLossDB < r.FreeSpaceLossDB-1 {
		t.Errorf("path loss %.1f below free space %.1f", r.PathLossDB, r.FreeSpaceLossDB)
	}
	if math.Abs(r.DistanceM-10008) > 20 {
		t.Errorf("distance = %.0f", r.DistanceM)
	}
	if math.Abs(r.BearingDeg) > 1e-6 {
		t.Errorf("bearing = %v", r.BearingDeg)
	}
	if r.TerrainAM != 500 || r.TerrainBM != 500 {
		t.Errorf("terrain %v %v", r.TerrainAM, r.TerrainBM)
	}
	wantRx := 40 - r.PathLossDB
	if math.Abs(r.RxLevelDBm-wantRx) > 1e-9 || math.Abs(r.MarginDB-(wantRx+120)) > 1e-9 {
		t.Errorf("rx %.2f margin %.2f", r.RxLevelDBm, r.MarginDB)
	}
	if len(r.Profile) != int(math.Ceil(r.DistanceM/30))+1 {
		t.Errorf("profile has %d samples", len(r.Profile))
	}
	last := r.Profile[len(r.Profile)-1]
	if math.Abs(last.DistM-r.DistanceM) > 1e-6 || last.FresnelM != 0 || last.BulgeM != 0 {
		t.Errorf("last sample %+v", last)
	}
	if r.WorstClearancePct < 10 {
		t.Errorf("worst clearance %.1f%%", r.WorstClearancePct)
	}
}

func TestKnifeEdge(t *testing.T) {
	ridge := fakeDEM(func(lat, _ float64) float64 {
		if math.Abs(lat-47.045) < 0.002 {
			return 800
		}
		return 500
	})
	c := Calculator{DEM: ridge}
	r, err := c.Analyze(context.Background(), Input{A: siteA, B: siteB, Radio: radio})
	if err != nil {
		t.Fatal(err)
	}
	if r.PathLossDB < r.FreeSpaceLossDB+6 {
		t.Errorf("path loss %.1f, free space %.1f", r.PathLossDB, r.FreeSpaceLossDB)
	}
	if r.WorstClearancePct >= 0 {
		t.Errorf("clearance %.1f%% should be negative", r.WorstClearancePct)
	}
	if math.Abs(r.WorstClearanceDistM-5000) > 300 {
		t.Errorf("worst clearance at %.0f m", r.WorstClearanceDistM)
	}
}

func TestErrors(t *testing.T) {
	c := Calculator{DEM: fakeDEM(func(_, _ float64) float64 { return 0 })}
	far := Site{Name: "far", Pos: geo.Point{Lat: 47, Lon: 40}, AntennaHeightM: 10}
	if _, err := c.Analyze(context.Background(), Input{A: siteA, B: far, Radio: radio}); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("2000 km limit: %v", err)
	}
	if _, err := c.Analyze(context.Background(), Input{A: siteA, B: siteA, Radio: radio}); err == nil {
		t.Error("same position accepted")
	}
	bad := radio
	bad.FreqMHz = 10
	if _, err := c.Analyze(context.Background(), Input{A: siteA, B: siteB, Radio: bad}); err != itm.ErrFrequency {
		t.Errorf("10 MHz: %v", err)
	}
}

func TestAzimuthPattern(t *testing.T) {
	c := Calculator{DEM: fakeDEM(func(_, _ float64) float64 { return 500 })}
	base, err := c.Analyze(context.Background(), Input{A: siteA, B: siteB, Radio: radio})
	if err != nil {
		t.Fatal(err)
	}

	p := make(antenna.Pattern, antenna.Steps)
	for i := range p {
		p[i] = 10 * (1 - math.Cos(float64(i)*math.Pi/180))
	}

	aimed := radio
	aimed.TxPattern, aimed.TxAzimuthDeg = antenna.Lobe{H: p}, 0
	aimed.RxPattern, aimed.RxAzimuthDeg = antenna.Lobe{H: p}, 180
	r, err := c.Analyze(context.Background(), Input{A: siteA, B: siteB, Radio: aimed})
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(r.MarginDB-base.MarginDB) > 1e-3 {
		t.Errorf("aimed antennas changed the margin by %.3f dB", r.MarginDB-base.MarginDB)
	}

	away := aimed
	away.TxAzimuthDeg, away.RxAzimuthDeg = 180, 0
	r, err = c.Analyze(context.Background(), Input{A: siteA, B: siteB, Radio: away})
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(r.TxPatternLossDB-20) > 1e-6 || math.Abs(r.RxPatternLossDB-20) > 1e-6 {
		t.Errorf("pattern loss tx %.3f rx %.3f, want 20 each", r.TxPatternLossDB, r.RxPatternLossDB)
	}
	if math.Abs(r.MarginDB-(base.MarginDB-40)) > 1e-6 {
		t.Errorf("margin %.3f, want %.3f", r.MarginDB, base.MarginDB-40)
	}
}

func TestReliability(t *testing.T) {
	c := Calculator{DEM: fakeDEM(func(_, _ float64) float64 { return 500 })}
	r, err := c.Analyze(context.Background(), Input{A: siteA, B: siteB, Radio: radio})
	if err != nil {
		t.Fatal(err)
	}
	if r.PathLossRelDB < r.PathLossDB {
		t.Errorf("loss at %.0f%% of time (%.1f dB) below the median (%.1f dB)",
			ReliabilityPct, r.PathLossRelDB, r.PathLossDB)
	}
	if r.MarginRelDB > r.MarginDB {
		t.Errorf("margin at %.0f%% of time (%.1f dB) above the median (%.1f dB)",
			ReliabilityPct, r.MarginRelDB, r.MarginDB)
	}
	if want := r.RxLevelRelDBm - radio.RxSensitivityDBm; math.Abs(r.MarginRelDB-want) > 1e-9 {
		t.Errorf("margin %.3f does not match the level %.3f", r.MarginRelDB, want)
	}
}

type fakeCanopy func(lat, lon float64) float64

func (f fakeCanopy) HeightM(lat, lon float64) (float64, error) { return f(lat, lon), nil }

func TestVegetation(t *testing.T) {
	flat := fakeDEM(func(_, _ float64) float64 { return 500 })
	ghz := radio
	ghz.FreqMHz = 5700
	bare, err := Calculator{DEM: flat}.Analyze(context.Background(), Input{A: siteA, B: siteB, Radio: ghz})
	if err != nil {
		t.Fatal(err)
	}

	wood := fakeCanopy(func(lat, lon float64) float64 {
		if geo.Distance(geo.Point{Lat: lat, Lon: lon}, siteB.Pos) < 50 {
			return 20
		}
		return 0
	})
	r, err := Calculator{DEM: flat, Canopy: wood}.Analyze(context.Background(), Input{A: siteA, B: siteB, Radio: ghz})
	if err != nil {
		t.Fatal(err)
	}
	if !r.HasCanopy || r.CanopyPathAM != 0 || r.CanopyPathBM < 30 || r.CanopyPathBM > 60 {
		t.Errorf("canopy path A %.1f B %.1f m", r.CanopyPathAM, r.CanopyPathBM)
	}
	want := excessDB(r.CanopyPathBM, 5700, itm.Vertical)
	if math.Abs(r.VegetationLossDB-want) > 1e-9 || want < 25 || want > 40 {
		t.Errorf("vegetation loss %.2f dB, want %.2f", r.VegetationLossDB, want)
	}
	if r.PathLossDB != bare.PathLossDB {
		t.Errorf("ITM loss changed: %.3f vs %.3f", r.PathLossDB, bare.PathLossDB)
	}
	if math.Abs(r.RxLevelDBm-(bare.RxLevelDBm-want)) > 1e-9 || math.Abs(r.MarginRelDB-(bare.MarginRelDB-want)) > 1e-9 {
		t.Errorf("rx %.3f margin(rel) %.3f, bare %.3f %.3f", r.RxLevelDBm, r.MarginRelDB, bare.RxLevelDBm, bare.MarginRelDB)
	}
	if r.WorstClearancePct >= 0 || r.WorstClearanceDistM < r.DistanceM-100 {
		t.Errorf("worst clearance %.0f%% at %.0f m of %.0f", r.WorstClearancePct, r.WorstClearanceDistM, r.DistanceM)
	}

	none := fakeCanopy(func(_, _ float64) float64 { return 0 })
	r, err = Calculator{DEM: flat, Canopy: none}.Analyze(context.Background(), Input{A: siteA, B: siteB, Radio: ghz})
	if err != nil {
		t.Fatal(err)
	}
	if !r.HasCanopy || r.VegetationLossDB != 0 || r.RxLevelDBm != bare.RxLevelDBm || r.WorstClearancePct != bare.WorstClearancePct {
		t.Errorf("treeless canopy changed the result: %+v", r)
	}
}

func TestCanopyClearing(t *testing.T) {
	flat := fakeDEM(func(_, _ float64) float64 { return 500 })
	ghz := radio
	ghz.FreqMHz = 5700
	tall := Site{Name: "B", Pos: siteB.Pos, AntennaHeightM: 10}
	in := Input{A: Site{Name: "A", Pos: siteA.Pos, AntennaHeightM: 10}, B: tall, Radio: ghz}
	bare, err := Calculator{DEM: flat}.Analyze(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}

	belt := fakeCanopy(func(lat, lon float64) float64 {
		if d := geo.Distance(geo.Point{Lat: lat, Lon: lon}, tall.Pos); d > 60 && d < 200 {
			return 25
		}
		return 0
	})
	r, err := Calculator{DEM: flat, Canopy: belt}.Analyze(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if r.CanopyPathAM != 0 || r.CanopyPathBM < 100 || r.CanopyPathBM > 160 {
		t.Errorf("canopy path A %.1f B %.1f m, want 0 and the belt width", r.CanopyPathAM, r.CanopyPathBM)
	}
	want := excessDB(r.CanopyPathBM, 5700, itm.Vertical)
	if want <= 0 || math.Abs(r.VegetationLossDB-want) > 1e-9 {
		t.Errorf("vegetation loss %.2f dB, want %.2f", r.VegetationLossDB, want)
	}
	if r.PathLossDB != bare.PathLossDB {
		t.Errorf("ITM loss changed: %.3f vs %.3f", r.PathLossDB, bare.PathLossDB)
	}
}

func TestP833(t *testing.T) {
	for _, c := range []struct {
		f     float64
		pol   itm.Polarization
		gamma float64
	}{
		{466, itm.Vertical, 0.2 * math.Pow(0.466, 0.66)},
		{949, itm.Vertical, 0.2 * math.Pow(0.949, 0.66)},
		{949, itm.Horizontal, 0.2 * 0.949},
		{1852, itm.Vertical, 0.2 * 1.852},
		{5700, itm.Vertical, 1.14},
	} {
		if g := gammaDBm(c.f, c.pol); math.Abs(g-c.gamma) > 1e-3 {
			t.Errorf("gamma(%v, %v) = %.4f, want %.4f", c.f, c.pol, g, c.gamma)
		}
	}
	if am := maxAttenuationDB(3605); math.Abs(am-42.7) > 0.2 {
		t.Errorf("A_m(3605) = %.1f, want 42.7", am)
	}
	if am := maxAttenuationDB(5700); math.Abs(am-51.8) > 0.2 {
		t.Errorf("A_m(5700) = %.1f, want 51.8", am)
	}
	if d := gammaDBm(999.9, itm.Vertical) - gammaDBm(1000.1, itm.Vertical); math.Abs(d) > 1e-3 {
		t.Errorf("gamma jumps by %.4f at 1 GHz", d)
	}
	if excessDB(0, 5700, itm.Vertical) != 0 {
		t.Error("loss without canopy path")
	}
	if a, am := excessDB(1e4, 5700, itm.Vertical), maxAttenuationDB(5700); math.Abs(a-am) > 1e-6 {
		t.Errorf("deep in the wood %.3f, want saturation at %.3f", a, am)
	}

	p := []Sample{{DistM: 0, RayM: 15, CanopyM: 20}, {DistM: 30, RayM: 15, CanopyM: 20}, {DistM: 60, RayM: 25, CanopyM: 20}, {DistM: 90, RayM: 25, CanopyM: 20}}
	if dA, dB := canopyPath(p); math.Abs(dA-45) > 1e-9 || dB != 0 {
		t.Errorf("canopy path %.1f %.1f, want 45 0", dA, dB)
	}
	for i := range p {
		p[i].RayM = 15
	}
	if dA, dB := canopyPath(p); dA != 90 || dB != 0 {
		t.Errorf("all inside: %.1f %.1f, want 90 0", dA, dB)
	}
}

func TestElevationPattern(t *testing.T) {
	c := Calculator{DEM: fakeDEM(func(_, _ float64) float64 { return 500 })}
	narrow := radio
	narrow.TxPattern = antenna.Lobe{V: antenna.Sector(10, 30)}
	r, err := c.Analyze(context.Background(), Input{A: siteA, B: siteB, Radio: narrow})
	if err != nil {
		t.Fatal(err)
	}
	if r.TxTakeoffDeg > 0 || r.TxTakeoffDeg < -0.1 || r.RxTakeoffDeg > 0 || r.RxTakeoffDeg < -0.1 {
		t.Errorf("take-off angles %.3f / %.3f deg", r.TxTakeoffDeg, r.RxTakeoffDeg)
	}
	if r.TxPatternLossDB > 0.1 {
		t.Errorf("untilted antenna loses %.2f dB", r.TxPatternLossDB)
	}

	tilted := narrow
	tilted.TxTiltDeg = 20
	r, err = c.Analyze(context.Background(), Input{A: siteA, B: siteB, Radio: tilted})
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(r.TxPatternLossDB-30) > 1e-6 {
		t.Errorf("tilted 20 deg: loss %.2f dB, want 30", r.TxPatternLossDB)
	}

	p := make(antenna.Pattern, antenna.Steps)
	for i := range p {
		p[i] = 10 * (1 - math.Cos(float64(i)*math.Pi/180))
	}
	mirror := radio
	mirror.RxPattern, mirror.RxAzimuthDeg, mirror.RxTiltDeg = antenna.Lobe{H: p}, 180, 90
	r, err = c.Analyze(context.Background(), Input{A: siteA, B: siteB, Radio: mirror})
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(r.RxPatternLossDB-10) > 0.05 {
		t.Errorf("mirrored cut tilted 90 deg: loss %.2f dB, want 10", r.RxPatternLossDB)
	}
}
