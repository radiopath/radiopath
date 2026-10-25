package p372

import (
	"encoding/csv"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// dataDir returns the P372 data directory or skips the test when absent.
func dataDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join("..", "..", "..", "tmp", "testfiles", "hf1")
	if root := os.Getenv("RADIOPATH_DATASET_DIR"); root != "" {
		dir = filepath.Join(root, "hf1")
	}
	if _, err := os.Stat(filepath.Join(dir, "COEFF01W.txt")); err != nil {
		if os.Getenv("HF_REQUIRE_DATA") != "" {
			t.Fatalf("P372 data not available in %s", dir)
		}
		t.Skipf("P372 data not available in %s", dir)
	}
	return dir
}

// closeRel reports whether a and b agree to the relative tolerance tol.
func closeRel(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol*math.Max(1.0, math.Abs(b))
}

// TestNoiseVectors compares Compute with output of the original C code
// (testdata/noise_vectors.csv, generated with gcc from MakeNoise.c).
func TestNoiseVectors(t *testing.T) {
	dir := dataDir(t)

	f, err := os.Open(filepath.Join("testdata", "noise_vectors.csv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatal(err)
	}

	coeffs := map[int]*Coefficients{}
	names := []string{"FaA", "DuA", "DlA", "FaM", "DuM", "DlM", "FaG", "DuG", "DlG", "FamT", "DuT", "DlT"}

	for _, row := range rows[1:] {
		v := make([]float64, len(row))
		for i, s := range row {
			v[i], err = strconv.ParseFloat(s, 64)
			if err != nil {
				t.Fatal(err)
			}
		}
		month := int(v[0])
		c, ok := coeffs[month]
		if !ok {
			c, err = LoadCoefficients(dir, month)
			if err != nil {
				t.Fatal(err)
			}
			coeffs[month] = c
		}

		n := Noise{Hour: int(v[1]), Lat: v[2] * d2r, Lng: v[3] * d2r, Frequency: v[4], ManMadeNoise: v[5]}
		if err := Compute(&n, c); err != nil {
			t.Fatal(err)
		}
		got := []float64{n.FaA, n.DuA, n.DlA, n.FaM, n.DuM, n.DlM, n.FaG, n.DuG, n.DlG, n.FamT, n.DuT, n.DlT}
		for i, name := range names {
			if !closeRel(got[i], v[6+i], 1e-9) {
				t.Errorf("row %v: %s = %.15g, want %.15g", row[:6], name, got[i], v[6+i])
			}
		}
	}
}

// TestComputeOverride checks the negative man-made noise override.
func TestComputeOverride(t *testing.T) {
	n := Noise{ManMadeNoise: -150}
	if err := Compute(&n, &Coefficients{}); err != nil {
		t.Fatal(err)
	}
	if n.FaM != -150 || n.FamT != 150 || n.FaA != 0 || n.DuT != 0 {
		t.Errorf("unexpected override result %+v", n)
	}
}

// TestComputeErrors checks the error values.
func TestComputeErrors(t *testing.T) {
	if err := Compute(&Noise{}, nil); err != ErrNoCoefficients {
		t.Errorf("got %v", err)
	}
	if _, err := LoadCoefficients(".", 12); err != ErrMonth {
		t.Errorf("got %v", err)
	}
}
