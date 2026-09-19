package report

import (
	"bytes"
	"fmt"
	"math"
	"time"

	"github.com/go-pdf/fpdf"

	"github.com/radiopath/radiopath/internal/plot"
	"github.com/radiopath/radiopath/internal/store"
	"github.com/radiopath/radiopath/internal/tiles"
)

type Coverage struct {
	Coverage  store.CoverageWithSite
	TxAntenna string
	User      string
	Generated time.Time
	Map       []byte
	View      tiles.View
	Raster    []byte
	Legend    plot.Legend
}

func CoveragePDF(in Coverage) ([]byte, error) {
	return buildCoverage(in).bytes()
}

func buildCoverage(in Coverage) *doc {
	d := newDoc()
	c := in.Coverage
	y := d.header(c.Name, "Area coverage report", in.User, in.Generated)
	if c.Note != "" {
		y = d.note(margin, y, contentW, c.Note, warn)
	}
	if c.Job.State == "failed" {
		y = d.note(margin, y, contentW, "Calculation failed: "+c.Job.Error, danger)
	}

	params := [][]string{
		{"Site (TX)", c.Site.Name},
		{"Position", fmt.Sprintf("%.5f, %.5f", c.Site.Lat, c.Site.Lon)},
		{"Antenna above ground", fmt.Sprintf("%.1f m", c.Site.AntennaHeightM)},
		{"Frequency / polarization", fmt.Sprintf("%.3f MHz, %s", c.FreqMHz, polarization(c.Polarization))},
		{"TX power / gain / line loss", fmt.Sprintf("%.1f dBm / %.1f dBi / %.1f dB", c.TxPowerDBm, c.TxGainDBi, c.TxLineLossDB)},
		{"RX height / gain / line loss", fmt.Sprintf("%.1f m / %.1f dBi / %.1f dB", c.RxHeightM, c.RxGainDBi, c.RxLineLossDB)},
		{"RX sensitivity", fmt.Sprintf("%.1f dBm", c.RxSensitivityDBm)},
		{"Range / resolution", fmt.Sprintf("%.1f km / %.0f m", c.RangeM/1000, c.ResolutionM)},
	}
	params = antenna(params, "TX", in.TxAntenna, c.TxAzimuthDeg, c.TxTiltDeg, "")
	x2 := margin + colW + gap
	y1 := d.table(margin, d.heading(margin, y, "Parameters"), colW, nil, params)
	y2 := d.heading(x2, y, "Result")
	if c.ComputedAt != nil && c.Bounds != nil {
		y2 = d.table(x2, y2, colW, nil, [][]string{
			{"Computed", c.ComputedAt.Format("2006-01-02 15:04:05") + " in " + c.ComputeTime()},
			{"Bounds N / S", fmt.Sprintf("%.5f / %.5f", c.Bounds.North, c.Bounds.South)},
			{"Bounds W / E", fmt.Sprintf("%.5f / %.5f", c.Bounds.West, c.Bounds.East)},
		})
		y2 = d.legend(x2, y2+3, in.Legend)
	} else {
		y2 = d.note(x2, y2, colW, "Not computed. Job state: "+string(c.Job.State)+".", muted)
	}
	y = math.Max(y1, y2) + 4

	y = d.heading(margin, y, "Coverage (link margin)")
	h := math.Min(bottom-8-y, contentW*float64(in.View.H)/float64(in.View.W))
	px := d.mapBox(margin, y, contentW, h, in.Map, in.View)
	if in.Raster != nil && c.Bounds != nil {
		x0, y0 := px(in.View.Pixel(c.Bounds.North, c.Bounds.West))
		x1, y1 := px(in.View.Pixel(c.Bounds.South, c.Bounds.East))
		d.RegisterImageOptionsReader("raster", fpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(in.Raster))
		d.SetAlpha(0.6, "Normal")
		d.ImageOptions("raster", x0, y0, x1-x0, y1-y0, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")
		d.SetAlpha(1, "Normal")
	}
	tx, ty := px(in.View.Pixel(c.Site.Lat, c.Site.Lon))
	d.ClipRect(margin, y, contentW, h, false)
	d.drawColor(accent)
	d.SetLineWidth(0.4)
	d.SetDashPattern([]float64{2, 1.5}, 0)
	d.Circle(tx, ty, c.RangeM/in.View.MetresPerPixel(c.Site.Lat)*contentW/float64(in.View.W), "D")
	d.SetDashPattern([]float64{}, 0)
	d.ClipEnd()
	d.marker(tx, ty, c.Site.Name)

	d.footer("Radiopath · ITM/Longley-Rice on Copernicus GLO-30 terrain")
	return d
}

func (d *doc) legend(x, y float64, l plot.Legend) float64 {
	if len(l) == 0 {
		l = plot.DefaultLegend()
	}
	d.textColor(ink)
	d.text(x, y+3, colW, "Legend", "Helvetica", "B", 8, "L")
	y += 5
	for i, b := range l {
		d.fillColor(b.Color)
		d.drawColor(line)
		d.SetLineWidth(0.2)
		d.Rect(x, y+0.7, 8, 3.2, "FD")
		d.textColor(muted)
		d.text(x+10, y+3.3, colW-10, l.Label(i), "Helvetica", "", labelPt, "L")
		y += rowH
	}
	d.Rect(x, y+0.7, 8, 3.2, "D")
	d.text(x+10, y+3.3, colW-10, l.Floor()+": transparent", "Helvetica", "", labelPt, "L")
	return y + rowH
}
