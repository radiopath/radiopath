package web

import (
	"context"
	"testing"

	"github.com/danielgtaylor/huma/v2"

	"github.com/radiopath/radiopath/internal/hf"
	"github.com/radiopath/radiopath/internal/store"
)

// hfMapInput returns a valid map input.
func hfMapInput() HFMapInput {
	return HFMapInput{
		SiteID: 1, Year: 2020, Month: 4, HoursUTC: []int{6, 12}, SSN: 60, FrequencyMHz: 14.2,
		BandwidthHz: 3000, RequiredSNRDB: 15, SNRPercentile: 90, NoiseEnvironment: "rural",
	}
}

// TestHFCellCentre puts the first cell in the north-west and the last in the south-east.
func TestHFCellCentre(t *testing.T) {
	if lat, lon := hfCellCentre(0, 0); lat != 85 || lon != -175 {
		t.Errorf("first cell %v, %v", lat, lon)
	}
	if lat, lon := hfCellCentre(17, 35); lat != -85 || lon != 175 {
		t.Errorf("last cell %v, %v", lat, lon)
	}
}

// TestHFReliability reports the overall reliability only for digital modulation up to 9000 km.
func TestHFReliability(t *testing.T) {
	p := hf.Point{Distance: 5000, BCR: 40, OCR: 25}
	if got := hfReliability(p, false); got != 40 {
		t.Errorf("analog %v, want the basic 40", got)
	}
	if got := hfReliability(p, true); got != 25 {
		t.Errorf("digital %v, want the overall 25", got)
	}
	p.Distance = 12000
	if got := hfReliability(p, true); got != 40 {
		t.Errorf("digital beyond 9000 km %v, want the basic 40", got)
	}
}

// TestHFMapRequest maps a valid input to the model request and names the frequency of the map input in errors.
func TestHFMapRequest(t *testing.T) {
	site := store.Site{ID: 1, Lat: 45.4, Lon: 11}
	req, err := hfMapRequest(hfMapInput(), site)
	if err != nil {
		t.Fatal(err)
	}
	if req.TxLat != 45.4 || len(req.Frequencies) != 1 || req.Frequencies[0] != 14.2 || len(req.Hours) != 2 {
		t.Errorf("unexpected request %+v", req)
	}
	in := hfMapInput()
	in.FrequencyMHz = 40
	_, err = hfMapRequest(in, site)
	em, ok := err.(*huma.ErrorModel)
	if !ok || em.Status != 422 || len(em.Errors) != 1 || em.Errors[0].Location != "body.frequency_mhz" {
		t.Errorf("want one error at body.frequency_mhz, got %v", err)
	}
	in = hfMapInput()
	in.LongPath = true
	req, err = hfMapRequest(in, site)
	if err != nil {
		t.Fatal(err)
	}
	if !req.LongPath {
		t.Error("long path was not passed to the model request")
	}
}

// TestHFMapCompute fills a frame per hour with a value for every cell; it needs the ITU data set
// (scripts/fetch-hf-data.sh).
func TestHFMapCompute(t *testing.T) {
	data, err := hf.Open(hfDataDir(t), "")
	if err != nil {
		t.Fatal(err)
	}
	req, err := hfMapRequest(hfMapInput(), store.Site{ID: 1, Lat: 45.4, Lon: 11})
	if err != nil {
		t.Fatal(err)
	}
	m, err := hfMapCompute(context.Background(), data, req)
	if err != nil {
		t.Fatal(err)
	}
	if m.Rows != 18 || m.Cols != 36 || len(m.Frames) != 2 || m.Frames[0].HourUTC != 6 || m.Frames[1].HourUTC != 12 {
		t.Fatalf("unexpected map %d x %d with %d frames", m.Rows, m.Cols, len(m.Frames))
	}
	for _, f := range m.Frames {
		if len(f.ReliabilityPct) != m.Rows*m.Cols {
			t.Fatalf("hour %d has %d cells", f.HourUTC, len(f.ReliabilityPct))
		}
		known := 0
		for _, v := range f.ReliabilityPct {
			if v != hfNotAvailable {
				known++
				if v < 0 || v > 100 {
					t.Errorf("hour %d: reliability %d out of range", f.HourUTC, v)
				}
			}
		}
		if known == 0 {
			t.Errorf("hour %d: no cell was calculated", f.HourUTC)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := hfMapCompute(ctx, data, req); err == nil {
		t.Error("a cancelled request was computed")
	}
}
