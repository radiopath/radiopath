package plot

import (
	"bytes"
	"encoding/xml"
	"image/png"
	"io"
	"math"
	"strings"
	"testing"

	"github.com/radiopath/radiopath/internal/antenna"

	"github.com/radiopath/radiopath/internal/coverage"
	"github.com/radiopath/radiopath/internal/link"
)

func TestSVG(t *testing.T) {
	var r link.Result
	for i := 0; i <= 10; i++ {
		d := float64(i) * 1000
		r.Profile = append(r.Profile, link.Sample{DistM: d, TerrainM: 500 + float64(i%3)*20, BulgeM: 1, RayM: 520, FresnelM: 5})
	}
	counts := elements(t, SVG(r, 10, 10))
	if counts["polygon"] != 1 || counts["polyline"] != 3 || counts["line"] != 4 || counts["text"] != 12 {
		t.Errorf("element counts: %v", counts)
	}

	for i := 3; i <= 5; i++ {
		r.Profile[i].CanopyM = 15
	}
	counts = elements(t, SVG(r, 10, 10))
	if counts["polygon"] != 2 || counts["polyline"] != 3 || counts["line"] != 4 || counts["text"] != 12 {
		t.Errorf("element counts with canopy: %v", counts)
	}
}

func elements(t *testing.T, out []byte) map[string]int {
	t.Helper()
	counts := map[string]int{}
	dec := xml.NewDecoder(bytes.NewReader(out))
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("invalid XML: %v", err)
		}
		if se, ok := tok.(xml.StartElement); ok {
			counts[se.Name.Local]++
		}
	}
	return counts
}

func TestCoveragePNG(t *testing.T) {
	r := coverage.Result{W: 20, H: 20, Margin: make([]float64, 400)}
	for i := range r.Margin {
		r.Margin[i] = math.NaN()
	}
	r.Margin[5*20+5] = 35
	r.Margin[5*20+6] = 5
	r.Margin[5*20+7] = -3
	img, err := png.Decode(bytes.NewReader(CoveragePNG(r, DefaultLegend())))
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 20 || img.Bounds().Dy() != 20 {
		t.Fatalf("size %v", img.Bounds())
	}
	alpha := func(x, y int) uint32 { _, _, _, a := img.At(x, y).RGBA(); return a }
	if alpha(5, 5) == 0 || alpha(6, 5) == 0 {
		t.Error("covered cells are transparent")
	}
	if alpha(7, 5) != 0 || alpha(0, 0) != 0 {
		t.Error("uncovered cells are opaque")
	}
	if alpha(10, 10) == 0 {
		t.Error("transmitter cross missing")
	}
}

func TestPatternSVG(t *testing.T) {
	p := make(antenna.Pattern, antenna.Steps)
	for i := range p {
		p[i] = 10 * (1 - math.Cos(float64(i)*math.Pi/180))
	}
	out := PatternSVG(p, 120)
	var doc struct {
		Circle  []struct{} `xml:"circle"`
		Line    []struct{} `xml:"line"`
		Polygon []struct {
			Points string `xml:"points,attr"`
		} `xml:"polygon"`
	}
	if err := xml.Unmarshal(out, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Circle) != 3 || len(doc.Line) != 2 || len(doc.Polygon) != 1 {
		t.Errorf("circles %d, lines %d, polygons %d", len(doc.Circle), len(doc.Line), len(doc.Polygon))
	}
	if n := len(strings.Fields(doc.Polygon[0].Points)); n != antenna.Steps {
		t.Errorf("lobe has %d points, want %d", n, antenna.Steps)
	}
	if out := PatternSVG(nil, 60); len(out) == 0 {
		t.Error("empty SVG for a nil pattern")
	}
}

func TestMarginPNGRoundtrip(t *testing.T) {
	r := coverage.Result{W: 8, H: 4, North: 48, South: 47, East: 9, West: 8, Margin: make([]float64, 32)}
	for i := range r.Margin {
		r.Margin[i] = math.NaN()
	}
	r.Margin[0] = 35.4
	r.Margin[1] = -3.6
	r.Margin[2] = 500
	r.Margin[3] = -500
	r.Margin[9] = 0
	r.Margin[10] = 29.9
	got, err := DecodeRaster(MarginPNG(r), 48, 47, 9, 8)
	if err != nil {
		t.Fatal(err)
	}
	if got.W != 8 || got.H != 4 || got.North != 48 || got.West != 8 {
		t.Fatalf("decoded %dx%d N%v W%v", got.W, got.H, got.North, got.West)
	}
	want := map[int]float64{0: 35, 1: -4, 2: 127, 3: -127, 9: 0, 10: 29}
	for i, m := range got.Margin {
		w, ok := want[i]
		switch {
		case !ok && !math.IsNaN(m):
			t.Errorf("cell %d: got %v, want NaN", i, m)
		case ok && m != w:
			t.Errorf("cell %d: got %v, want %v", i, m, w)
		}
	}
}

func TestDecodeCoveragePNG(t *testing.T) {
	r := coverage.Result{W: 20, H: 20, Margin: make([]float64, 400)}
	for i := range r.Margin {
		r.Margin[i] = math.NaN()
	}
	r.Margin[5*20+5] = 35
	r.Margin[5*20+6] = 5
	r.Margin[5*20+7] = -3
	got, err := DecodeRaster(CoveragePNG(r, DefaultLegend()), 1, 0, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got.At(5, 5) != 30 || got.At(6, 5) != 0 {
		t.Errorf("classes: %v %v, want 30 0", got.At(5, 5), got.At(6, 5))
	}
	if !math.IsNaN(got.At(7, 5)) || !math.IsNaN(got.At(10, 10)) {
		t.Error("transparent cell or cross should be NaN")
	}
	comp, err := DecodeRaster(CompositePNG(r, DefaultLegend()), 1, 0, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !math.IsNaN(comp.At(10, 10)) || comp.At(5, 5) != 30 {
		t.Error("composite differs from coverage beyond the cross")
	}
	if _, err := DecodeRaster([]byte("nope"), 1, 0, 1, 0); err == nil {
		t.Error("garbage decoded")
	}
}

func TestLegend(t *testing.T) {
	l, err := NewLegend([]float64{0, 40, 12.5}, []string{"#ff0000", "#00ff00", "#0000ff"})
	if err != nil {
		t.Fatal(err)
	}
	if l[0].MinDB != 40 || l[1].MinDB != 12.5 || l[2].MinDB != 0 || l[0].Hex() != "#00ff00" {
		t.Errorf("not ordered best first: %v", l)
	}
	if l.Label(0) != ">= 40 dB" || l.Label(1) != "12.5 to 40 dB" || l.Label(2) != "0 to 12.5 dB" || l.Floor() != "below 0 dB / no data" {
		t.Errorf("labels %q %q %q %q", l.Label(0), l.Label(1), l.Label(2), l.Floor())
	}
	if got, _ := NewLegend(l.Mins(), l.Hex()); got[1].Color != l[1].Color {
		t.Error("Mins/Hex do not round-trip")
	}
	bad := []struct {
		mins   []float64
		colors []string
	}{
		{[]float64{10, 10}, []string{"#000000", "#000000"}},
		{[]float64{10}, []string{"red"}},
		{[]float64{10}, []string{"#12345"}},
		{[]float64{200}, []string{"#000000"}},
		{[]float64{10, 20}, []string{"#000000"}},
		{nil, nil},
		{make([]float64, MaxBins+1), make([]string, MaxBins+1)},
	}
	for _, b := range bad {
		if _, err := NewLegend(b.mins, b.colors); err == nil {
			t.Errorf("accepted %v %v", b.mins, b.colors)
		}
	}
	if d := LegendOr(nil, nil); len(d) != 4 || d[0].MinDB != 30 {
		t.Errorf("default legend %v", d)
	}
	if d := LegendOr([]float64{5, 5}, []string{"#000000", "#000000"}); d[0].MinDB != 30 {
		t.Error("a broken stored legend should fall back to the default")
	}
}

func TestCoveragePNGLegend(t *testing.T) {
	r := coverage.Result{W: 20, H: 20, Margin: make([]float64, 400)}
	for i := range r.Margin {
		r.Margin[i] = math.NaN()
	}
	r.Margin[0], r.Margin[1], r.Margin[2] = 45, 25, 5
	l, _ := NewLegend([]float64{40, 20}, []string{"#ff0000", "#0000ff"})
	img, err := png.Decode(bytes.NewReader(CoveragePNG(r, l)))
	if err != nil {
		t.Fatal(err)
	}
	rgb := func(x int) (uint32, uint32, uint32, uint32) {
		cr, cg, cb, ca := img.At(x, 0).RGBA()
		return cr >> 8, cg >> 8, cb >> 8, ca >> 8
	}
	if cr, _, _, _ := rgb(0); cr != 255 {
		t.Error("45 dB should be red")
	}
	if _, _, cb, _ := rgb(1); cb != 255 {
		t.Error("25 dB should be blue")
	}
	if _, _, _, ca := rgb(2); ca != 0 {
		t.Error("5 dB is below every threshold and should be transparent")
	}
	if _, _, _, ca := rgb(3); ca != 0 {
		t.Error("NaN should be transparent")
	}
}
