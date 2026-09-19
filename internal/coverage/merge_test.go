package coverage

import (
	"math"
	"testing"
)

func grid(w, h int, n, s, e, wl, margin float64, hole func(x, y int) bool) Result {
	r := Result{W: w, H: h, North: n, South: s, East: e, West: wl, Margin: make([]float64, w*h)}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r.Margin[y*w+x] = margin
			if hole != nil && hole(x, y) {
				r.Margin[y*w+x] = math.NaN()
			}
		}
	}
	return r
}

func TestMergeBestWins(t *testing.T) {
	a := grid(10, 10, 48, 47, 9, 8, 15, func(x, _ int) bool { return x >= 8 })
	b := grid(4, 4, 48.5, 47.5, 9.5, 8.5, 5, nil)
	m := Merge([]Result{a, b})

	if m.North != 48.5 || m.South != 47 || m.East != 9.5 || m.West != 8 {
		t.Fatalf("bounds N%v S%v E%v W%v", m.North, m.South, m.East, m.West)
	}
	if m.W != 15 || m.H != 15 {
		t.Fatalf("size %dx%d, want 15x15", m.W, m.H)
	}
	at := func(lat, lon float64) float64 {
		x := int((lon - m.West) / (m.East - m.West) * float64(m.W))
		y := int((m.North - lat) / (m.North - m.South) * float64(m.H))
		return m.At(x, y)
	}
	cases := []struct {
		lat, lon, want float64
	}{
		{47.75, 8.25, 15},
		{47.75, 8.75, 15},
		{47.75, 8.9, 5},
		{48.25, 9.25, 5},
		{47.25, 9.25, math.NaN()},
	}
	for _, c := range cases {
		got := at(c.lat, c.lon)
		if math.IsNaN(c.want) != math.IsNaN(got) || (!math.IsNaN(c.want) && got != c.want) {
			t.Errorf("at %v,%v: got %v, want %v", c.lat, c.lon, got, c.want)
		}
	}
}

func TestMergeSingleAndEmpty(t *testing.T) {
	a := grid(3, 3, 48, 47, 9, 8, 7, nil)
	m := Merge([]Result{a})
	if m.W != 3 || m.H != 3 || m.At(1, 1) != 7 {
		t.Errorf("single raster changed: %+v", m)
	}
	if e := Merge(nil); e.W != 0 || e.H != 0 || len(e.Margin) != 0 {
		t.Errorf("empty merge: %+v", e)
	}
	if m := Merge([]Result{a, {}}); m.W != 3 {
		t.Errorf("empty input not ignored: %dx%d", m.W, m.H)
	}
}

func TestMergeCap(t *testing.T) {
	a := grid(2000, 2000, 48, 47, 9, 8, 1, nil)
	b := grid(2000, 2000, 48, 47, 10, 9, 1, nil)
	c := grid(2000, 2000, 49, 48, 10, 9, 1, nil)
	m := Merge([]Result{a, b, c})
	if m.W*m.H > maxMergePixels || m.W*m.H < maxMergePixels/2 {
		t.Errorf("merged %dx%d = %d cells, cap %d", m.W, m.H, m.W*m.H, maxMergePixels)
	}
	if m.W != m.H {
		t.Errorf("square span gave %dx%d", m.W, m.H)
	}
}
