package hf

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// benchRequest is a long path (Zurich to Sydney, about 16500 km) with the hours and frequencies of a full
// prediction; the long path model builds its control point table on every run.
func benchRequest() Request {
	req := Request{
		TxLat: 47.4, TxLng: 8.5, RxLat: -33.9, RxLng: 151.2,
		Year: 2026, Month: 6, SSN: 100,
		TxPower: 0, Bandwidth: 3000, RequiredSNR: 15, SNRXXPercent: 90, ManMadeNoise: NoiseRural,
		Modulation: Analog,
	}
	for h := 1; h <= 24; h++ {
		req.Hours = append(req.Hours, h)
	}
	for f := 1; f <= 30; f++ {
		req.Frequencies = append(req.Frequencies, float64(f))
	}
	return req
}

// BenchmarkPredict measures one prediction of 24 hours and 30 frequencies on a long path.
func BenchmarkPredict(b *testing.B) {
	dataDir, _ := testDirs(b)
	d, err := Open(dataDir, "")
	if err != nil {
		b.Fatal(err)
	}
	req := benchRequest()
	if _, err := d.Predict(req); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := d.Predict(req); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkMap measures the runs of a reliability map as the web handler makes them: one prediction per cell
// of a 10 degree grid (648 cells), 24 hours at one frequency, run one after the other.
func BenchmarkMap(b *testing.B) {
	dataDir, _ := testDirs(b)
	d, err := Open(dataDir, "")
	if err != nil {
		b.Fatal(err)
	}
	req := benchRequest()
	req.Frequencies = []float64{14}
	b.ReportAllocs()
	for b.Loop() {
		for row := 0; row < 18; row++ {
			for col := 0; col < 36; col++ {
				req.RxLat, req.RxLng = 90-10*(float64(row)+0.5), -180+10*(float64(col)+0.5)
				if _, err := d.Predict(req); err != nil {
					b.Fatal(err)
				}
			}
		}
	}
}

// TestGolden hashes the full result of every reference case and compares it with testdata/golden_<GOARCH>.txt.
// TestReferenceCases only checks the digits the C report prints; this test catches any change of a bit, so a
// performance change can prove it left the results alone. The hash is per architecture because arm64 fuses
// multiply-add. HF_GOLDEN_WRITE=1 writes the file for a deliberate change.
func TestGolden(t *testing.T) {
	dataDir, refDir := testDirs(t)
	d, err := Open(dataDir, "")
	if err != nil {
		t.Fatal(err)
	}
	inputs, _ := filepath.Glob(filepath.Join(refDir, "[cx]*??.in"))
	if len(inputs) == 0 {
		t.Fatal("no reference cases in testdata")
	}
	h := sha256.New()
	for _, in := range inputs {
		res, err := d.Predict(readCaseInput(t, in))
		if err != nil {
			t.Fatal(err)
		}
		// JSON prints the shortest form that round-trips, so equal text means equal bits.
		enc, _ := json.Marshal(res)
		h.Write(enc)
	}
	got := hex.EncodeToString(h.Sum(nil))
	file := filepath.Join(refDir, "golden_"+runtime.GOARCH+".txt")
	if os.Getenv("HF_GOLDEN_WRITE") != "" {
		if err := os.WriteFile(file, []byte(got+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("%s: %v (hash now %s; run with HF_GOLDEN_WRITE=1 to write it)", file, err, got)
	}
	if got != strings.TrimSpace(string(want)) {
		t.Fatalf("results changed: hash %s, golden %s", got, strings.TrimSpace(string(want)))
	}
}
