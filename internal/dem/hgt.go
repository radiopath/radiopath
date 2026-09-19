package dem

import (
	"encoding/binary"
	"fmt"
	"math"
)

const void = -32768

const defaultSpacingM = 30.0

const metersPerDegree = 111320.0

type format struct {
	ext  string
	size int
}

var (
	hgt = format{".hgt", 2}
	chm = format{".chm", 1}
)

type tile struct {
	side int
	raw  []byte
	f    format
}

func tileName(lat, lon float64) string {
	la, lo := int(math.Floor(lat)), int(math.Floor(lon))
	ns, ew := "N", "E"
	if la < 0 {
		ns, la = "S", -la
	}
	if lo < 0 {
		ew, lo = "W", -lo
	}
	return fmt.Sprintf("%s%02d%s%03d", ns, la, ew, lo)
}

func parseTile(raw []byte, name string, f format) (*tile, error) {
	n := len(raw) / f.size
	side := int(math.Sqrt(float64(n)))
	if side < 2 || side*side*f.size != len(raw) {
		return nil, fmt.Errorf("dem: %s: size %d is not a square grid of %d-byte samples", name, len(raw), f.size)
	}
	return &tile{side: side, raw: raw, f: f}, nil
}

func (t *tile) spacingM() float64 { return metersPerDegree / float64(t.side-1) }

func (t *tile) at(row, col int) (float64, bool) {
	i := (row*t.side + col) * t.f.size
	if t.f.size == 1 {
		return float64(t.raw[i]), true
	}
	v := int16(binary.BigEndian.Uint16(t.raw[i:]))
	return float64(v), v != void
}

func (t *tile) sample(lat, lon float64) (float64, error) {
	n := float64(t.side - 1)
	row := (math.Floor(lat) + 1 - lat) * n
	col := (lon - math.Floor(lon)) * n
	r0, c0 := int(row), int(col)
	if r0 >= t.side-1 {
		r0 = t.side - 2
	}
	if c0 >= t.side-1 {
		c0 = t.side - 2
	}
	fr, fc := row-float64(r0), col-float64(c0)

	var s [4]float64
	for i, rc := range [4][2]int{{r0, c0}, {r0, c0 + 1}, {r0 + 1, c0}, {r0 + 1, c0 + 1}} {
		v, ok := t.at(rc[0], rc[1])
		if !ok {
			return 0, ErrVoid
		}
		s[i] = v
	}
	top := s[0]*(1-fc) + s[1]*fc
	bot := s[2]*(1-fc) + s[3]*fc
	return top*(1-fr) + bot*fr, nil
}
