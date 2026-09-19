package itm

import (
	"encoding/csv"
	"math"
	"os"
	"strconv"
	"testing"
)

func readCSV(t *testing.T, name string) [][]string {
	t.Helper()
	f, err := os.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func atof(t *testing.T, s string) float64 {
	t.Helper()
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestNTIAVectors(t *testing.T) {
	cases := readCSV(t, "testdata/p2p.csv")[1:]
	pfls := readCSV(t, "testdata/pfls.csv")
	if len(cases) != len(pfls) {
		t.Fatalf("%d cases but %d profiles", len(cases), len(pfls))
	}
	for i, c := range cases {
		pfl := make([]float64, len(pfls[i]))
		for j, s := range pfls[i] {
			pfl[j] = atof(t, s)
		}
		p := Params{
			HTxM:         atof(t, c[0]),
			HRxM:         atof(t, c[1]),
			Epsilon:      atof(t, c[2]),
			Sigma:        atof(t, c[3]),
			N0:           atof(t, c[4]),
			FreqMHz:      atof(t, c[5]),
			Polarization: Polarization(atof(t, c[6])),
			Climate:      Climate(atof(t, c[7])),
			Time:         atof(t, c[8]),
			Location:     atof(t, c[9]),
			Situation:    atof(t, c[10]),
			Mdvar:        int(atof(t, c[11])),
		}
		want := atof(t, c[12])

		r, err := pointToPoint(pfl, p)
		if err != nil {
			t.Errorf("case %d: %v", i, err)
			continue
		}
		if math.Abs(r.LossDB-want) > 0.01 {
			t.Errorf("case %d: loss = %.4f dB, want %.2f (mode %s, warnings %#x)", i, r.LossDB, want, r.Mode, r.Warnings)
		}
	}
}

func TestPointToPointFlat(t *testing.T) {
	elev := make([]float64, 334)
	for i := range elev {
		elev[i] = 500
	}
	p := Params{HTxM: 10, HRxM: 10, FreqMHz: 145, Polarization: Vertical, Epsilon: 15, Sigma: 0.005,
		N0: 301, Climate: ContinentalTemperate, Mdvar: 12, Time: 50, Location: 50, Situation: 50}
	r, err := PointToPoint(elev, 30, p)
	if err != nil {
		t.Fatal(err)
	}
	if r.Mode != ModeLineOfSight {
		t.Errorf("mode = %s, want line of sight", r.Mode)
	}
	fs := FreeSpaceLoss(333*30, 145)
	if r.LossDB < fs-1 || r.LossDB > fs+30 {
		t.Errorf("loss = %.1f dB, free space %.1f dB", r.LossDB, fs)
	}
}

func TestPointToPointErrors(t *testing.T) {
	elev := []float64{100, 100, 100}
	base := Params{HTxM: 10, HRxM: 10, FreqMHz: 145, Polarization: Vertical, Epsilon: 15, Sigma: 0.005,
		N0: 301, Climate: ContinentalTemperate, Mdvar: 12, Time: 50, Location: 50, Situation: 50}

	p := base
	p.FreqMHz = 10
	if _, err := PointToPoint(elev, 1000, p); err != ErrFrequency {
		t.Errorf("freq 10 MHz: %v", err)
	}
	p = base
	p.HTxM = 0.1
	if _, err := PointToPoint(elev, 1000, p); err != ErrTxTerminalHeight {
		t.Errorf("h_tx 0.1: %v", err)
	}
	if _, err := PointToPoint([]float64{1}, 30, base); err == nil {
		t.Error("single point accepted")
	}
}

func TestFreeSpaceLoss(t *testing.T) {
	if got := FreeSpaceLoss(1000, 1000); math.Abs(got-92.45) > 1e-9 {
		t.Errorf("FSL = %v", got)
	}
}

func TestTakeoff(t *testing.T) {
	p := Params{HTxM: 10, HRxM: 10, FreqMHz: 145, Polarization: Vertical, Epsilon: 15, Sigma: 0.005,
		N0: 301, Climate: ContinentalTemperate, Mdvar: 12, Time: 50, Location: 50, Situation: 50}
	flat := make([]float64, 101)
	for i := range flat {
		flat[i] = 500
	}
	r, err := PointToPoint(flat, 100, p)
	if err != nil {
		t.Fatal(err)
	}
	for i, v := range r.TakeoffRad {
		if v > 0 || v < -1e-3 || math.Abs(v-r.TakeoffRad[1-i]) > 1e-12 {
			t.Errorf("flat path: takeoff[%d] = %v rad", i, v)
		}
	}
	if r.TakeoffRad == r.ThetaHzn {
		t.Errorf("takeoff angles equal the smoothed horizon angles %v", r.ThetaHzn)
	}

	high := p
	high.HTxM = 110
	r, err = PointToPoint(flat, 100, high)
	if err != nil {
		t.Fatal(err)
	}
	if r.TakeoffRad[0] >= 0 || r.TakeoffRad[1] <= 0 {
		t.Errorf("high TX: takeoff = %v", r.TakeoffRad)
	}
	want := -100.0 / 10000
	if got := r.TakeoffRad[0]; math.Abs(got-want) > 1e-3 {
		t.Errorf("high TX: takeoff[0] = %v, want about %v", got, want)
	}

	ridge := append([]float64(nil), flat...)
	ridge[50] = 700
	r, err = PointToPoint(ridge, 100, p)
	if err != nil {
		t.Fatal(err)
	}
	want = (700 - 510) / 5000.0
	for i, v := range r.TakeoffRad {
		if math.Abs(v-want) > 1e-3 {
			t.Errorf("ridge: takeoff[%d] = %v, want about %v", i, v, want)
		}
	}
}
