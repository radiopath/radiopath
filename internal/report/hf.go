package report

import (
	"fmt"
	"image/color"
	"math"
	"slices"
	"time"

	"github.com/go-pdf/fpdf"

	"github.com/radiopath/radiopath/internal/store"
)

// hfNA is the value the HF model reports for a quantity that does not apply to the path.
const hfNA = -307

// hfPalette is one colour per frequency, in the order of the HF page.
var hfPalette = []color.RGBA{accent, ok, warn, ray, fresnel, canopy}

type HFPoint struct {
	Hour                int // UTC, 1 to 24; 24 is 00:00
	FreqMHz             float64
	MUF10, MUF50, MUF90 float64
	ReliabilityPct      float64
	SNRDB               float64
	Mode                string
}

type HFMonth struct {
	Year, Month, SSN int
	Points           []HFPoint
}

type HF struct {
	SiteA, SiteB store.Site
	Radio        [][]string // label and value rows of the conditions
	Months       []HFMonth
	User         string
	Generated    time.Time
}

// HFPDF renders the HF prediction report as an A4 PDF.
func HFPDF(in HF) ([]byte, error) {
	return buildHF(in).bytes()
}

// buildHF lays out the HF report: sites and conditions, the reliability grid of all months, then per month the MUF
// and reliability charts and a table per hour.
func buildHF(in HF) *doc {
	d := newDoc()
	foot := "Radiopath · ITU-R P.533 HF propagation prediction, monthly statistics"
	d.SetFooterFunc(func() { d.footer(foot) })
	a, b := in.SiteA, in.SiteB
	y := d.header(a.Name+" – "+b.Name, "HF circuit prediction report", in.User, in.Generated)

	sites := [][]string{
		{"Name", a.Name, b.Name},
		{"Position", fmt.Sprintf("%.4f, %.4f", a.Lat, a.Lon), fmt.Sprintf("%.4f, %.4f", b.Lat, b.Lon)},
	}
	x2 := margin + colW + gap
	y1 := d.table(margin, d.heading(margin, y, "Sites"), colW, []string{"", "A (TX)", "B (RX)"}, sites)
	y2 := d.table(x2, d.heading(x2, y, "Conditions"), colW, nil, in.Radio)
	y = math.Max(y1, y2) + 4

	hours, freqs := hfAxes(in.Months)
	y = d.heading(margin, y, "Circuit reliability")
	y = d.note(margin, y, contentW, "Share of the days of the month on which the SNR reaches the required one, per UTC hour; a dash: not predicted or not applicable.", muted) + 1
	for _, f := range freqs {
		y = d.hfGrid(d.need(y, float64(len(in.Months)+2)*4.2+2), in.Months, hours, f) + 2
	}

	// Every month on a page of its own; up to eight frequencies fit on it.
	for _, m := range in.Months {
		d.AddPage()
		d.hfMonth(margin, m)
	}
	return d
}

// need starts a new page when h mm no longer fit below y and returns the y to draw at.
func (d *doc) need(y, h float64) float64 {
	if y+h <= bottom-8 {
		return y
	}
	d.AddPage()
	return margin
}

// hfAxes returns the hours and frequencies of all months, sorted; hour 24 (00:00) sorts last, as on the HF page.
func hfAxes(months []HFMonth) ([]int, []float64) {
	var hours []int
	var freqs []float64
	for _, m := range months {
		for _, p := range m.Points {
			if !slices.Contains(hours, p.Hour) {
				hours = append(hours, p.Hour)
			}
			if !slices.Contains(freqs, p.FreqMHz) {
				freqs = append(freqs, p.FreqMHz)
			}
		}
	}
	slices.Sort(hours)
	slices.Sort(freqs)
	return hours, freqs
}

// hfAt returns the point of a month at an hour and frequency.
func hfAt(m HFMonth, hour int, f float64) (HFPoint, bool) {
	for _, p := range m.Points {
		if p.Hour == hour && p.FreqMHz == f {
			return p, true
		}
	}
	return HFPoint{}, false
}

// na reports whether a model value does not apply.
func na(v float64) bool { return v == hfNA || math.IsNaN(v) }

// monthName returns a month like "April 2026".
func monthName(m HFMonth) string { return fmt.Sprintf("%s %d", time.Month(m.Month), m.Year) }

// hourLabel returns a model hour as "06", hour 24 as "00".
func hourLabel(h int) string { return fmt.Sprintf("%02d", h%24) }

// heat returns the colour of a reliability like the HF page: hsl(1.2 × pct, 75 %, 45 %), red to green.
func heat(pct float64) color.RGBA {
	h, s, l := pct*1.2/60, 0.75, 0.45
	c := (1 - math.Abs(2*l-1)) * s
	x := c * (1 - math.Abs(math.Mod(h, 2)-1))
	var r, g, b float64
	switch {
	case h < 1:
		r, g = c, x
	case h < 2:
		r, g = x, c
	default:
		g, b = c, x
	}
	m := l - c/2
	return color.RGBA{uint8((r + m) * 255), uint8((g + m) * 255), uint8((b + m) * 255), 255}
}

// hfGrid draws the reliability of frequency f with a row per month and a column per hour, and returns the next y.
func (d *doc) hfGrid(y float64, months []HFMonth, hours []int, f float64) float64 {
	const labelW, cellH = 22.0, 4.2
	cellW := (contentW - labelW) / float64(len(hours))
	d.textColor(ink)
	d.text(margin, y+cellH-1.2, labelW, fmt.Sprintf("%g MHz", f), "Helvetica", "B", 7.5, "L")
	d.textColor(muted)
	for i, h := range hours {
		d.text(margin+labelW+float64(i)*cellW, y+cellH-1.2, cellW-0.5, hourLabel(h), "Courier", "", 6, "R")
	}
	y += cellH
	d.SetLineWidth(0.2)
	for _, m := range months {
		d.textColor(muted)
		d.text(margin, y+cellH-1.2, labelW-1, monthName(m), "Helvetica", "", 7, "L")
		for i, h := range hours {
			x := margin + labelW + float64(i)*cellW
			p, found := hfAt(m, h, f)
			if !found || na(p.ReliabilityPct) {
				d.textColor(muted)
				d.text(x, y+cellH-1.2, cellW-0.5, "–", "Courier", "", 6, "R")
				continue
			}
			d.fillColor(heat(p.ReliabilityPct))
			d.drawColor(color.RGBA{255, 255, 255, 255})
			d.Rect(x, y, cellW, cellH, "FD")
			d.textColor(color.RGBA{255, 255, 255, 255})
			d.text(x, y+cellH-1.2, cellW-0.5, fmt.Sprintf("%.0f", p.ReliabilityPct), "Courier", "B", 6, "R")
		}
		y += cellH
	}
	return y
}

// hfMonth draws the charts and the hour table of one month and returns the next y.
func (d *doc) hfMonth(y float64, m HFMonth) float64 {
	hours, freqs := hfAxes([]HFMonth{m})
	y = d.heading(margin, y, fmt.Sprintf("%s, sunspot number %d", monthName(m), m.SSN))

	// The MUF is a property of the path and hour, the same at every frequency.
	muf := func(get func(HFPoint) float64) []float64 {
		out := make([]float64, len(hours))
		for i, h := range hours {
			p, found := hfAt(m, h, freqs[0])
			out[i] = hfNA
			if found {
				out[i] = get(p)
			}
		}
		return out
	}
	lo, mid, hi := muf(func(p HFPoint) float64 { return p.MUF90 }), muf(func(p HFPoint) float64 { return p.MUF50 }), muf(func(p HFPoint) float64 { return p.MUF10 })
	top := freqs[len(freqs)-1]
	for _, v := range hi {
		if !na(v) {
			top = math.Max(top, v)
		}
	}
	top = math.Ceil(top*1.1/5) * 5

	const chartH = 45.0
	fx, fy := d.hfChart(margin, y, colW, chartH, "MUF (MHz)", hours, top)
	if n := len(hours); n > 1 && !slices.ContainsFunc(append(slices.Clone(lo), hi...), na) {
		band := make([]fpdf.PointType, 0, 2*n)
		for i := range n {
			band = append(band, fpdf.PointType{X: fx(i), Y: fy(lo[i])})
		}
		for i := n - 1; i >= 0; i-- {
			band = append(band, fpdf.PointType{X: fx(i), Y: fy(hi[i])})
		}
		d.fillColor(soft)
		d.Polygon(band, "F")
	}
	d.SetDashPattern([]float64{1.2, 1}, 0)
	for i, f := range freqs {
		d.drawColor(hfPalette[i%len(hfPalette)])
		d.SetLineWidth(0.3)
		d.Line(fx(0), fy(f), fx(len(hours)-1), fy(f))
	}
	d.SetDashPattern([]float64{}, 0)
	d.hfSeries(fx, fy, mid, ink)
	d.endChart()

	x2 := margin + colW + gap
	fx, fy = d.hfChart(x2, y, colW, chartH, "Circuit reliability (%)", hours, 100)
	for i, f := range freqs {
		vals := make([]float64, len(hours))
		for j, h := range hours {
			p, found := hfAt(m, h, f)
			vals[j] = hfNA
			if found {
				vals[j] = p.ReliabilityPct
			}
		}
		d.hfSeries(fx, fy, vals, hfPalette[i%len(hfPalette)])
	}
	d.endChart()
	y += chartH + 1
	d.note(margin, y, colW, "Line: median MUF. Band: exceeded on 90 % to 10 % of the days.", muted)
	y = d.hfLegend(x2, y, colW, freqs) + 2

	// A table per four frequencies, so the cells stay readable.
	for i := 0; i < len(freqs); i += 4 {
		chunk := freqs[i:min(i+4, len(freqs))]
		head := []string{"UTC", "MUF median"}
		for _, f := range chunk {
			head = append(head, fmt.Sprintf("%g MHz", f))
		}
		rows := make([][]string, 0, len(hours))
		for j, h := range hours {
			row := []string{hourLabel(h) + ":00", value(mid[j], "%.1f MHz")}
			for _, f := range chunk {
				p, found := hfAt(m, h, f)
				if !found {
					row = append(row, "–")
					continue
				}
				row = append(row, fmt.Sprintf("%s  %s  %s", value(p.ReliabilityPct, "%.0f %%"), value(p.SNRDB, "%+.0f dB"), p.Mode))
			}
			rows = append(rows, row)
		}
		y = d.hfTable(d.need(y, float64(len(rows)+1)*hourRowH+1), head, rows) + 2
	}
	return d.note(margin, y-1, contentW, "Per frequency: circuit reliability, median SNR in the bandwidth of the mode, dominant propagation mode.", muted)
}

// hourRowH is the row height of the hour tables, low enough for two of them below the charts of a month.
const hourRowH = 3.8

// hfTable draws the hour table: narrow hour and MUF columns, the rest shared by the frequencies; it returns the next y.
func (d *doc) hfTable(y float64, head []string, rows [][]string) float64 {
	widths := []float64{12, 20}
	rest := (contentW - 32) / float64(len(head)-2)
	for range head[2:] {
		widths = append(widths, rest)
	}
	draw := func(r []string, c color.RGBA, family string) {
		x := margin
		for i, v := range r {
			d.textColor(c)
			if i == 0 {
				d.text(x, y+hourRowH-1.1, widths[i], v, "Helvetica", "", labelPt, "L")
			} else {
				d.text(x, y+hourRowH-1.1, widths[i], v, family, "", valuePt, "R")
			}
			x += widths[i]
		}
		y += hourRowH
	}
	d.SetLineWidth(0.2)
	draw(head, muted, "Helvetica")
	d.drawColor(line)
	d.Line(margin, y, margin+contentW, y)
	for i, r := range rows {
		draw(r, ink, "Courier")
		if i < len(rows)-1 {
			d.drawColor(soft)
			d.Line(margin, y, margin+contentW, y)
		}
	}
	return y
}

// value formats a model value, or a dash when it does not apply.
func value(v float64, format string) string {
	if na(v) {
		return "–"
	}
	return fmt.Sprintf(format, v)
}

// hfChart draws the frame, title and axes of a chart with an x position per hour and 0 to ymax upwards, leaves a clip
// on the plot area that the caller ends with endChart, and returns the mappings from hour index and value to page mm.
func (d *doc) hfChart(x, y, w, h float64, title string, hours []int, ymax float64) (func(int) float64, func(float64) float64) {
	const left, bot, head = 9.0, 4.0, 4.5
	px, py, pw, ph := x+left, y+head, w-left-1, h-head-bot
	n := len(hours)
	fx := func(i int) float64 {
		if n < 2 {
			return px + pw/2
		}
		return px + float64(i)/float64(n-1)*pw
	}
	fy := func(v float64) float64 { return py + (1-v/ymax)*ph }

	d.textColor(ink)
	d.text(x, y+3, w, title, "Helvetica", "B", 7.5, "L")
	d.SetLineWidth(0.1)
	d.drawColor(soft)
	d.textColor(muted)
	for i := 0; i <= 4; i++ {
		v := ymax * float64(i) / 4
		d.Line(px, fy(v), px+pw, fy(v))
		d.text(x, fy(v)+0.8, left-1.5, fmt.Sprintf("%.0f", v), "Courier", "", 6, "R")
	}
	every := (n + 11) / 12 // at most twelve hour labels
	for i, hr := range hours {
		if i%every == 0 {
			d.text(fx(i)-2, py+ph+3, 4, hourLabel(hr), "Courier", "", 6, "L")
		}
	}
	d.drawColor(ink)
	d.SetLineWidth(0.2)
	d.Line(px, py, px, py+ph)
	d.Line(px, py+ph, px+pw, py+ph)
	d.ClipRect(px, py-0.5, pw, ph+0.5, false)
	return fx, fy
}

// endChart ends the clip of a chart. The clip restores the fill colour of before it, but fpdf still tracks the last
// one and writes text without a colour when it equals that; setting the fill colour again brings both in line.
func (d *doc) endChart() {
	d.ClipEnd()
	d.fillColor(color.RGBA{255, 255, 255, 255})
}

// hfSeries draws vals as a line in colour c, broken where a value does not apply; a lone value is a dot.
func (d *doc) hfSeries(fx func(int) float64, fy func(float64) float64, vals []float64, c color.RGBA) {
	d.drawColor(c)
	d.fillColor(c)
	d.SetLineWidth(0.45)
	for i, v := range vals {
		if na(v) {
			continue
		}
		prev := i > 0 && !na(vals[i-1])
		if prev {
			d.Line(fx(i-1), fy(vals[i-1]), fx(i), fy(v))
		}
		if !prev && (i == len(vals)-1 || na(vals[i+1])) {
			d.Circle(fx(i), fy(v), 0.5, "F")
		}
	}
}

// hfLegend draws a coloured line and label per frequency, wrapping within w, and returns the next y.
func (d *doc) hfLegend(x, y, w float64, freqs []float64) float64 {
	cx := x
	for i, f := range freqs {
		s := fmt.Sprintf("%g MHz", f)
		d.SetFont("Helvetica", "", 7)
		sw := 6 + d.GetStringWidth(s) + 3
		if cx+sw > x+w && cx > x {
			cx, y = x, y+4
		}
		d.drawColor(hfPalette[i%len(hfPalette)])
		d.SetLineWidth(0.8)
		d.Line(cx, y+2, cx+4, y+2)
		d.textColor(muted)
		d.text(cx+5, y+3, sw, s, "Helvetica", "", 7, "L")
		cx += sw
	}
	return y + 4.5
}
