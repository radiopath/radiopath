package antenna

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
)

const Steps = 360

const dBdToDBi = 2.15 // half-wave dipole over isotropic

type Pattern []float64

type Antenna struct {
	Name       string
	GainDBi    float64
	FreqMHz    float64
	Horizontal Pattern
	Vertical   Pattern
}

type Lobe struct {
	H, V Pattern
}

func (l Lobe) AttenuationDB(azRel, elRel float64) float64 {
	v := l.V
	if len(v) != Steps {
		v = l.H
	}
	return math.Min(l.H.AttenuationDB(azRel)+v.AttenuationDB(elRel), math.Max(l.H.Front(), v.Front()))
}

func (p Pattern) AttenuationDB(relDeg float64) float64 {
	if len(p) != Steps || math.IsNaN(relDeg) {
		return 0
	}
	d := math.Mod(relDeg, 360)
	if d < 0 {
		d += 360
	}
	i := int(d)
	frac := d - float64(i)
	return p[i]*(1-frac) + p[(i+1)%Steps]*frac
}

func (p Pattern) Front() float64 {
	worst := 0.0
	for _, v := range p {
		worst = math.Max(worst, v)
	}
	return worst
}

func ParseMSI(r io.Reader) (Antenna, error) {
	var a Antenna
	var gainUnit string
	var gain float64
	var haveGain bool
	angles, values := []float64{}, []float64{}
	vAngles, vValues := []float64{}, []float64{}

	sc := bufio.NewScanner(io.LimitReader(r, 1<<20))
	sc.Buffer(make([]byte, 0, 64*1024), 64*1024)
	section, left := "", 0
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		f := strings.Fields(line)

		if left > 0 {
			if len(f) < 2 {
				return a, fmt.Errorf("antenna: bad pattern line %q", line)
			}
			ang, err1 := strconv.ParseFloat(f[0], 64)
			val, err2 := strconv.ParseFloat(f[1], 64)
			if err1 != nil || err2 != nil {
				return a, fmt.Errorf("antenna: bad pattern line %q", line)
			}
			switch section {
			case "HORIZONTAL":
				angles = append(angles, ang)
				values = append(values, val)
			case "VERTICAL":
				vAngles = append(vAngles, ang)
				vValues = append(vValues, val)
			}
			left--
			continue
		}

		switch strings.ToUpper(f[0]) {
		case "NAME":
			a.Name = strings.TrimSpace(strings.TrimPrefix(line, f[0]))
		case "FREQUENCY":
			if len(f) > 1 {
				a.FreqMHz, _ = strconv.ParseFloat(f[1], 64)
			}
		case "GAIN":
			if len(f) < 2 {
				return a, fmt.Errorf("antenna: GAIN without a value")
			}
			v, err := strconv.ParseFloat(f[1], 64)
			if err != nil {
				return a, fmt.Errorf("antenna: bad GAIN %q", f[1])
			}
			gain, haveGain = v, true
			if len(f) > 2 {
				gainUnit = strings.ToLower(f[2])
			}
		case "HORIZONTAL", "VERTICAL":
			if len(f) < 2 {
				return a, fmt.Errorf("antenna: %s without a count", f[0])
			}
			n, err := strconv.Atoi(f[1])
			if err != nil || n < 2 || n > 3600 {
				return a, fmt.Errorf("antenna: bad %s count %q", f[0], f[1])
			}
			section, left = strings.ToUpper(f[0]), n
		}
	}
	if err := sc.Err(); err != nil {
		return a, fmt.Errorf("antenna: %w", err)
	}
	if len(angles) == 0 {
		return a, fmt.Errorf("antenna: no HORIZONTAL pattern in the file")
	}
	if !haveGain {
		return a, fmt.Errorf("antenna: no GAIN in the file")
	}

	a.GainDBi = gain
	if !strings.HasPrefix(gainUnit, "dbi") {
		a.GainDBi = gain + dBdToDBi
	}

	p, err := resample(angles, values)
	if err != nil {
		return a, err
	}
	a.Horizontal = p
	if len(vAngles) > 0 {
		if a.Vertical, err = resample(vAngles, vValues); err != nil {
			return a, err
		}
	}
	return a, nil
}

func resample(angles, values []float64) (Pattern, error) {
	type sample struct{ ang, val float64 }
	ss := make([]sample, 0, len(angles))
	best := math.Inf(1)
	for i, a := range angles {
		v := values[i]
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return nil, fmt.Errorf("antenna: pattern holds a non-finite value")
		}
		ss = append(ss, sample{math.Mod(math.Mod(a, 360)+360, 360), v})
		best = math.Min(best, v)
	}
	sort.Slice(ss, func(i, j int) bool { return ss[i].ang < ss[j].ang })

	out := make(Pattern, Steps)
	last := len(ss) - 1
	for d := range out {
		x := float64(d)
		lo, hi := ss[last], ss[0]
		switch {
		case x < ss[0].ang:
			lo.ang -= 360
		case x >= ss[last].ang:
			hi.ang += 360
		default:
			i := sort.Search(len(ss), func(k int) bool { return ss[k].ang > x })
			lo, hi = ss[i-1], ss[i]
		}
		f := 0.0
		if span := hi.ang - lo.ang; span > 0 {
			f = (x - lo.ang) / span
		}
		out[d] = lo.val*(1-f) + hi.val*f - best
	}
	return out, nil
}

func Sector(beamwidthDeg, frontToBackDB float64) Pattern {
	p := make(Pattern, Steps)
	if beamwidthDeg <= 0 || beamwidthDeg >= 360 || frontToBackDB <= 0 {
		return p
	}
	for i := range p {
		t := float64(i)
		if t > 180 {
			t -= 360
		}
		p[i] = math.Min(12*(t/beamwidthDeg)*(t/beamwidthDeg), frontToBackDB)
	}
	return p
}

func SectorParams(p Pattern) (beamwidthDeg, frontToBackDB float64) {
	fb := p.Front()
	if len(p) != Steps || fb <= 0 {
		return 360, 0
	}
	for t := 180; t >= 1; t-- {
		if p[t] < fb && p[t] > 0 {
			bw := float64(t) * math.Sqrt(12/p[t])
			return math.Round(bw*1000) / 1000, math.Round(fb*1000) / 1000
		}
	}
	return 360, math.Round(fb*1000) / 1000
}
