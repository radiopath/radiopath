package web

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/radiopath/radiopath/internal/plot"
	"github.com/radiopath/radiopath/internal/store"
)

type legendBin struct {
	Label string
	CSS   string
}

type legendRow struct {
	N   int
	Min string
	CSS string
}

type legendData struct {
	Bins       []legendBin
	Floor      string
	LegendRows []legendRow
}

const blankColor = "#9ca3af"

func legendViews(l plot.Legend) legendData {
	d := legendData{Floor: l.Floor(), LegendRows: make([]legendRow, plot.MaxBins)}
	for i, b := range l {
		d.Bins = append(d.Bins, legendBin{l.Label(i), b.Hex()})
	}
	for i := range d.LegendRows {
		d.LegendRows[i] = legendRow{N: i + 1, CSS: blankColor}
		if i < len(l) {
			d.LegendRows[i].Min = strconv.FormatFloat(l[i].MinDB, 'f', -1, 64)
			d.LegendRows[i].CSS = l[i].Hex()
		}
	}
	return d
}

func legendOf(c store.Coverage) plot.Legend { return plot.LegendOr(c.LegendDB, c.LegendColors) }

func encodeLegend(c store.Coverage) string {
	if len(c.LegendDB) == 0 {
		return ""
	}
	parts := make([]string, len(c.LegendDB))
	for i, m := range c.LegendDB {
		parts[i] = strconv.FormatFloat(m, 'f', -1, 64) + ":" + c.LegendColors[i]
	}
	return strings.Join(parts, ",")
}

func decodeLegend(v string) (mins []float64, colors []string, err error) {
	if strings.TrimSpace(v) == "" {
		return nil, nil, nil
	}
	for _, p := range strings.Split(v, ",") {
		m, c, ok := strings.Cut(p, ":")
		if !ok {
			return nil, nil, fmt.Errorf("legend: bad entry %q", p)
		}
		f, err := strconv.ParseFloat(strings.TrimSpace(m), 64)
		if err != nil {
			return nil, nil, fmt.Errorf("legend: bad threshold %q", m)
		}
		mins, colors = append(mins, f), append(colors, strings.TrimSpace(c))
	}
	l, err := plot.NewLegend(mins, colors)
	if err != nil {
		return nil, nil, err
	}
	return l.Mins(), l.Hex(), nil
}

func (s *Server) setCoverageLegend(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		s.notFound(w, r)
		return
	}
	back := fmt.Sprintf("/coverages/%d", id)
	var mins []float64
	var colors []string
	if r.FormValue("reset") == "" {
		f := &form{r: r}
		for i := 1; i <= plot.MaxBins; i++ {
			n := strconv.Itoa(i)
			if f.get("min_"+n) == "" {
				continue
			}
			mins = append(mins, f.num("min_"+n, "Threshold "+n, plot.MinLegendDB, plot.MaxLegendDB))
			colors = append(colors, f.get("color_"+n))
		}
		if len(f.Errors) > 0 {
			http.Redirect(w, r, back+"?error="+escapeQuery(strings.Join(f.Errors, "; ")), http.StatusSeeOther)
			return
		}
		l, err := plot.NewLegend(mins, colors)
		if err != nil {
			http.Redirect(w, r, back+"?error="+escapeQuery(err.Error()), http.StatusSeeOther)
			return
		}
		mins, colors = l.Mins(), l.Hex()
	}
	if err := s.Store.SetCoverageLegend(r.Context(), currentUser(r).ID, id, mins, colors); err != nil {
		s.fail(w, err)
		return
	}
	http.Redirect(w, r, back, http.StatusSeeOther)
}

func renderRaster(ra store.Raster) ([]byte, error) {
	if ra.Margin == nil {
		return ra.PNG, nil
	}
	res, err := plot.DecodeRaster(ra.Margin, ra.Bounds.North, ra.Bounds.South, ra.Bounds.East, ra.Bounds.West)
	if err != nil {
		return nil, err
	}
	return plot.CoveragePNG(res, plot.LegendOr(ra.LegendDB, ra.LegendColors)), nil
}
