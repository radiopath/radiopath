package plot

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"sort"
	"strconv"

	"github.com/radiopath/radiopath/internal/antenna"
	"github.com/radiopath/radiopath/internal/coverage"
	"github.com/radiopath/radiopath/internal/link"
)

const (
	width, height = 900, 400
	left, right   = 60, 20
	top, bottom   = 20, 40
)

func SVG(r link.Result, antAM, antBM float64) []byte {
	p := r.Profile
	if len(p) < 2 {
		return []byte(`<svg xmlns="http://www.w3.org/2000/svg" class="profile-svg" width="900" height="40"><text class="tick" x="10" y="25">no profile</text></svg>`)
	}
	n := len(p)
	dist := p[n-1].DistM

	ymin, ymax := math.Inf(1), math.Inf(-1)
	hasCanopy := false
	for _, s := range p {
		ymin = math.Min(ymin, s.TerrainM+s.BulgeM)
		ymin = math.Min(ymin, s.RayM-s.FresnelM)
		ymax = math.Max(ymax, s.TerrainM+s.BulgeM+s.CanopyM)
		ymax = math.Max(ymax, s.RayM+s.FresnelM)
		hasCanopy = hasCanopy || s.CanopyM > 0
	}
	ymax = math.Max(ymax, p[0].TerrainM+antAM)
	ymax = math.Max(ymax, p[n-1].TerrainM+antBM)
	pad := math.Max(10, (ymax-ymin)*0.05)
	ymin, ymax = ymin-pad, ymax+pad

	plotW := float64(width - left - right)
	plotH := float64(height - top - bottom)
	x := func(d float64) float64 { return left + d/dist*plotW }
	y := func(h float64) float64 { return top + (ymax-h)/(ymax-ymin)*plotH }

	var b bytes.Buffer
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" class="profile-svg" width="%d" height="%d" viewBox="0 0 %d %d" font-family="monospace" font-size="11">`, width, height, width, height)

	b.WriteString(`<polygon class="terrain" fill="#c8b58a" stroke="#6b5a34" points="`)
	fmt.Fprintf(&b, "%.1f,%.1f ", x(0), y(ymin))
	for _, s := range p {
		fmt.Fprintf(&b, "%.1f,%.1f ", x(s.DistM), y(s.TerrainM+s.BulgeM))
	}
	fmt.Fprintf(&b, "%.1f,%.1f", x(dist), y(ymin))
	b.WriteString(`"/>`)

	if hasCanopy {
		b.WriteString(`<polygon class="canopy" fill="#5e9c4f" stroke="none" points="`)
		for _, s := range p {
			fmt.Fprintf(&b, "%.1f,%.1f ", x(s.DistM), y(s.TerrainM+s.BulgeM+s.CanopyM))
		}
		for i := n - 1; i >= 0; i-- {
			fmt.Fprintf(&b, "%.1f,%.1f ", x(p[i].DistM), y(p[i].TerrainM+p[i].BulgeM))
		}
		b.WriteString(`"/>`)
	}

	for _, sign := range []float64{1, -1} {
		b.WriteString(`<polyline class="fresnel" fill="none" stroke="#2a7fd4" stroke-dasharray="4 3" points="`)
		for _, s := range p {
			fmt.Fprintf(&b, "%.1f,%.1f ", x(s.DistM), y(s.RayM+sign*s.FresnelM))
		}
		b.WriteString(`"/>`)
	}

	b.WriteString(`<polyline class="ray" fill="none" stroke="#d42a2a" stroke-width="1.5" points="`)
	fmt.Fprintf(&b, "%.1f,%.1f %.1f,%.1f", x(0), y(p[0].RayM), x(dist), y(p[n-1].RayM))
	b.WriteString(`"/>`)

	fmt.Fprintf(&b, `<line class="mast" x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="black" stroke-width="2"/>`, x(0), y(p[0].TerrainM), x(0), y(p[0].TerrainM+antAM))
	fmt.Fprintf(&b, `<line class="mast" x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="black" stroke-width="2"/>`, x(dist), y(p[n-1].TerrainM), x(dist), y(p[n-1].TerrainM+antBM))

	fmt.Fprintf(&b, `<line class="axis" x1="%d" y1="%d" x2="%d" y2="%d" stroke="black"/>`, left, top, left, height-bottom)
	fmt.Fprintf(&b, `<line class="axis" x1="%d" y1="%d" x2="%d" y2="%d" stroke="black"/>`, left, height-bottom, width-right, height-bottom)
	for i := 0; i <= 5; i++ {
		d := dist * float64(i) / 5
		fmt.Fprintf(&b, `<text class="tick" x="%.1f" y="%d" text-anchor="middle">%.1f km</text>`, x(d), height-bottom+15, d/1000)
		h := ymin + (ymax-ymin)*float64(i)/5
		fmt.Fprintf(&b, `<text class="tick" x="%d" y="%.1f" text-anchor="end">%.0f m</text>`, left-5, y(h)+4, h)
	}
	b.WriteString(`</svg>`)
	return b.Bytes()
}

type Bin struct {
	MinDB float64
	Color color.RGBA
}

type Legend []Bin

const (
	MaxBins     = 8
	MinLegendDB = -127
	MaxLegendDB = 127
)

func DefaultLegend() Legend {
	return Legend{
		{30, color.RGBA{0, 100, 0, 255}},
		{20, color.RGBA{60, 180, 60, 255}},
		{10, color.RGBA{230, 220, 40, 255}},
		{0, color.RGBA{240, 140, 30, 255}},
	}
}

func NewLegend(mins []float64, colors []string) (Legend, error) {
	if len(mins) != len(colors) {
		return nil, fmt.Errorf("legend: %d thresholds but %d colours", len(mins), len(colors))
	}
	if len(mins) == 0 || len(mins) > MaxBins {
		return nil, fmt.Errorf("legend: needs 1 to %d thresholds, got %d", MaxBins, len(mins))
	}
	l := make(Legend, len(mins))
	for i, m := range mins {
		if math.IsNaN(m) || m < MinLegendDB || m > MaxLegendDB {
			return nil, fmt.Errorf("legend: threshold %s dB is outside %d..%d", db(m), MinLegendDB, MaxLegendDB)
		}
		c, err := ParseHex(colors[i])
		if err != nil {
			return nil, err
		}
		l[i] = Bin{m, c}
	}
	sort.SliceStable(l, func(i, j int) bool { return l[i].MinDB > l[j].MinDB })
	for i := 1; i < len(l); i++ {
		if l[i].MinDB == l[i-1].MinDB {
			return nil, fmt.Errorf("legend: threshold %s dB is listed twice", db(l[i].MinDB))
		}
	}
	return l, nil
}

func LegendOr(mins []float64, colors []string) Legend {
	if len(mins) == 0 {
		return DefaultLegend()
	}
	l, err := NewLegend(mins, colors)
	if err != nil {
		return DefaultLegend()
	}
	return l
}

func (l Legend) Mins() []float64 {
	out := make([]float64, len(l))
	for i, b := range l {
		out[i] = b.MinDB
	}
	return out
}

func (l Legend) Hex() []string {
	out := make([]string, len(l))
	for i, b := range l {
		out[i] = b.Hex()
	}
	return out
}

func (l Legend) Label(i int) string {
	if i == 0 {
		return ">= " + db(l[0].MinDB) + " dB"
	}
	return db(l[i].MinDB) + " to " + db(l[i-1].MinDB) + " dB"
}

func (l Legend) Floor() string {
	return "below " + db(l[len(l)-1].MinDB) + " dB / no data"
}

func (l Legend) bin(m float64) int {
	if math.IsNaN(m) {
		return -1
	}
	for i, b := range l {
		if m >= b.MinDB {
			return i
		}
	}
	return -1
}

func db(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

func (b Bin) Hex() string { return fmt.Sprintf("#%02x%02x%02x", b.Color.R, b.Color.G, b.Color.B) }

func ParseHex(s string) (color.RGBA, error) {
	var c color.RGBA
	if len(s) != 7 || s[0] != '#' {
		return c, fmt.Errorf("legend: colour %q is not #rrggbb", s)
	}
	v, err := strconv.ParseUint(s[1:], 16, 32)
	if err != nil {
		return c, fmt.Errorf("legend: colour %q is not #rrggbb", s)
	}
	return color.RGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 255}, nil
}

func CoveragePNG(r coverage.Result, l Legend) []byte { return paint(r, l, true) }

func CompositePNG(r coverage.Result, l Legend) []byte { return paint(r, l, false) }

func paint(r coverage.Result, l Legend, cross bool) []byte {
	pal := color.Palette{color.RGBA{0, 0, 0, 0}}
	for _, b := range l {
		pal = append(pal, b.Color)
	}
	pal = append(pal, color.RGBA{0, 0, 0, 255})
	crossIdx := uint8(len(pal) - 1)

	img := image.NewPaletted(image.Rect(0, 0, r.W, r.H), pal)
	for y := 0; y < r.H; y++ {
		for x := 0; x < r.W; x++ {
			if i := l.bin(r.At(x, y)); i >= 0 {
				img.SetColorIndex(x, y, uint8(i+1))
			}
		}
	}
	if cross {
		cx, cy := r.W/2, r.H/2
		for d := -4; d <= 4; d++ {
			img.SetColorIndex(cx+d, cy, crossIdx)
			img.SetColorIndex(cx, cy+d, crossIdx)
		}
	}
	var b bytes.Buffer
	png.Encode(&b, img)
	return b.Bytes()
}

const marginOffset = 128 // grey = floor(margin dB) + offset, clamped 1..255; 0 = no data

func MarginPNG(r coverage.Result) []byte {
	img := image.NewGray(image.Rect(0, 0, r.W, r.H))
	for y := 0; y < r.H; y++ {
		for x := 0; x < r.W; x++ {
			m := r.At(x, y)
			if math.IsNaN(m) {
				continue
			}
			img.Pix[y*img.Stride+x] = uint8(math.Min(255, math.Max(1, math.Floor(m)+marginOffset)))
		}
	}
	var b bytes.Buffer
	png.Encode(&b, img)
	return b.Bytes()
}

func DecodeRaster(b []byte, north, south, east, west float64) (coverage.Result, error) {
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		return coverage.Result{}, fmt.Errorf("raster: %w", err)
	}
	bd := img.Bounds()
	r := coverage.Result{W: bd.Dx(), H: bd.Dy(), North: north, South: south, East: east, West: west}
	r.Margin = make([]float64, r.W*r.H)
	for i := range r.Margin {
		r.Margin[i] = math.NaN()
	}
	switch im := img.(type) {
	case *image.Gray:
		for y := 0; y < r.H; y++ {
			for x := 0; x < r.W; x++ {
				if v := im.Pix[y*im.Stride+x]; v != 0 {
					r.Margin[y*r.W+x] = float64(v) - marginOffset
				}
			}
		}
	case *image.Paletted:
		l := DefaultLegend()
		for y := 0; y < r.H; y++ {
			for x := 0; x < r.W; x++ {
				if i := int(im.Pix[y*im.Stride+x]); i >= 1 && i <= len(l) {
					r.Margin[y*r.W+x] = l[i-1].MinDB
				}
			}
		}
	default:
		return coverage.Result{}, fmt.Errorf("raster: unexpected image type %T", img)
	}
	return r, nil
}

func PatternSVG(p antenna.Pattern, size int) []byte { return patternSVG(p, size, 0) }

func ElevationSVG(p antenna.Pattern, size int) []byte { return patternSVG(p, size, 90) }

func patternSVG(p antenna.Pattern, size int, rotDeg float64) []byte {
	const rangeDB = 30.0
	c := float64(size) / 2
	rad := c - 1

	r := func(att float64) float64 {
		v := 1 - math.Min(math.Max(att, 0), rangeDB)/rangeDB
		return v * rad
	}

	var b bytes.Buffer
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" class="pattern-svg" width="%d" height="%d" viewBox="0 0 %d %d">`,
		size, size, size, size)
	for d := 10.0; d < rangeDB; d += 10 {
		fmt.Fprintf(&b, `<circle class="ring" cx="%.1f" cy="%.1f" r="%.1f" fill="none"/>`, c, c, r(d))
	}
	fmt.Fprintf(&b, `<circle class="ring outer" cx="%.1f" cy="%.1f" r="%.1f" fill="none"/>`, c, c, rad)
	fmt.Fprintf(&b, `<line class="ring" x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f"/>`, c, c-rad, c, c+rad)
	fmt.Fprintf(&b, `<line class="ring" x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f"/>`, c-rad, c, c+rad, c)

	b.WriteString(`<polygon class="lobe" points="`)
	for i := 0; i < antenna.Steps; i++ {
		a := (float64(i) + rotDeg) * math.Pi / 180
		rr := r(p.AttenuationDB(float64(i)))
		fmt.Fprintf(&b, "%.1f,%.1f ", c+rr*math.Sin(a), c-rr*math.Cos(a))
	}
	b.WriteString(`"/>`)
	b.WriteString(`</svg>`)
	return b.Bytes()
}
