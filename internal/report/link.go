package report

import (
	"fmt"
	"image/color"
	"math"
	"time"

	"github.com/go-pdf/fpdf"

	"github.com/radiopath/radiopath/internal/link"
	"github.com/radiopath/radiopath/internal/store"
	"github.com/radiopath/radiopath/internal/tiles"
)

type Link struct {
	Link                       store.LinkWithSites
	Result                     *link.Result
	Error                      string
	TxAntenna, RxAntenna       string
	TxAzimuthDeg, RxAzimuthDeg float64
	ReliabilityPct             float64
	User                       string
	Generated                  time.Time
	Map                        []byte
	View                       tiles.View
}

func LinkPDF(in Link) ([]byte, error) {
	return buildLink(in).bytes()
}

func buildLink(in Link) *doc {
	d := newDoc()
	l, r := in.Link, in.Result
	y := d.header(l.Name, "Point-to-point link report", in.User, in.Generated)

	sites := [][]string{
		{"Name", l.SiteA.Name, l.SiteB.Name},
		{"Position", fmt.Sprintf("%.5f, %.5f", l.SiteA.Lat, l.SiteA.Lon), fmt.Sprintf("%.5f, %.5f", l.SiteB.Lat, l.SiteB.Lon)},
		{"Antenna above ground", fmt.Sprintf("%.1f m", l.SiteA.AntennaHeightM), fmt.Sprintf("%.1f m", l.SiteB.AntennaHeightM)},
	}
	if r != nil {
		sites = append(sites, []string{"Ground elevation", fmt.Sprintf("%.0f m", r.TerrainAM), fmt.Sprintf("%.0f m", r.TerrainBM)})
	}
	radio := [][]string{
		{"Frequency", fmt.Sprintf("%.3f MHz", l.FreqMHz)},
		{"Polarization", polarization(l.Polarization)},
		{"TX power", fmt.Sprintf("%.1f dBm", l.TxPowerDBm)},
		{"TX gain / line loss", fmt.Sprintf("%.1f dBi / %.1f dB", l.TxGainDBi, l.TxLineLossDB)},
		{"RX gain / line loss", fmt.Sprintf("%.1f dBi / %.1f dB", l.RxGainDBi, l.RxLineLossDB)},
		{"RX sensitivity", fmt.Sprintf("%.1f dBm", l.RxSensitivityDBm)},
	}
	radio = antenna(radio, "TX", in.TxAntenna, in.TxAzimuthDeg, l.TxTiltDeg, towards(l.TxAzimuthDeg, "B"))
	radio = antenna(radio, "RX", in.RxAntenna, in.RxAzimuthDeg, l.RxTiltDeg, towards(l.RxAzimuthDeg, "A"))
	x2 := margin + colW + gap
	y1 := d.table(margin, d.heading(margin, y, "Sites"), colW, []string{"", "A (TX)", "B (RX)"}, sites)
	y2 := d.table(x2, d.heading(x2, y, "Radio"), colW, nil, radio)
	y = math.Max(y1, y2) + 4

	y = d.heading(margin, y, "Result")
	if r == nil {
		y = d.note(margin, y, contentW, "Calculation failed: "+in.Error, danger) + 2
	} else {
		y = d.marginPanel(y, r, in.ReliabilityPct)
		rows := [][]string{
			{"Distance", fmt.Sprintf("%.2f km", r.DistanceM/1000)},
			{"Bearing A to B", fmt.Sprintf("%.1f deg", r.BearingDeg)},
			{"Free space loss", fmt.Sprintf("%.1f dB", r.FreeSpaceLossDB)},
			{"Path loss (ITM), median", fmt.Sprintf("%.1f dB", r.PathLossDB)},
			{fmt.Sprintf("Path loss, %.0f %% of the time", in.ReliabilityPct), fmt.Sprintf("%.1f dB", r.PathLossRelDB)},
			{"ITM propagation mode", fmt.Sprint(r.Mode)},
		}
		if in.TxAntenna != "" {
			rows = append(rows, []string{"TX pattern loss", fmt.Sprintf("%.1f dB (take-off %+.1f deg, %s)", r.TxPatternLossDB, r.TxTakeoffDeg, link.OffBoresight(r.TxOffBoresightDeg))})
		}
		if in.RxAntenna != "" {
			rows = append(rows, []string{"RX pattern loss", fmt.Sprintf("%.1f dB (take-off %+.1f deg, %s)", r.RxPatternLossDB, r.RxTakeoffDeg, link.OffBoresight(r.RxOffBoresightDeg))})
		}
		if r.HasCanopy {
			rows = append(rows, []string{"Vegetation loss (ITU-R P.833)", fmt.Sprintf("%.1f dB (%.0f m at A, %.0f m at B)", r.VegetationLossDB, r.CanopyPathAM, r.CanopyPathBM)})
		}
		if r.ClutterLossDB != 0 {
			rows = append(rows, []string{"Extra path loss", fmt.Sprintf("%.1f dB", r.ClutterLossDB)})
		}
		rows = append(rows,
			[]string{"EIRP", fmt.Sprintf("%.1f dBm", r.EIRPdBm)},
			[]string{"Received level", fmt.Sprintf("%.1f dBm", r.RxLevelDBm)})
		if l.MeasuredRxDBm != nil {
			rows = append(rows, []string{"Measured level", fmt.Sprintf("%.1f dBm (%+.1f dB)", *l.MeasuredRxDBm, *l.MeasuredRxDBm-r.RxLevelDBm)})
		}
		rows = append(rows, []string{"Worst first Fresnel zone clearance", fmt.Sprintf("%.0f %% at %.2f km", r.WorstClearancePct, r.WorstClearanceDistM/1000)})
		half := (len(rows) + 1) / 2
		y1 = d.table(margin, y, colW, nil, rows[:half])
		y2 = d.table(x2, y, colW, nil, rows[half:])
		y = math.Max(y1, y2) + 1
		for _, w := range r.Warnings {
			y = d.note(margin, y, contentW, w, warn)
		}
		y += 3

		y = d.heading(margin, y, "Terrain profile")
		d.profile(margin, y, contentW, 52, r, l.SiteA.AntennaHeightM, l.SiteB.AntennaHeightM)
		y += 52
		y = d.note(margin, y, contentW, "Terrain with 4/3 earth curvature. Green: tree canopy. Red: direct ray. Blue dashed: first Fresnel zone.", muted) + 2
	}

	y = d.heading(margin, y, "Path")
	h := math.Min(bottom-8-y, contentW*MapH/MapW)
	px := d.mapBox(margin, y, contentW, h, in.Map, in.View)
	ax, ay := px(in.View.Pixel(l.SiteA.Lat, l.SiteA.Lon))
	bx, by := px(in.View.Pixel(l.SiteB.Lat, l.SiteB.Lon))
	d.drawColor(ray)
	d.SetLineWidth(0.7)
	d.Line(ax, ay, bx, by)
	d.marker(ax, ay, l.SiteA.Name)
	d.marker(bx, by, l.SiteB.Name)

	d.footer("Radiopath · ITM/Longley-Rice on Copernicus GLO-30 terrain, ITU-R P.833 vegetation")
	return d
}

func towards(az *float64, other string) string {
	if az == nil {
		return " (towards " + other + ")"
	}
	return ""
}

func (d *doc) marginPanel(y float64, r *link.Result, rel float64) float64 {
	const h = 12.0
	d.drawColor(line)
	d.SetLineWidth(0.2)
	d.Rect(margin, y, contentW, h, "D")
	d.textColor(muted)
	d.text(margin+3, y+5, 60, "Link margin, median", "Helvetica", "", 8, "L")
	d.text(margin+3, y+10, 60, fmt.Sprintf("%.0f %% of the time", rel), "Helvetica", "", 8, "L")
	d.textColor(signColor(r.MarginDB))
	d.text(margin+contentW-53, y+6.5, 50, fmt.Sprintf("%+.1f dB", r.MarginDB), "Courier", "B", 14, "R")
	d.textColor(signColor(r.MarginRelDB))
	d.text(margin+contentW-53, y+10.5, 50, fmt.Sprintf("%+.1f dB", r.MarginRelDB), "Courier", "", 9, "R")
	return y + h + 3
}

func signColor(v float64) color.RGBA {
	if v < 0 {
		return danger
	}
	return ok
}

func (d *doc) profile(x, y, w, h float64, r *link.Result, antAM, antBM float64) {
	p := r.Profile
	if len(p) < 2 {
		d.note(x, y, w, "no profile", muted)
		return
	}
	n := len(p)
	dist := p[n-1].DistM
	ymin, ymax := math.Inf(1), math.Inf(-1)
	hasCanopy := false
	for _, s := range p {
		ymin = math.Min(ymin, math.Min(s.TerrainM+s.BulgeM, s.RayM-s.FresnelM))
		ymax = math.Max(ymax, math.Max(s.TerrainM+s.BulgeM+s.CanopyM, s.RayM+s.FresnelM))
		hasCanopy = hasCanopy || s.CanopyM > 0
	}
	ymax = math.Max(ymax, math.Max(p[0].TerrainM+antAM, p[n-1].TerrainM+antBM))
	pad := math.Max(10, (ymax-ymin)*0.05)
	ymin, ymax = ymin-pad, ymax+pad

	const left, bot = 14.0, 5.0
	px, py := x+left, y+1
	pw, ph := w-left-2, h-bot-1
	fx := func(dm float64) float64 { return px + dm/dist*pw }
	fy := func(hm float64) float64 { return py + (ymax-hm)/(ymax-ymin)*ph }
	pts := func(f func(s link.Sample) float64) []fpdf.PointType {
		out := make([]fpdf.PointType, n)
		for i, s := range p {
			out[i] = fpdf.PointType{X: fx(s.DistM), Y: fy(f(s))}
		}
		return out
	}

	d.ClipRect(px, py, pw, ph, false)
	d.SetLineWidth(0.2)
	ground := pts(func(s link.Sample) float64 { return s.TerrainM + s.BulgeM })
	d.fillColor(terrain)
	d.drawColor(edge)
	d.Polygon(append(append([]fpdf.PointType{{X: fx(0), Y: fy(ymin)}}, ground...), fpdf.PointType{X: fx(dist), Y: fy(ymin)}), "FD")
	if hasCanopy {
		top := pts(func(s link.Sample) float64 { return s.TerrainM + s.BulgeM + s.CanopyM })
		for i := n - 1; i >= 0; i-- {
			top = append(top, ground[i])
		}
		d.fillColor(canopy)
		d.Polygon(top, "F")
	}
	d.drawColor(fresnel)
	d.SetDashPattern([]float64{1.2, 1}, 0)
	for _, sign := range []float64{1, -1} {
		d.polyline(pts(func(s link.Sample) float64 { return s.RayM + sign*s.FresnelM }))
	}
	d.SetDashPattern([]float64{}, 0)
	d.drawColor(ray)
	d.SetLineWidth(0.4)
	d.Line(fx(0), fy(p[0].RayM), fx(dist), fy(p[n-1].RayM))
	d.drawColor(ink)
	d.SetLineWidth(0.5)
	d.Line(fx(0), fy(p[0].TerrainM), fx(0), fy(p[0].TerrainM+antAM))
	d.Line(fx(dist), fy(p[n-1].TerrainM), fx(dist), fy(p[n-1].TerrainM+antBM))
	d.ClipEnd()

	d.drawColor(ink)
	d.SetLineWidth(0.2)
	d.Line(px, py, px, py+ph)
	d.Line(px, py+ph, px+pw, py+ph)
	d.textColor(muted)
	for i := 0; i <= 5; i++ {
		dm := dist * float64(i) / 5
		s := fmt.Sprintf("%.1f km", dm/1000)
		d.SetFont("Courier", "", 6.5)
		d.text(fx(dm)-d.GetStringWidth(s)/2, py+ph+3.5, 20, s, "Courier", "", 6.5, "L")
		hm := ymin + (ymax-ymin)*float64(i)/5
		d.text(x, fy(hm)+0.8, left-1.5, fmt.Sprintf("%.0f m", hm), "Courier", "", 6.5, "R")
	}
}

func (d *doc) polyline(pts []fpdf.PointType) {
	for i := 1; i < len(pts); i++ {
		d.Line(pts[i-1].X, pts[i-1].Y, pts[i].X, pts[i].Y)
	}
}
