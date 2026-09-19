package web

import (
	"math"
	"net/http"
	"strings"
	"unicode"

	"github.com/radiopath/radiopath/internal/report"
	"github.com/radiopath/radiopath/internal/store"
	"github.com/radiopath/radiopath/internal/tiles"
)

func (s *Server) reportMap(r *http.Request, v tiles.View) []byte {
	png, err := s.Tiles.Static(r.Context(), v)
	if err != nil {
		s.Log.Warn("report map", "err", err)
		return nil
	}
	return png
}

func rangeView(c store.CoverageWithSite) tiles.View {
	dLat := c.RangeM / 111320
	dLon := dLat / math.Cos(c.Site.Lat*math.Pi/180)
	s, w, n, e := c.Site.Lat-dLat, c.Site.Lon-dLon, c.Site.Lat+dLat, c.Site.Lon+dLon
	return tiles.Fit(s, w, n, e, report.CoverageMapPx, report.CoverageMapPx, report.MapPad).Trim(s, w, n, e, report.MapPad)
}

func servePDF(w http.ResponseWriter, name string, pdf []byte) {
	name = strings.Map(func(c rune) rune {
		if c > unicode.MaxASCII || c < ' ' || strings.ContainsRune(`"\/`, c) {
			return '_'
		}
		return c
	}, name)
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `inline; filename="`+name+`.pdf"`)
	w.Header().Set("Cache-Control", "no-store")
	w.Write(pdf)
}
