package web

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/radiopath/radiopath/internal/store"
)

type siteFormData struct {
	base
	Site   store.Site
	Form   map[string]string
	Errors []string
}

func siteValues(st store.Site) map[string]string {
	return map[string]string{
		"lat":              strconv.FormatFloat(st.Lat, 'f', -1, 64),
		"lon":              strconv.FormatFloat(st.Lon, 'f', -1, 64),
		"antenna_height_m": strconv.FormatFloat(st.AntennaHeightM, 'f', -1, 64),
	}
}

func parseSite(r *http.Request) (store.Site, *form) {
	f := &form{r: r}
	st := store.Site{
		Name:           f.text("name", "Name"),
		Lat:            f.num("lat", "Latitude", -90, 90),
		Lon:            f.num("lon", "Longitude", -180, 180),
		AntennaHeightM: f.num("antenna_height_m", "Antenna height", 0.5, 3000),
	}
	return st, f
}

func (s *Server) listSites(w http.ResponseWriter, r *http.Request) {
	sites, err := s.Store.ListSites(r.Context(), currentUser(r).ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	s.render(w, http.StatusOK, "sites_list", struct {
		base
		Sites []store.Site
		Error string
	}{s.base(r), sites, r.URL.Query().Get("error")})
}

func (s *Server) newSite(w http.ResponseWriter, r *http.Request) {
	st := store.Site{AntennaHeightM: 10}
	f := siteValues(st)
	f["lat"], f["lon"] = "", ""
	if from, ok := fromID(r); ok {
		orig, err := s.Store.GetSite(r.Context(), currentUser(r).ID, from)
		if err != nil {
			s.fail(w, err)
			return
		}
		st = store.Site{Name: orig.Name + " (copy)", Lat: orig.Lat, Lon: orig.Lon, AntennaHeightM: orig.AntennaHeightM}
		f = siteValues(st)
	}
	s.render(w, http.StatusOK, "site_form", siteFormData{base: s.base(r), Site: st, Form: f})
}

func (s *Server) createSite(w http.ResponseWriter, r *http.Request) {
	st, f := parseSite(r)
	if len(f.Errors) > 0 {
		s.render(w, http.StatusUnprocessableEntity, "site_form", siteFormData{base: s.base(r), Site: st, Form: f.values(), Errors: f.Errors})
		return
	}
	st.OwnerID = currentUser(r).ID
	_, err := s.Store.CreateSite(r.Context(), st)
	if errors.Is(err, store.ErrQuota) {
		s.render(w, http.StatusUnprocessableEntity, "site_form", siteFormData{base: s.base(r), Site: st, Form: f.values(),
			Errors: []string{fmt.Sprintf("Limit of %d sites reached", store.MaxSitesPerUser)}})
		return
	}
	if err != nil {
		s.fail(w, err)
		return
	}
	http.Redirect(w, r, "/sites", http.StatusSeeOther)
}

func (s *Server) editSite(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	st, err := s.Store.GetSite(r.Context(), currentUser(r).ID, id)
	if err != nil {
		s.fail(w, err)
		return
	}
	s.render(w, http.StatusOK, "site_form", siteFormData{base: s.base(r), Site: st, Form: siteValues(st)})
}

func (s *Server) updateSite(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	st, f := parseSite(r)
	st.ID, st.OwnerID = id, currentUser(r).ID
	if len(f.Errors) > 0 {
		s.render(w, http.StatusUnprocessableEntity, "site_form", siteFormData{base: s.base(r), Site: st, Form: f.values(), Errors: f.Errors})
		return
	}
	if err := s.Store.UpdateSite(r.Context(), st); err != nil {
		s.fail(w, err)
		return
	}
	http.Redirect(w, r, "/sites", http.StatusSeeOther)
}

func (s *Server) deleteSite(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	err = s.Store.DeleteSite(r.Context(), currentUser(r).ID, id)
	if errors.Is(err, store.ErrInUse) {
		http.Redirect(w, r, "/sites?error="+escapeQuery("Site is used by a link or coverage and cannot be deleted."), http.StatusSeeOther)
		return
	}
	if err != nil {
		s.fail(w, err)
		return
	}
	http.Redirect(w, r, "/sites", http.StatusSeeOther)
}
