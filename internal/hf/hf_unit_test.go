package hf

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/radiopath/radiopath/internal/hf/p372"
)

// TestGreatCircleDistance checks quarter and half circles and the symmetry.
func TestGreatCircleDistance(t *testing.T) {
	a := location{0, 0}
	b := location{0, pi / 2}
	if got, want := greatCircleDistance(a, b), r0*pi/2; math.Abs(got-want) > 1e-6 {
		t.Errorf("quarter circle = %v, want %v", got, want)
	}
	if got := greatCircleDistance(a, a); got != 0 {
		t.Errorf("same point = %v, want 0", got)
	}
	c := location{41.9 * d2r, 12.5 * d2r}
	d := location{52.5 * d2r, 13.4 * d2r}
	if x, y := greatCircleDistance(c, d), greatCircleDistance(d, c); x != y {
		t.Errorf("distance not symmetric: %v != %v", x, y)
	}
	// Rome-Berlin of the reference case
	if got := greatCircleDistance(c, d); math.Abs(got-1180.602328) > 1e-6 {
		t.Errorf("Rome-Berlin = %v, want 1180.602328", got)
	}
}

// TestGreatCirclePoint checks the mid-point of an equatorial path and the
// zero distance branch.
func TestGreatCirclePoint(t *testing.T) {
	var cp controlPt
	a := location{0, 0}
	b := location{0, pi / 2}
	greatCirclePoint(a, b, &cp, r0*pi/2, 0.5)
	if math.Abs(cp.L.lng-pi/4) > 1e-12 || math.Abs(cp.L.lat) > 1e-12 || math.Abs(cp.distance-r0*pi/4) > 1e-6 {
		t.Errorf("mid-point = %+v", cp)
	}
	greatCirclePoint(a, a, &cp, 0, 0.5)
	if cp.L != a || cp.distance != 0 {
		t.Errorf("zero distance point = %+v", cp)
	}
}

// TestBearing checks the cardinal directions and the long path flip.
func TestBearing(t *testing.T) {
	o := location{0, 0}
	cases := []struct {
		to   location
		want float64
	}{
		{location{0, pi / 2}, pi / 2},
		{location{pi / 4, 0}, 0},
		{location{0, -pi / 2}, 3 * pi / 2},
		{location{-pi / 4, 0}, pi},
	}
	for _, c := range cases {
		if got := bearing(o, c.to, shortPath); math.Abs(got-c.want) > 1e-12 {
			t.Errorf("bearing to %+v = %v, want %v", c.to, got, c.want)
		}
		long := bearing(o, c.to, longPath)
		if want := math.Mod(c.want+pi, 2*pi); math.Abs(long-want) > 1e-12 {
			t.Errorf("long bearing to %+v = %v, want %v", c.to, long, want)
		}
	}
}

// TestMagfit checks the example in the header of Magfit.c.
func TestMagfit(t *testing.T) {
	var cp controlPt
	cp.L = location{0.732665, -1.534227}
	// Heights other than 100 and 300 km store at index 0, as in the C code.
	magfit(&cp, 1800)
	if math.Abs(cp.fH[0]-0.733686) > 1e-5 || math.Abs(cp.dip[0]-1.241669) > 1e-5 {
		t.Errorf("fH = %v, dip = %v; want 0.733686, 1.241669", cp.fH[0], cp.dip[0])
	}
}

// TestWhatSeason checks the season of both hemispheres.
func TestWhatSeason(t *testing.T) {
	n := location{0.5, 0}
	s := location{-0.5, 0}
	cases := []struct {
		l    location
		m    int
		want int
	}{
		{n, jan, winter}, {n, apr, equinox}, {n, jul, summer}, {n, dec, winter},
		{s, jan, summer}, {s, apr, equinox}, {s, jul, winter}, {s, nov, summer},
	}
	for _, c := range cases {
		if got := whatSeason(c.l, c.m); got != c.want {
			t.Errorf("whatSeason(%v, %d) = %d, want %d", c.l.lat, c.m, got, c.want)
		}
	}
}

// TestFloatSemantics guards the helpers that mimic C behaviour.
func TestFloatSemantics(t *testing.T) {
	if got := cMin(math.NaN(), 1); got != 1 {
		t.Errorf("cMin(NaN, 1) = %v, want 1", got)
	}
	if got := cMax(1, math.NaN()); !math.IsNaN(got) {
		t.Errorf("cMax(1, NaN) = %v, want NaN", got)
	}
	if got := cFmod(-1, 24); got != -1 {
		t.Errorf("cFmod(-1, 24) = %v, want -1", got)
	}
	if maxMds != 9 {
		t.Errorf("maxMds = %d, want 9", maxMds)
	}
}

// validPath returns a path that passes validatePath.
func validPath() *pathData {
	p := new(pathData)
	p.year = 2020
	p.SSN = 50
	p.frequency = 14.2
	p.BW = 3000
	p.SNRXXp = 90
	p.foF2 = &ionMap{}
	p.M3kF2 = &ionMap{}
	p.foF2var = new(decileFactors)
	p.coeff = new(p372.Coefficients)
	isotropicPattern(&p.A_rx, 0)
	isotropicPattern(&p.A_tx, 0)
	return p
}

// TestValidatePath maps every invalid input to its error.
func TestValidatePath(t *testing.T) {
	if err := validatePath(validPath()); err != nil {
		t.Fatalf("valid path: %v", err)
	}
	cases := []struct {
		name string
		mod  func(p *pathData)
		want error
	}{
		{"year", func(p *pathData) { p.year = 1899 }, ErrYear},
		{"month", func(p *pathData) { p.month = 12 }, ErrMonth},
		{"hour", func(p *pathData) { p.hour = 24 }, ErrHour},
		{"foF2", func(p *pathData) { p.foF2 = nil }, ErrNoFoF2Data},
		{"M3kF2", func(p *pathData) { p.M3kF2 = nil }, ErrNoM3kF2Data},
		{"coefficients", func(p *pathData) { p.coeff = nil }, ErrNoDudData},
		{"foF2var", func(p *pathData) { p.foF2var = nil }, ErrNoFoF2VarData},
		{"ssn", func(p *pathData) { p.SSN = 312 }, ErrSSN},
		{"modulation", func(p *pathData) { p.Modulation = 2 }, ErrModulation},
		{"frequency", func(p *pathData) { p.frequency = 30.5 }, ErrFrequency},
		{"bandwidth", func(p *pathData) { p.BW = 0.001 }, ErrBandwidth},
		{"power", func(p *pathData) { p.txpower = 61 }, ErrTxPower},
		{"snr", func(p *pathData) { p.SNRr = -31 }, ErrSNRRequired},
		{"sir", func(p *pathData) { p.SIRr = 201 }, ErrSIRRequired},
		{"f0", func(p *pathData) { p.F0 = 1001 }, ErrF0},
		{"t0", func(p *pathData) { p.T0 = -1 }, ErrT0},
		{"a", func(p *pathData) { p.A = 1001 }, ErrA},
		{"tw", func(p *pathData) { p.TW = 51 }, ErrTW},
		{"fw", func(p *pathData) { p.FW = 1001 }, ErrFW},
		{"tx", func(p *pathData) { p.L_tx.lat = 2 }, ErrTxLocation},
		{"rx", func(p *pathData) { p.L_rx.lng = 4 }, ErrRxLocation},
		{"rx antenna", func(p *pathData) { p.A_rx.pattern = nil }, ErrRxAntennaPattern},
		{"tx antenna", func(p *pathData) { p.A_tx.pattern = nil }, ErrTxAntennaPattern},
		{"snrxx", func(p *pathData) { p.SNRXXp = 100 }, ErrSNRXXp},
	}
	for _, c := range cases {
		p := validPath()
		c.mod(p)
		if err := validatePath(p); !errors.Is(err, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, err, c.want)
		}
	}
}

// TestRequestValidate checks the driver level checks.
func TestRequestValidate(t *testing.T) {
	ok := Request{Month: 1, Hours: []int{1, 24}, Frequencies: []float64{1, 30}}
	if err := ok.validate(); err != nil {
		t.Fatalf("valid request: %v", err)
	}
	bad := 361.0
	cases := []struct {
		name string
		mod  func(r *Request)
		want error
	}{
		{"no hours", func(r *Request) { r.Hours = nil }, ErrNoHours},
		{"hour 0", func(r *Request) { r.Hours = []int{0} }, ErrHour},
		{"hour 25", func(r *Request) { r.Hours = []int{25} }, ErrHour},
		{"no frequencies", func(r *Request) { r.Frequencies = nil }, ErrNoFrequencies},
		{"frequency", func(r *Request) { r.Frequencies = []float64{0.9} }, ErrFrequency},
		{"month", func(r *Request) { r.Month = 13 }, ErrMonth},
		{"gain offset", func(r *Request) { r.TxAntenna.GainOffset = 61 }, ErrGainOffset},
		{"bearing", func(r *Request) { r.RxAntenna.Bearing = &bad }, ErrBearing},
	}
	for _, c := range cases {
		r := ok
		c.mod(&r)
		if err := r.validate(); !errors.Is(err, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, err, c.want)
		}
	}
}

// TestScanNumbers checks the sscanf compatible number scanner with the fixed
// width columns of VOACAP antenna files.
func TestScanNumbers(t *testing.T) {
	got := scanNumbers("0    -99.999-15.670 -9.730  1.5e2 x 3")
	want := []float64{0, -99.999, -15.670, -9.730, 150}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("value %d = %v, want %v", i, got[i], want[i])
		}
	}
	if got := scanNumbers("abc"); len(got) != 0 {
		t.Errorf("scanNumbers(abc) = %v", got)
	}
}

// type11Text builds a type 11 antenna file with a gain that equals the
// elevation index, and a maximum gain of 2 dBi.
func type11Text() string {
	var b strings.Builder
	b.WriteString("Test antenna\n 3     3 parameters\n 2.00  [ 1] Max Gain dBi..:\n 11    [ 2] Antenna Type..\n -4.8   [ 3] Efficiency\n")
	for i := 0; i < 91; i++ {
		fmt.Fprintf(&b, "%8.1f", float64(i))
		if i%10 == 9 || i == 90 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// TestParseAntennaType11 reads a type 11 file and checks the gain table.
func TestParseAntennaType11(t *testing.T) {
	ant, err := parseAntenna(strings.NewReader(type11Text()), 0)
	if err != nil {
		t.Fatal(err)
	}
	if ant.name != "Test antenna" || ant.freqn != 1 {
		t.Errorf("name %q freqn %d", ant.name, ant.freqn)
	}
	for _, az := range []int{0, 100, 359} {
		for _, el := range []int{0, 10, 45, 90} {
			if got, want := ant.pattern[0][az][el], float64(el)+2.0; got != want {
				t.Errorf("pattern[%d][%d] = %v, want %v", az, el, got, want)
			}
		}
	}
}

// TestParseAntennaErrors checks the unsupported type error.
func TestParseAntennaErrors(t *testing.T) {
	_, err := parseAntenna(strings.NewReader("x\n1\n2\n 7 type\n"), 0)
	if !errors.Is(err, ErrAntennaType) {
		t.Errorf("got %v, want ErrAntennaType", err)
	}
	if _, err := readAntennaFile("no-such-file.t13", 0); !errors.Is(err, ErrOpenAntennaFile) {
		t.Errorf("got %v, want ErrOpenAntennaFile", err)
	}
}

// TestIsotropicAntennaGain checks that the gain of an isotropic pattern does
// not depend on direction or elevation.
func TestIsotropicAntennaGain(t *testing.T) {
	p := new(pathData)
	p.frequency = 14.2
	p.L_tx = location{0.7, 0.2}
	p.L_rx = location{0.9, 0.23}
	var ant antenna
	isotropicPattern(&ant, 3.5)
	for _, d := range []float64{0, 0.1, 0.5, 1.2} {
		if got := antennaGain(p, &ant, d, txToRx); math.Abs(got-3.5) > 1e-12 {
			t.Errorf("gain at %v rad = %v, want 3.5", d, got)
		}
	}
	// A negative elevation is clamped where the C code reads out of bounds.
	if got := antennaGain(p, &ant, -0.05, rxToTx); math.Abs(got-3.5) > 1e-12 {
		t.Errorf("gain at negative elevation = %v", got)
	}
}

// TestWinterAnomaly checks the limits and the peak at 60 degrees.
func TestWinterAnomaly(t *testing.T) {
	if got := winterAnomaly(20*d2r, jan); got != 0 {
		t.Errorf("below 30 degrees: %v", got)
	}
	if got := winterAnomaly(60*d2r-1e-9, jan); math.Abs(got-0.30) > 1e-6 {
		t.Errorf("peak in January north: %v", got)
	}
	if got := winterAnomaly(-60*d2r+1e-9, jul); math.Abs(got-0.30) > 1e-6 {
		t.Errorf("peak in July south: %v", got)
	}
	if got := winterAnomaly(60*d2r, jul); got != 0 {
		t.Errorf("July north: %v", got)
	}
}

// TestParseNoiseEnvironment checks the names of the noise environments.
func TestParseNoiseEnvironment(t *testing.T) {
	if v, ok := ParseNoiseEnvironment(" rural "); !ok || v != NoiseRural {
		t.Errorf("rural = %v %v", v, ok)
	}
	if _, ok := ParseNoiseEnvironment("moon"); ok {
		t.Error("unknown name accepted")
	}
}
