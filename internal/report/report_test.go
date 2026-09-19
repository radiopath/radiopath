package report

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/radiopath/radiopath/internal/itm"
	"github.com/radiopath/radiopath/internal/link"
	"github.com/radiopath/radiopath/internal/store"
	"github.com/radiopath/radiopath/internal/tiles"
)

var colorGrey = color.RGBA{230, 235, 225, 255}

func flatPNG(w, h int, c color.RGBA) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = []byte{c.R, c.G, c.B, c.A}[i%4]
	}
	var b bytes.Buffer
	png.Encode(&b, img)
	return b.Bytes()
}

func testResult() *link.Result {
	r := &link.Result{
		DistanceM: 34030, BearingDeg: 296.7, TerrainAM: 824, TerrainBM: 568,
		FreeSpaceLossDB: 106.3, PathLossDB: 137.4, PathLossRelDB: 138.9, Mode: itm.ModeLineOfSight,
		EIRPdBm: 50, RxLevelDBm: -61.1, MarginDB: 58.9, MarginRelDB: 57.3,
		TxPatternLossDB: 0.3, HasCanopy: true, CanopyPathAM: 14, VegetationLossDB: 0.7,
		WorstClearancePct: -249, WorstClearanceDistM: 5500,
		Warnings: []string{"antenna gain looks high for 145 MHz"},
	}
	const n = 300
	for i := 0; i < n; i++ {
		f := float64(i) / (n - 1)
		d := f * r.DistanceM
		t := 700 + 250*math.Exp(-math.Pow((f-0.17)/0.05, 2)) + 150*math.Exp(-math.Pow((f-0.5)/0.08, 2)) - 200*f
		s := link.Sample{DistM: d, TerrainM: t, BulgeM: d * (r.DistanceM - d) / (2 * 8495000), RayM: 827 + (598-827)*f, FresnelM: 17.3 * math.Sqrt(d*(r.DistanceM-d)/r.DistanceM/1000)}
		if f > 0.85 {
			s.CanopyM = 18
		}
		r.Profile = append(r.Profile, s)
	}
	return r
}

func TestLinkPDF(t *testing.T) {
	measured := -83.0
	in := Link{
		Link: store.LinkWithSites{
			Link:  store.Link{Name: "b34 > datapark", FreqMHz: 145, TxPowerDBm: 23, TxGainDBi: 27, RxGainDBi: 27, RxSensitivityDBm: -120, Polarization: 1, MeasuredRxDBm: &measured},
			SiteA: store.Site{Name: "b34", Lat: 47.32253, Lon: 9.43516, AntennaHeightM: 3},
			SiteB: store.Site{Name: "datapark", Lat: 47.45937, Lon: 9.03081, AntennaHeightM: 30},
		},
		Result: testResult(), TxAntenna: "Mikrotik LHG 5 XL ac", RxAntenna: "Mikrotik LHG 5 XL ac",
		TxAzimuthDeg: 297, RxAzimuthDeg: 117, ReliabilityPct: 99, User: "hb9hil", Generated: time.Date(2026, 9, 6, 13, 51, 0, 0, time.UTC),
		Map: flatPNG(MapW, MapH, color.RGBA{230, 235, 225, 255}), View: tiles.Fit(47.32253, 9.03081, 47.45937, 9.43516, MapW, MapH, 80),
	}
	check(t, "link", buildLink(in))

	in.Result, in.Error, in.Map = nil, "no DEM tile N47E009", nil
	check(t, "link_failed", buildLink(in))
}

func TestCoveragePDF(t *testing.T) {
	now := time.Date(2026, 9, 6, 13, 51, 0, 0, time.UTC)
	c := store.CoverageWithSite{
		Coverage: store.Coverage{Name: "Kronberg 70 cm", FreqMHz: 438.5, TxPowerDBm: 40, TxGainDBi: 6, RxHeightM: 2, RxGainDBi: 0, RxSensitivityDBm: -110, Polarization: 1, RangeM: 20000, ResolutionM: 100,
			ComputedAt: &now, ComputeMs: 4200, Bounds: &store.Bounds{North: 47.45, South: 47.09, East: 9.55, West: 9.02}},
		Site: store.Site{Name: "Kronberg", Lat: 47.27, Lon: 9.29, AntennaHeightM: 10},
	}
	in := Coverage{Coverage: c, TxAntenna: "Sector 90", User: "hb9hil", Generated: now,
		View:   tiles.Fit(c.Bounds.South, c.Bounds.West, c.Bounds.North, c.Bounds.East, CoverageMapPx, CoverageMapPx, MapPad).Trim(c.Bounds.South, c.Bounds.West, c.Bounds.North, c.Bounds.East, MapPad),
		Raster: flatPNG(40, 30, color.RGBA{60, 180, 60, 255})}
	in.Map = flatPNG(in.View.W, in.View.H, colorGrey)
	check(t, "coverage", buildCoverage(in))

	c.ComputedAt, c.Bounds, c.Note = nil, nil, "DEM tile N47E009 missing"
	in.Coverage, in.Raster = c, nil
	check(t, "coverage_uncomputed", buildCoverage(in))
}

func check(t *testing.T, name string, d *doc) {
	t.Helper()
	if err := d.Error(); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if n := d.PageCount(); n != 1 {
		t.Errorf("%s: %d pages, want 1", name, n)
	}
	out, err := d.bytes()
	if err != nil || !bytes.HasPrefix(out, []byte("%PDF")) {
		t.Fatalf("%s: %v, %d bytes", name, err, len(out))
	}
	if dir := os.Getenv("RADIOPATH_TEST_OUT"); dir != "" {
		os.WriteFile(filepath.Join(dir, name+".pdf"), out, 0o644)
	}
}
