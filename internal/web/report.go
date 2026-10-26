package web

import (
	"context"
	"math"
	"net/http"
	"strings"
	"unicode"

	"github.com/radiopath/radiopath/internal/report"
	"github.com/radiopath/radiopath/internal/store"
	"github.com/radiopath/radiopath/internal/tiles"
)

// reportMap renders the static map for a PDF report; on failure it logs and returns nil so the report goes out without a map.
func (s *Server) reportMap(ctx context.Context, v tiles.View) []byte {
	png, err := s.Tiles.Static(ctx, v)
	if err != nil {
		s.Log.Warn("report map", "err", err)
		return nil
	}
	return png
}

// rangeView returns the map view that frames the coverage's site and its full range circle.
func rangeView(c store.CoverageWithSite) tiles.View {
	dLat := c.RangeM / 111320
	dLon := dLat / math.Cos(c.Site.Lat*math.Pi/180)
	s, w, n, e := c.Site.Lat-dLat, c.Site.Lon-dLon, c.Site.Lat+dLat, c.Site.Lon+dLon
	return tiles.Fit(s, w, n, e, report.CoverageMapPx, report.CoverageMapPx, report.MapPad).Trim(s, w, n, e, report.MapPad)
}

// servePDF writes pdf inline as name.pdf; never cached.
func servePDF(w http.ResponseWriter, name string, pdf []byte) {
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `inline; filename="`+pdfName(name)+`.pdf"`)
	w.Header().Set("Cache-Control", "no-store")
	w.Write(pdf)
}

// pdfName replaces non-ASCII, control and quote or slash characters in a file name.
func pdfName(name string) string {
	return strings.Map(func(c rune) rune {
		if c > unicode.MaxASCII || c < ' ' || strings.ContainsRune(`"\/`, c) {
			return '_'
		}
		return c
	}, name)
}
