package web

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/danielgtaylor/huma/v2"

	"github.com/radiopath/radiopath/internal/antenna"
	"github.com/radiopath/radiopath/internal/hf"
	"github.com/radiopath/radiopath/internal/store"
)

// hfInput returns a valid prediction input.
func hfInput() HFPredictionInput {
	return HFPredictionInput{
		SiteAID: 1, SiteBID: 2, Year: 2020, Month: 4, HoursUTC: []int{6, 12}, SSN: 60,
		FrequenciesMHz: []float64{7.1, 14.2}, TxPowerDBW: 0, BandwidthHz: 3000, RequiredSNRDB: 15,
		SNRPercentile: 90, NoiseEnvironment: "rural",
	}
}

// TestParseHF maps a valid input to the model request: sites, noise environment, defaults of the digital parameters.
func TestParseHF(t *testing.T) {
	a := store.Site{ID: 1, Lat: 41.9, Lon: 12.5}
	b := store.Site{ID: 2, Lat: 52.5, Lon: 13.4}
	req, err := parseHF(hfInput(), a, b)
	if err != nil {
		t.Fatal(err)
	}
	if req.TxLat != 41.9 || req.RxLng != 13.4 || req.Month != 4 || req.ManMadeNoise != hf.NoiseRural || req.Modulation != hf.Analog {
		t.Errorf("unexpected request %+v", req)
	}
	in := hfInput()
	in.Modulation = "digital"
	if req, err = parseHF(in, a, b); err != nil || req.Modulation != hf.Digital || req.Digital.RequiredSIR != 10 {
		t.Errorf("digital without parameters: %+v, %v", req, err)
	}
}

// Every invalid field comes back in one 422 with its location.
func TestParseHFInvalid(t *testing.T) {
	a := store.Site{ID: 1}
	b := store.Site{ID: 2}
	in := hfInput()
	in.Year = 1800
	in.Month = 13
	in.HoursUTC = []int{0}
	in.FrequenciesMHz = []float64{0.5, 40}
	in.SNRPercentile = 100
	in.NoiseEnvironment = "swamp"
	in.Modulation = "ssb"
	_, err := parseHF(in, a, b)
	em, ok := err.(*huma.ErrorModel)
	if !ok || em.Status != 422 {
		t.Fatalf("want 422, got %v", err)
	}
	for _, loc := range []string{"body.year", "body.month", "body.hours_utc", "body.frequencies_mhz", "body.snr_percentile", "body.noise_environment", "body.modulation"} {
		found := false
		for _, d := range em.Errors {
			if d.Location == loc {
				found = true
			}
		}
		if !found {
			t.Errorf("no error for %s", loc)
		}
	}

	in = hfInput()
	in.HoursUTC = make([]int, maxHFHours+1)
	for i := range in.HoursUTC {
		in.HoursUTC[i] = 1
	}
	if _, err := parseHF(in, a, b); err == nil {
		t.Error("too many hours accepted")
	}
	if _, err := parseHF(hfInput(), a, a); err == nil {
		t.Error("same site accepted")
	}
	in = hfInput()
	id := int64(7)
	in.TxAntennaID, in.TxGainDBi = &id, 100
	if req, err := parseHF(in, a, b); err != nil || req.TxAntenna.GainOffset != 0 {
		t.Errorf("gain with an antenna: %v, offset %v", err, req.TxAntenna.GainOffset)
	}
	in.TxAntennaID = nil
	if _, err := parseHF(in, a, b); err == nil {
		t.Error("isotropic gain of 100 accepted")
	}
	in = hfInput()
	in.HoursUTC = []int{6, 12, 6}
	if _, err := parseHF(in, a, b); err == nil {
		t.Error("duplicate hour accepted")
	}
	in = hfInput()
	in.FrequenciesMHz = []float64{7.1, 14.2, 7.1}
	if _, err := parseHF(in, a, b); err == nil {
		t.Error("duplicate frequency accepted")
	}
}

// Values JSON cannot carry become the model's "does not apply" value.
func TestFinite(t *testing.T) {
	if finite(math.NaN()) != hfNotAvailable || finite(math.Inf(1)) != hfNotAvailable || finite(1.5) != 1.5 {
		t.Error("finite")
	}
}

// TestHFPredictionJSON runs the model for a path and checks that the answer has one point per hour and frequency and
// serialises; it needs the ITU data set (scripts/fetch-hf-data.sh).
func TestHFPredictionJSON(t *testing.T) {
	data, err := hf.Open(hfDataDir(t), "")
	if err != nil {
		t.Fatal(err)
	}
	req, err := parseHF(hfInput(), store.Site{ID: 1, Lat: 41.9, Lon: 12.5}, store.Site{ID: 2, Lat: 52.5, Lon: 13.4})
	if err != nil {
		t.Fatal(err)
	}
	res, err := data.Predict(req)
	if err != nil {
		t.Fatal(err)
	}
	out := HFPrediction{Points: make([]HFPoint, 0, len(res.Points))}
	for _, p := range res.Points {
		out.Points = append(out.Points, toHFPoint(p))
	}
	if len(out.Points) != 4 || out.Points[0].HourUTC != 6 || out.Points[0].FrequencyMHz != 7.1 || out.Points[0].DistanceKm < 1100 || out.Points[0].DistanceKm > 1250 {
		t.Errorf("unexpected points %+v", out.Points)
	}
	if _, err := json.Marshal(out); err != nil {
		t.Error(err)
	}
}

// TestHFPattern gives the antenna's gain at the main lobe and its attenuation off it; an isotropic antenna has no function.
func TestHFPattern(t *testing.T) {
	if (*hfAntenna)(nil).pattern() != nil {
		t.Error("isotropic antenna has a pattern function")
	}
	g := (&hfAntenna{GainDBi: 9, H: antenna.Sector(60, 20)}).pattern()
	if got := g(0, 0); got != 9 {
		t.Errorf("gain on boresight %v, want 9", got)
	}
	if got := g(180, 0); got >= 9 {
		t.Errorf("gain at the back %v, want below 9", got)
	}
}

// hfDataDir returns the ITU data set of the HF tests, hf1 below RADIOPATH_DATASET_DIR or tmp/testfiles, and skips the
// test without it (fails with HF_REQUIRE_DATA, as in CI).
func hfDataDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join("..", "..", "tmp", "testfiles", "hf1")
	if root := os.Getenv("RADIOPATH_DATASET_DIR"); root != "" {
		dir = filepath.Join(root, "hf1")
	}
	if _, err := os.Stat(filepath.Join(dir, "ionos01.bin")); err != nil {
		if os.Getenv("HF_REQUIRE_DATA") != "" {
			t.Fatalf("HF data not found in %s", dir)
		}
		t.Skipf("HF data not found in %s", dir)
	}
	return dir
}
