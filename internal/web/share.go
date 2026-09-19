package web

import (
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
)

const shareTTL = 30 * 24 * time.Hour

const (
	shareWindow = time.Hour
	shareMax    = 120
)

func (s *Server) shareURL(path string) string {
	return s.BaseURL + path
}

func (s *Server) shareLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) < 4 || parts[3] == "" {
			s.notFound(w, r)
			return
		}
		n, err := s.Store.RateHit(r.Context(), "share:"+parts[3], shareWindow)
		if err != nil {
			s.fail(w, err)
			return
		}
		if n > shareMax {
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
		w.Header().Set("X-Robots-Tag", "noindex")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) shareLink(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		s.notFound(w, r)
		return
	}
	if err := s.Store.ShareLink(r.Context(), currentUser(r).ID, id, randomToken(), time.Now().Add(shareTTL)); err != nil {
		s.fail(w, err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/links/%d", id), http.StatusSeeOther)
}

func (s *Server) unshareLink(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		s.notFound(w, r)
		return
	}
	if err := s.Store.UnshareLink(r.Context(), currentUser(r).ID, id); err != nil {
		s.fail(w, err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/links/%d", id), http.StatusSeeOther)
}

func (s *Server) shareCoverage(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		s.notFound(w, r)
		return
	}
	if err := s.Store.ShareCoverage(r.Context(), currentUser(r).ID, id, randomToken(), time.Now().Add(shareTTL)); err != nil {
		s.fail(w, err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/coverages/%d", id), http.StatusSeeOther)
}

func (s *Server) unshareCoverage(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		s.notFound(w, r)
		return
	}
	if err := s.Store.UnshareCoverage(r.Context(), currentUser(r).ID, id); err != nil {
		s.fail(w, err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/coverages/%d", id), http.StatusSeeOther)
}

func (s *Server) sharedLink(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	l, err := s.Store.GetSharedLink(r.Context(), token)
	if err != nil {
		s.fail(w, err)
		return
	}
	data := s.linkAnalysis(r, l)
	data.Shared, data.Path = true, "/s/l/"+token
	s.render(w, http.StatusOK, "link_detail", data)
}

func (s *Server) sharedLinkReport(w http.ResponseWriter, r *http.Request) {
	l, err := s.Store.GetSharedLink(r.Context(), r.PathValue("token"))
	if err != nil {
		s.fail(w, err)
		return
	}
	s.sendLinkReport(w, r, s.linkAnalysis(r, l))
}

func (s *Server) sharedCoverage(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	c, err := s.Store.GetSharedCoverage(r.Context(), token)
	if err != nil {
		s.fail(w, err)
		return
	}
	extra, err := s.Store.CoveragesByID(r.Context(), c.OwnerID, c.ViewOverlays)
	if err != nil {
		s.fail(w, err)
		return
	}
	data := s.coverageDetail(r, c, "/s/c/"+token)
	data.Shared = true
	for _, o := range extra {
		data.Others = append(data.Others, overlay{CoverageWithSite: o, Checked: true,
			URL: fmt.Sprintf("/s/c/%s/image.png?id=%d&t=%d", token, o.ID, o.ComputedAt.Unix())})
	}
	s.render(w, http.StatusOK, "coverage_detail", data)
}

func (s *Server) sharedCoverageReport(w http.ResponseWriter, r *http.Request) {
	c, err := s.Store.GetSharedCoverage(r.Context(), r.PathValue("token"))
	if err != nil {
		s.fail(w, err)
		return
	}
	s.sendCoverageReport(w, r, c, "")
}

func (s *Server) sharedCoverageComposite(w http.ResponseWriter, r *http.Request) {
	c, err := s.Store.GetSharedCoverage(r.Context(), r.PathValue("token"))
	if err != nil {
		s.fail(w, err)
		return
	}
	with := withIDs(r.URL.Query().Get("with"))
	for _, n := range with {
		if !slices.Contains(c.ViewOverlays, n) {
			s.notFound(w, r)
			return
		}
	}
	s.composite(w, r, c.OwnerID, with, c.ID, legendOf(c.Coverage))
}

func (s *Server) sharedCoverageImage(w http.ResponseWriter, r *http.Request) {
	c, err := s.Store.GetSharedCoverage(r.Context(), r.PathValue("token"))
	if err != nil {
		s.fail(w, err)
		return
	}
	id := c.ID
	if q := r.URL.Query().Get("id"); q != "" {
		n, err := strconv.ParseInt(q, 10, 64)
		if err != nil || !slices.Contains(c.ViewOverlays, n) {
			s.notFound(w, r)
			return
		}
		id = n
	}
	ra, err := s.Store.CoverageRaster(r.Context(), c.OwnerID, id)
	if err != nil {
		s.fail(w, err)
		return
	}
	png, err := renderRaster(ra)
	if err != nil {
		s.fail(w, err)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Write(png)
}
