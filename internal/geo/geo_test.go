package geo

import (
	"math"
	"testing"
)

var (
	saentis   = Point{47.2494, 9.3432}
	uetliberg = Point{47.3497, 8.4911}
)

func near(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

func TestDistance(t *testing.T) {
	if d := Distance(saentis, uetliberg); !near(d, 65400, 300) {
		t.Errorf("Saentis-Uetliberg = %.0f m, want ~65400", d)
	}
	if d := Distance(Point{0, 0}, Point{0, 1}); !near(d, 111195, 5) {
		t.Errorf("1 deg equator = %.0f m, want ~111195", d)
	}
	if d := Distance(Point{10, 5}, Point{11, 5}); !near(d, 111195, 5) {
		t.Errorf("1 deg meridian = %.0f m, want ~111195", d)
	}
	if d := Distance(saentis, saentis); d != 0 {
		t.Errorf("zero distance = %v", d)
	}
}

func TestInitialBearing(t *testing.T) {
	cases := []struct {
		a, b Point
		want float64
	}{
		{Point{0, 0}, Point{1, 0}, 0},
		{Point{0, 0}, Point{0, 1}, 90},
		{Point{0, 0}, Point{-1, 0}, 180},
		{Point{0, 0}, Point{0, -1}, 270},
	}
	for _, c := range cases {
		if got := InitialBearing(c.a, c.b); !near(got, c.want, 1e-9) {
			t.Errorf("bearing %v->%v = %v, want %v", c.a, c.b, got, c.want)
		}
	}
	if b := InitialBearing(saentis, uetliberg); !near(b, 279, 1.5) {
		t.Errorf("Saentis->Uetliberg bearing = %.1f, want ~279", b)
	}
}

func TestPointAt(t *testing.T) {
	if p := PointAt(saentis, uetliberg, 0); p != saentis {
		t.Errorf("frac 0 = %v", p)
	}
	if p := PointAt(saentis, uetliberg, 1); p != uetliberg {
		t.Errorf("frac 1 = %v", p)
	}
	mid := PointAt(saentis, uetliberg, 0.5)
	d1, d2 := Distance(saentis, mid), Distance(mid, uetliberg)
	if !near(d1, d2, 0.01) {
		t.Errorf("midpoint not equidistant: %v vs %v", d1, d2)
	}
	if !near(d1+d2, Distance(saentis, uetliberg), 0.01) {
		t.Errorf("midpoint not on great circle")
	}
	if p := PointAt(Point{0, 0}, Point{0, 10}, 0.3); !near(p.Lat, 0, 1e-9) || !near(p.Lon, 3, 1e-9) {
		t.Errorf("equator frac 0.3 = %v, want {0 3}", p)
	}
}

func TestDestination(t *testing.T) {
	for _, brg := range []float64{0, 45, 90, 180, 270, 359} {
		p := Destination(saentis, brg, 20000)
		if d := Distance(saentis, p); !near(d, 20000, 0.01) {
			t.Errorf("bearing %v: distance %v", brg, d)
		}
		if b := InitialBearing(saentis, p); !near(math.Mod(b-brg+540, 360)-180, 0, 1e-6) {
			t.Errorf("bearing %v: got %v", brg, b)
		}
	}
	if p := Destination(Point{0, 0}, 90, 111195); !near(p.Lat, 0, 1e-9) || !near(p.Lon, 1, 1e-4) {
		t.Errorf("1 deg east on equator = %v", p)
	}
	if p := Destination(Point{0, 179.9}, 90, 30000); p.Lon > 0 {
		t.Errorf("no wrap: %v", p)
	}
}
