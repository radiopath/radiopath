package antenna

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

func msi(gainLine string, n int) string {
	return msiH(gainLine, n) + msiV()
}

func msiH(gainLine string, n int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "NAME Test antenna\nFREQUENCY 145\n%s\nTILT 0\nPOLARIZATION V\nHORIZONTAL %d\n", gainLine, n)
	for i := 0; i < n; i++ {
		ang := float64(i) * 360 / float64(n)
		fmt.Fprintf(&b, "%.1f %.2f\n", ang, 10*(1-math.Cos(ang*math.Pi/180)))
	}
	return b.String()
}

func msiV() string {
	var b strings.Builder
	b.WriteString("VERTICAL 360\n")
	for i := 0; i < 360; i++ {
		t := float64(i) - 3
		if t > 180 {
			t -= 360
		}
		fmt.Fprintf(&b, "%d %.2f\n", i, math.Min(12*(t/10)*(t/10), 30))
	}
	return b.String()
}

func TestParseMSIVertical(t *testing.T) {
	a, err := ParseMSI(strings.NewReader(msi("GAIN 12 dBi", 360)))
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Vertical) != Steps {
		t.Fatalf("vertical pattern has %d values, want %d", len(a.Vertical), Steps)
	}
	if got := a.Vertical.AttenuationDB(3); got != 0 {
		t.Errorf("main lobe attenuated by %v dB", got)
	}
	if got := a.Vertical.AttenuationDB(0); math.Abs(got-12*0.09) > 0.001 {
		t.Errorf("horizon attenuation = %v dB, want 1.08", got)
	}
	if got := a.Vertical.AttenuationDB(90); math.Abs(got-30) > 0.001 {
		t.Errorf("ground attenuation = %v dB, want 30", got)
	}
	a, err = ParseMSI(strings.NewReader(msiH("GAIN 12 dBi", 360)))
	if err != nil {
		t.Fatal(err)
	}
	if a.Vertical != nil {
		t.Errorf("file without VERTICAL gave a vertical pattern of %d values", len(a.Vertical))
	}
}

func TestLobe(t *testing.T) {
	var omni Lobe
	for _, d := range [][2]float64{{0, 0}, {90, 0}, {0, 90}, {180, 180}} {
		if got := omni.AttenuationDB(d[0], d[1]); got != 0 {
			t.Errorf("omni attenuates %v dB at %v", got, d)
		}
	}
	h := Sector(65, 25)
	mirror := Lobe{H: h}
	for _, deg := range []float64{0, 10, 40, 180} {
		if got, want := mirror.AttenuationDB(0, deg), h.AttenuationDB(deg); math.Abs(got-want) > 1e-9 {
			t.Errorf("mirrored at %v deg down: %v dB, want %v", deg, got, want)
		}
	}
	l := Lobe{H: h, V: Sector(10, 30)}
	if got := l.AttenuationDB(0, 0); got != 0 {
		t.Errorf("boresight attenuated by %v", got)
	}
	if got, want := l.AttenuationDB(32.5, 5), 3.0+3.0; math.Abs(got-want) > 0.001 {
		t.Errorf("3 dB points: %v dB, want %v", got, want)
	}
	if got := l.AttenuationDB(180, 90); math.Abs(got-30) > 0.001 {
		t.Errorf("back and down: %v dB, want the 30 dB cap", got)
	}
	if math.Abs(l.AttenuationDB(0, 5)-l.AttenuationDB(0, -5)) > 1e-9 {
		t.Error("asymmetric around the horizon")
	}
	v := Lobe{V: Sector(10, 30)}
	if got := v.AttenuationDB(180, 0); got != 0 {
		t.Errorf("no azimuth cut but %v dB behind", got)
	}
	if got := v.AttenuationDB(0, 20); math.Abs(got-30) > 0.001 {
		t.Errorf("20 deg down: %v dB, want 30", got)
	}
}

func TestParseMSI(t *testing.T) {
	a, err := ParseMSI(strings.NewReader(msi("GAIN 14.15 dBd", 360)))
	if err != nil {
		t.Fatal(err)
	}
	if a.Name != "Test antenna" {
		t.Errorf("name = %q", a.Name)
	}
	if math.Abs(a.GainDBi-16.30) > 0.001 {
		t.Errorf("gain = %.3f dBi, want 16.30 (14.15 dBd)", a.GainDBi)
	}
	if a.FreqMHz != 145 {
		t.Errorf("frequency = %v", a.FreqMHz)
	}
	if len(a.Horizontal) != Steps {
		t.Fatalf("pattern has %d values, want %d", len(a.Horizontal), Steps)
	}
	if got := a.Horizontal.AttenuationDB(0); math.Abs(got) > 0.001 {
		t.Errorf("main lobe attenuated by %.3f dB", got)
	}
	if got := a.Horizontal.AttenuationDB(180); math.Abs(got-20) > 0.001 {
		t.Errorf("back attenuation = %.3f dB, want 20", got)
	}
	if got := a.Horizontal.Front(); math.Abs(got-20) > 0.001 {
		t.Errorf("front-to-back = %.3f dB, want 20", got)
	}
}

func TestParseMSIGainDBi(t *testing.T) {
	a, err := ParseMSI(strings.NewReader(msi("GAIN 12 dBi", 360)))
	if err != nil {
		t.Fatal(err)
	}
	if a.GainDBi != 12 {
		t.Errorf("gain = %v dBi, want 12", a.GainDBi)
	}
}

func TestParseMSIResample(t *testing.T) {
	a, err := ParseMSI(strings.NewReader(msi("GAIN 10", 720)))
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Horizontal) != Steps {
		t.Fatalf("pattern has %d values", len(a.Horizontal))
	}
	for _, deg := range []float64{0, 45, 90, 180, 270, 359} {
		want := 10 * (1 - math.Cos(deg*math.Pi/180))
		if got := a.Horizontal.AttenuationDB(deg); math.Abs(got-want) > 0.01 {
			t.Errorf("at %.0f deg: %.3f dB, want %.3f", deg, got, want)
		}
	}
}

func TestAttenuationWraps(t *testing.T) {
	p := make(Pattern, Steps)
	for i := range p {
		p[i] = float64(i)
	}
	cases := []struct{ in, want float64 }{
		{0, 0}, {90.5, 90.5}, {360, 0}, {-1, 359}, {720 + 45, 45},
	}
	for _, c := range cases {
		if got := p.AttenuationDB(c.in); math.Abs(got-c.want) > 0.001 {
			t.Errorf("AttenuationDB(%v) = %v, want %v", c.in, got, c.want)
		}
	}
	var nilPattern Pattern
	if got := nilPattern.AttenuationDB(123); got != 0 {
		t.Errorf("nil pattern attenuates %v dB", got)
	}
}

func TestParseMSIErrors(t *testing.T) {
	cases := map[string]string{
		"no gain":       "NAME x\nHORIZONTAL 2\n0 0\n1 0\n",
		"no pattern":    "NAME x\nGAIN 3 dBd\n",
		"bad count":     "NAME x\nGAIN 3 dBd\nHORIZONTAL abc\n",
		"short pattern": "NAME x\nGAIN 3 dBd\nHORIZONTAL 2\n0\n",
	}
	for name, in := range cases {
		if _, err := ParseMSI(strings.NewReader(in)); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestSector(t *testing.T) {
	p := Sector(65, 25)
	if len(p) != Steps {
		t.Fatalf("pattern has %d values", len(p))
	}
	for _, deg := range []float64{32.5, -32.5} {
		if got := p.AttenuationDB(deg); math.Abs(got-3) > 0.001 {
			t.Errorf("at %.1f deg: %.3f dB, want 3", deg, got)
		}
	}
	if got := p.AttenuationDB(0); got != 0 {
		t.Errorf("main lobe attenuated by %v", got)
	}
	if got := p.Front(); math.Abs(got-25) > 0.001 {
		t.Errorf("front-to-back = %v, want 25", got)
	}
	if got := p.AttenuationDB(180); math.Abs(got-25) > 0.001 {
		t.Errorf("back attenuation = %v, want 25", got)
	}
	for _, deg := range []float64{10, 45, 90, 140} {
		if math.Abs(p.AttenuationDB(deg)-p.AttenuationDB(-deg)) > 1e-9 {
			t.Errorf("asymmetric at %.0f deg", deg)
		}
	}
	for _, om := range []Pattern{Sector(360, 25), Sector(65, 0), Sector(0, 25)} {
		if om.Front() != 0 {
			t.Errorf("expected an omni pattern, front-to-back %v", om.Front())
		}
	}
}

func TestSectorParams(t *testing.T) {
	for _, c := range []struct{ beam, fb float64 }{{65, 25}, {90, 30}, {120, 18}, {30, 40}, {1, 60}, {359, 2}} {
		bw, fb := SectorParams(Sector(c.beam, c.fb))
		if math.Abs(bw-c.beam) > 0.001 || math.Abs(fb-c.fb) > 0.001 {
			t.Errorf("Sector(%v, %v): got %v deg, %v dB", c.beam, c.fb, bw, fb)
		}
		if got, want := Sector(bw, fb), Sector(c.beam, c.fb); !patternsEqual(got, want) {
			t.Errorf("Sector(%v, %v) does not round-trip", c.beam, c.fb)
		}
	}
	bw, fb := SectorParams(Sector(200, 25))
	if math.Abs(bw-200) > 0.001 || math.Abs(fb-9.72) > 0.001 {
		t.Errorf("Sector(200, 25): got %v deg, %v dB", bw, fb)
	}
	for _, om := range []Pattern{Sector(360, 25), Sector(65, 0), nil} {
		if bw, fb := SectorParams(om); bw != 360 || fb != 0 {
			t.Errorf("omni: got %v deg, %v dB", bw, fb)
		}
	}
}

func patternsEqual(a, b Pattern) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if math.Abs(a[i]-b[i]) > 1e-6 {
			return false
		}
	}
	return true
}
