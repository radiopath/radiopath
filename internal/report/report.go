package report

import (
	"bytes"
	"fmt"
	"image/color"
	"time"

	"github.com/go-pdf/fpdf"

	"github.com/radiopath/radiopath/internal/tiles"
)

const (
	margin   = 12.0
	contentW = 210 - 2*margin
	bottom   = 297 - margin
	gap      = 6.0
	colW     = (contentW - gap) / 2
	rowH     = 4.6
	labelPt  = 8.0
	valuePt  = 8.0
)

const (
	MapW, MapH    = 1400, 600
	CoverageMapPx = 2400
	MapPad        = 60
)

var (
	ink     = color.RGBA{20, 23, 27, 255}
	muted   = color.RGBA{92, 100, 112, 255}
	line    = color.RGBA{184, 188, 194, 255}
	soft    = color.RGBA{221, 224, 228, 255}
	accent  = color.RGBA{26, 95, 180, 255}
	ok      = color.RGBA{46, 125, 50, 255}
	danger  = color.RGBA{164, 0, 0, 255}
	warn    = color.RGBA{138, 90, 0, 255}
	terrain = color.RGBA{200, 181, 138, 255}
	edge    = color.RGBA{107, 90, 52, 255}
	canopy  = color.RGBA{94, 156, 79, 255}
	fresnel = color.RGBA{42, 127, 212, 255}
	ray     = color.RGBA{212, 42, 42, 255}
)

type doc struct {
	*fpdf.Fpdf
	tr func(string) string
}

func newDoc() *doc {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(margin, margin, margin)
	pdf.SetAutoPageBreak(false, 0)
	pdf.AddPage()
	return &doc{pdf, pdf.UnicodeTranslatorFromDescriptor("")}
}

func (d *doc) bytes() ([]byte, error) {
	var buf bytes.Buffer
	if err := d.Output(&buf); err != nil {
		return nil, fmt.Errorf("report: %w", err)
	}
	return buf.Bytes(), nil
}

func (d *doc) textColor(c color.RGBA) { d.SetTextColor(int(c.R), int(c.G), int(c.B)) }
func (d *doc) drawColor(c color.RGBA) { d.SetDrawColor(int(c.R), int(c.G), int(c.B)) }
func (d *doc) fillColor(c color.RGBA) { d.SetFillColor(int(c.R), int(c.G), int(c.B)) }

func (d *doc) text(x, y, w float64, s, family, style string, pt float64, align string) {
	s = d.tr(s)
	d.SetFont(family, style, pt)
	for pt > 5.5 && d.GetStringWidth(s) > w {
		pt -= 0.5
		d.SetFont(family, style, pt)
	}
	if align == "R" {
		x += w - d.GetStringWidth(s)
	}
	d.Text(x, y, s)
}

func (d *doc) header(title, kind, user string, at time.Time) float64 {
	d.textColor(ink)
	d.text(margin, margin+6, contentW-60, title, "Helvetica", "B", 16, "L")
	d.textColor(muted)
	d.text(margin, margin+11, contentW-60, kind, "Helvetica", "", 9, "L")
	d.text(margin+contentW-60, margin+6, 60, at.Format("2006-01-02 15:04"), "Helvetica", "", 9, "R")
	if user != "" {
		d.text(margin+contentW-60, margin+11, 60, "by "+user, "Helvetica", "", 9, "R")
	}
	d.drawColor(line)
	d.SetLineWidth(0.3)
	d.Line(margin, margin+14, margin+contentW, margin+14)
	return margin + 18
}

func (d *doc) heading(x, y float64, s string) float64 {
	d.textColor(ink)
	d.text(x, y+3, colW, s, "Helvetica", "B", 9.5, "L")
	return y + 6
}

func (d *doc) table(x, y, w float64, head []string, rows [][]string) float64 {
	ncol := 1
	if len(head) > 0 {
		ncol = len(head) - 1
	} else if len(rows) > 0 {
		ncol = len(rows[0]) - 1
	}
	labelW := w * 0.45
	if ncol > 1 {
		labelW = w * 0.28
	}
	valW := (w - labelW) / float64(ncol)
	d.SetLineWidth(0.2)
	if len(head) > 0 {
		d.textColor(muted)
		for i, h := range head[1:] {
			d.text(x+labelW+float64(i)*valW, y+rowH-1.3, valW, h, "Helvetica", "", labelPt, "R")
		}
		d.drawColor(line)
		d.Line(x, y+rowH, x+w, y+rowH)
		y += rowH
	}
	for i, r := range rows {
		d.textColor(muted)
		d.text(x, y+rowH-1.3, labelW-2, r[0], "Helvetica", "", labelPt, "L")
		d.textColor(ink)
		for j, v := range r[1:] {
			d.text(x+labelW+float64(j)*valW, y+rowH-1.3, valW, v, "Courier", "", valuePt, "R")
		}
		if i < len(rows)-1 {
			d.drawColor(soft)
			d.Line(x, y+rowH, x+w, y+rowH)
		}
		y += rowH
	}
	return y
}

func (d *doc) note(x, y, w float64, s string, c color.RGBA) float64 {
	d.textColor(c)
	d.text(x, y+3, w, s, "Helvetica", "", 8, "L")
	return y + 4.5
}

func (d *doc) mapBox(x, y, w, h float64, png []byte, v tiles.View) func(px, py float64) (float64, float64) {
	d.drawColor(line)
	d.SetLineWidth(0.2)
	if png == nil {
		d.Rect(x, y, w, h, "D")
		d.note(x+2, y+2, w-4, "Map unavailable", muted)
	} else {
		d.RegisterImageOptionsReader("map", fpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(png))
		d.ImageOptions("map", x, y, w, h, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")
		d.Rect(x, y, w, h, "D")
		attr := "© OpenStreetMap contributors"
		d.SetFont("Helvetica", "", 6.5)
		aw := d.GetStringWidth(attr) + 2
		d.fillColor(color.RGBA{255, 255, 255, 255})
		d.Rect(x+w-aw, y+h-3.5, aw, 3.5, "F")
		d.textColor(muted)
		d.text(x+w-aw+1, y+h-1, aw, attr, "Helvetica", "", 6.5, "L")
	}
	return func(px, py float64) (float64, float64) {
		return x + px*w/float64(v.W), y + py*h/float64(v.H)
	}
}

func (d *doc) marker(x, y float64, label string) {
	d.fillColor(accent)
	d.drawColor(color.RGBA{255, 255, 255, 255})
	d.SetLineWidth(0.5)
	d.Circle(x, y, 1.6, "FD")
	d.SetFont("Helvetica", "B", 7.5)
	lw := d.GetStringWidth(d.tr(label)) + 2
	d.fillColor(color.RGBA{255, 255, 255, 255})
	d.drawColor(line)
	d.SetLineWidth(0.2)
	d.Rect(x+2.5, y-2, lw, 4, "FD")
	d.textColor(ink)
	d.text(x+3.5, y+1, lw, label, "Helvetica", "B", 7.5, "L")
}

func (d *doc) footer(s string) {
	d.drawColor(soft)
	d.SetLineWidth(0.2)
	d.Line(margin, bottom-4, margin+contentW, bottom-4)
	d.textColor(muted)
	d.SetFont("Helvetica", "", 7)
	d.text(margin+(contentW-d.GetStringWidth(d.tr(s)))/2, bottom-1, contentW, s, "Helvetica", "", 7, "L")
}

func polarization(p int16) string {
	if p == 1 {
		return "vertical"
	}
	return "horizontal"
}

func antenna(rows [][]string, end, name string, azimuthDeg, tiltDeg float64, towards string) [][]string {
	if name == "" {
		return append(rows, []string{end + " antenna", "omnidirectional"})
	}
	rows = append(rows, []string{end + " antenna", name}, []string{end + " azimuth", fmt.Sprintf("%.0f deg%s", azimuthDeg, towards)})
	if tiltDeg != 0 {
		rows = append(rows, []string{end + " tilt", fmt.Sprintf("%.1f deg", tiltDeg)})
	}
	return rows
}
