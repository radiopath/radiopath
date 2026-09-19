package dem

import (
	"encoding/binary"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/radiopath/radiopath/internal/s3"
)

func writeTile(t *testing.T, dir, name string, side int, vals []int16) {
	t.Helper()
	buf := make([]byte, 2*len(vals))
	for i, v := range vals {
		binary.BigEndian.PutUint16(buf[2*i:], uint16(v))
	}
	if err := os.WriteFile(filepath.Join(dir, name+".hgt"), buf, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestTileName(t *testing.T) {
	cases := map[[2]float64]string{
		{47.5, 9.5}:    "N47E009",
		{47.0, 9.0}:    "N47E009",
		{-0.5, -1.5}:   "S01W002",
		{-33.9, 151.2}: "S34E151",
		{0.1, -0.1}:    "N00W001",
	}
	for in, want := range cases {
		if got := tileName(in[0], in[1]); got != want {
			t.Errorf("tileName(%v) = %s, want %s", in, got, want)
		}
	}
}

func TestDirSourceBilinear(t *testing.T) {
	dir := t.TempDir()
	writeTile(t, dir, "N47E009", 3, []int16{
		0, 10, 20,
		100, 110, 120,
		200, 210, 220,
	})
	s := NewDirSource(dir, 2)

	cases := []struct {
		lat, lon, want float64
	}{
		{47.5, 9.5, 110},
		{47.0, 9.0, 200},
		{47.999, 9.0, 0.2},
		{47.25, 9.25, 155},
		{47.75, 9.75, 65},
		{47.0, 9.5, 210},
	}
	for _, c := range cases {
		got, err := s.Elevation(c.lat, c.lon)
		if err != nil {
			t.Errorf("Elevation(%v,%v): %v", c.lat, c.lon, err)
			continue
		}
		if math.Abs(got-c.want) > 1e-9 {
			t.Errorf("Elevation(%v,%v) = %v, want %v", c.lat, c.lon, got, c.want)
		}
	}
}

func TestDirSourceErrors(t *testing.T) {
	dir := t.TempDir()
	writeTile(t, dir, "N47E009", 3, []int16{
		0, 10, void,
		100, 110, 120,
		200, 210, 220,
	})
	s := NewDirSource(dir, 2)

	if _, err := s.Elevation(47.9, 9.9); !errors.Is(err, ErrVoid) {
		t.Errorf("void: got %v, want ErrVoid", err)
	}
	if v, err := s.Elevation(47.4, 9.4); err != nil || v == 0 {
		t.Errorf("cell away from void: %v, %v", v, err)
	}
	if _, err := s.Elevation(10, 10); !errors.Is(err, ErrNoTile) {
		t.Errorf("missing tile: got %v, want ErrNoTile", err)
	}
	if _, err := s.Elevation(91, 0); err == nil {
		t.Error("invalid lat accepted")
	}
}

func TestDirSourceLRU(t *testing.T) {
	dir := t.TempDir()
	flat := func(v int16) []int16 { return []int16{v, v, v, v, v, v, v, v, v} }
	writeTile(t, dir, "N47E009", 3, flat(1))
	writeTile(t, dir, "N47E010", 3, flat(2))
	writeTile(t, dir, "N48E009", 3, flat(3))
	s := NewDirSource(dir, 2)

	for _, p := range [][2]float64{{47.5, 9.5}, {47.5, 10.5}, {48.5, 9.5}, {47.5, 9.5}} {
		if _, err := s.Elevation(p[0], p[1]); err != nil {
			t.Fatal(err)
		}
	}
	if s.c.lru.Len() != 2 {
		t.Errorf("cache holds %d tiles, want 2", s.c.lru.Len())
	}
	if _, ok := s.c.tiles["N47E010"]; ok {
		t.Error("least recently used tile N47E010 not evicted")
	}
}

func TestS3Source(t *testing.T) {
	buf := make([]byte, 18)
	for i, v := range []int16{0, 10, 20, 100, 110, 120, 200, 210, 220} {
		binary.BigEndian.PutUint16(buf[2*i:], uint16(v))
	}
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path == "/dem/srtm/N47/N47E009.hgt" {
			w.Write(buf)
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	s := NewS3Source(&s3.Client{Endpoint: srv.URL, Bucket: "dem"}, "srtm/", 2)
	for i := 0; i < 3; i++ {
		if h, err := s.Elevation(47.5, 9.5); err != nil || h != 110 {
			t.Errorf("Elevation = %v, %v", h, err)
		}
	}
	if requests != 1 {
		t.Errorf("tile fetched %d times, want 1", requests)
	}
	if _, err := s.Elevation(48.5, 9.5); !errors.Is(err, ErrNoTile) {
		t.Errorf("missing object: %v", err)
	}
}

func TestRealTile(t *testing.T) {
	dir := os.Getenv("RADIOPATH_DEM_DIR")
	if dir == "" {
		t.Skip("RADIOPATH_DEM_DIR not set")
	}
	s := NewDirSource(dir, 2)
	h, err := s.Elevation(47.2494, 9.3432)
	if err != nil {
		t.Fatal(err)
	}
	if h < 2440 || h > 2510 {
		t.Errorf("Saentis = %.0f m, want 2440..2510", h)
	}
}

func writeCanopy(t *testing.T, dir, name string, vals []byte) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name+".chm"), vals, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDirCanopy(t *testing.T) {
	dir := t.TempDir()
	writeCanopy(t, dir, "N47E009", []byte{
		0, 10, 20,
		30, 40, 50,
		60, 70, 80,
	})
	writeCanopy(t, dir, "N48E009", []byte{1, 2, 3})
	s := NewDirCanopy(dir, 2)

	for _, c := range []struct{ lat, lon, want float64 }{
		{47.5, 9.5, 40},
		{47.25, 9.25, 50},
		{10, 10, 0},
	} {
		got, err := s.HeightM(c.lat, c.lon)
		if err != nil || math.Abs(got-c.want) > 1e-9 {
			t.Errorf("HeightM(%v,%v) = %v, %v, want %v", c.lat, c.lon, got, err, c.want)
		}
	}
	if _, err := s.HeightM(48.5, 9.5); err == nil {
		t.Error("malformed tile accepted")
	}
}

func TestS3Canopy(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path == "/dem/canopy1/N47/N47E009.chm" {
			w.Write([]byte{0, 10, 20, 30, 40, 50, 60, 70, 80})
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	s := NewS3Canopy(&s3.Client{Endpoint: srv.URL, Bucket: "dem"}, "canopy1/", 2)
	for i := 0; i < 3; i++ {
		if h, err := s.HeightM(47.5, 9.5); err != nil || h != 40 {
			t.Errorf("HeightM = %v, %v", h, err)
		}
		if h, err := s.HeightM(48.5, 9.5); err != nil || h != 0 {
			t.Errorf("missing tile: HeightM = %v, %v", h, err)
		}
	}
	if requests != 2 {
		t.Errorf("%d requests, want 2", requests)
	}
}

func TestRealCanopy(t *testing.T) {
	dir := os.Getenv("RADIOPATH_CANOPY_DIR")
	if dir == "" {
		t.Skip("RADIOPATH_CANOPY_DIR not set")
	}
	s := NewDirCanopy(dir, 2)
	if h, err := s.HeightM(47.2494, 9.3432); err != nil || h > 3 {
		t.Errorf("Saentis canopy = %.0f m, %v", h, err)
	}
	maxH := 0.0
	for lat := 47.30; lat < 47.33; lat += 0.001 {
		for lon := 9.38; lon < 9.42; lon += 0.001 {
			h, err := s.HeightM(lat, lon)
			if err != nil {
				t.Fatal(err)
			}
			maxH = math.Max(maxH, h)
		}
	}
	if maxH < 15 {
		t.Errorf("tallest canopy near Appenzell = %.0f m, want > 15", maxH)
	}
}
