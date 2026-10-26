package report

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/radiopath/radiopath/internal/store"
)

// testHF returns months of a prediction with every hour and the given frequencies; hour 3 has no MUF and the
// highest frequency is not supported at night.
func testHF(months int, freqs []float64) HF {
	in := HF{
		SiteA: store.Site{Name: "Säntis", Lat: 47.2495, Lon: 9.3433}, SiteB: store.Site{Name: "Washington", Lat: 38.9, Lon: -77},
		Radio: [][]string{{"Path", "6600 km, short path"}, {"TX power", "20.0 dBW (100 W)"}},
		User:  "hb9hil", Generated: time.Date(2026, 10, 26, 10, 0, 0, 0, time.UTC),
	}
	for m := range months {
		hm := HFMonth{Year: 2026, Month: 10 + m%3, SSN: 120 - m}
		for h := 1; h <= 24; h++ {
			muf := 14 + 10*math.Sin(float64(h)/24*2*math.Pi)
			for _, f := range freqs {
				p := HFPoint{Hour: h, FreqMHz: f, MUF10: muf * 1.2, MUF50: muf, MUF90: muf * 0.8,
					ReliabilityPct: math.Max(0, math.Min(100, (muf-f)*10+50)), SNRDB: muf - f, Mode: "2F2"}
				if h == 3 {
					p.MUF10, p.MUF50, p.MUF90, p.ReliabilityPct, p.Mode = hfNA, hfNA, hfNA, hfNA, "NONE"
				}
				hm.Points = append(hm.Points, p)
			}
		}
		in.Months = append(in.Months, hm)
	}
	return in
}

func TestHFPDF(t *testing.T) {
	for _, c := range []struct {
		name   string
		months int
		freqs  []float64
	}{
		{"hf", 1, []float64{7.1, 14.2}},
		{"hf_year", 12, []float64{3.6, 7.1, 10.1, 14.2, 18.1, 21.2, 24.9}},
	} {
		d := buildHF(testHF(c.months, c.freqs))
		if err := d.Error(); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		out, err := d.bytes()
		if err != nil || !bytes.HasPrefix(out, []byte("%PDF")) {
			t.Fatalf("%s: %v, %d bytes", c.name, err, len(out))
		}
		if dir := os.Getenv("RADIOPATH_TEST_OUT"); dir != "" {
			os.WriteFile(filepath.Join(dir, c.name+".pdf"), out, 0o644)
		}
	}
}

// heat matches the colour ramp of the HF page at its ends and middle.
func TestHeat(t *testing.T) {
	for pct, want := range map[float64][3]uint8{0: {201, 29, 29}, 50: {201, 201, 29}, 100: {29, 201, 29}} {
		c := heat(pct)
		if d := math.Abs(float64(c.R)-float64(want[0])) + math.Abs(float64(c.G)-float64(want[1])) + math.Abs(float64(c.B)-float64(want[2])); d > 3 {
			t.Errorf("heat(%v) = %v, want %v", pct, c, want)
		}
	}
}
