package web

import (
	"fmt"
	"html/template"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/radiopath/radiopath/internal/geo"
	"github.com/radiopath/radiopath/internal/itm"
	"github.com/radiopath/radiopath/internal/link"
	"github.com/radiopath/radiopath/internal/plot"
	"github.com/radiopath/radiopath/internal/report"
	"github.com/radiopath/radiopath/internal/store"
	"github.com/radiopath/radiopath/internal/tiles"
)

type linkDetailData struct {
	base
	Link            store.LinkWithSites
	Result          *link.Result
	SVG             template.HTML
	Error           string
	TxAntenna       string
	RxAntenna       string
	TxAzimuthDeg    float64
	RxAzimuthDeg    float64
	Reliability     float64
	MeasuredDBm     float64
	MeasuredDeltaDB float64

	Path     string
	Shared   bool
	ShareURL string
}

type linkFormData struct {
	base
	Link     store.Link
	Sites    []store.Site
	Antennas []store.Antenna
	Form     map[string]string
	Errors   []string
}

func linkValues(l store.Link) map[string]string {
	f := func(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }
	return map[string]string{
		"frequency_mhz":      f(l.FreqMHz),
		"tx_power_dbm":       f(l.TxPowerDBm),
		"tx_gain_dbi":        f(l.TxGainDBi),
		"tx_line_loss_db":    f(l.TxLineLossDB),
		"rx_gain_dbi":        f(l.RxGainDBi),
		"rx_line_loss_db":    f(l.RxLineLossDB),
		"rx_sensitivity_dbm": f(l.RxSensitivityDBm),
		"tx_azimuth_deg":     optNumValue(l.TxAzimuthDeg),
		"rx_azimuth_deg":     optNumValue(l.RxAzimuthDeg),
		"tx_tilt_deg":        f(l.TxTiltDeg),
		"rx_tilt_deg":        f(l.RxTiltDeg),
		"clutter_loss_db":    f(l.ClutterLossDB),
		"measured_rx_dbm":    optNumValue(l.MeasuredRxDBm),
	}
}

func parseLink(r *http.Request) (store.Link, *form) {
	f := &form{r: r}
	l := store.Link{
		Name:             f.text("name", "Name"),
		SiteAID:          f.id("site_a_id", "Site A"),
		SiteBID:          f.id("site_b_id", "Site B"),
		FreqMHz:          f.num("frequency_mhz", "Frequency", 20, 20000),
		TxPowerDBm:       f.num("tx_power_dbm", "TX power", -100, 200),
		TxGainDBi:        f.optNum("tx_gain_dbi", "TX gain", -50, 100),
		TxLineLossDB:     f.optNum("tx_line_loss_db", "TX line loss", 0, 100),
		RxGainDBi:        f.optNum("rx_gain_dbi", "RX gain", -50, 100),
		RxLineLossDB:     f.optNum("rx_line_loss_db", "RX line loss", 0, 100),
		RxSensitivityDBm: f.num("rx_sensitivity_dbm", "RX sensitivity", -200, 50),
		Polarization:     int16(f.num("polarization", "Polarization", 0, 1)),
		TxAntennaID:      f.optID("tx_antenna_id", "TX antenna"),
		TxAzimuthDeg:     f.optDeg("tx_azimuth_deg", "TX azimuth"),
		TxTiltDeg:        f.optNum("tx_tilt_deg", "TX tilt", -90, 90),
		RxAntennaID:      f.optID("rx_antenna_id", "RX antenna"),
		RxAzimuthDeg:     f.optDeg("rx_azimuth_deg", "RX azimuth"),
		RxTiltDeg:        f.optNum("rx_tilt_deg", "RX tilt", -90, 90),
		ClutterLossDB:    f.optNum("clutter_loss_db", "Extra path loss", 0, 200),
		MeasuredRxDBm:    f.optNumP("measured_rx_dbm", "Measured RX level", -200, 50),
	}
	if l.SiteAID == l.SiteBID {
		f.Errors = append(f.Errors, "Site A and B must differ")
	}
	return l, f
}

func (s *Server) linkForm(w http.ResponseWriter, r *http.Request, status int, d linkFormData) {
	sites, err := s.Store.ListSites(r.Context(), currentUser(r).ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	antennas, err := s.Store.ListAntennas(r.Context(), currentUser(r).ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	d.Sites, d.Antennas = sites, antennas
	d.base = s.base(r)
	s.render(w, status, "link_form", d)
}

func (s *Server) listLinks(w http.ResponseWriter, r *http.Request) {
	links, err := s.Store.ListLinks(r.Context(), currentUser(r).ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	sites, err := s.Store.CountSites(r.Context(), currentUser(r).ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	s.render(w, http.StatusOK, "links_list", struct {
		base
		Links []store.LinkWithSites
		Sites int
	}{s.base(r), links, sites})
}

func (s *Server) newLink(w http.ResponseWriter, r *http.Request) {
	l := store.Link{FreqMHz: 145, TxPowerDBm: 40, RxSensitivityDBm: -120, Polarization: 1}
	if from, ok := fromID(r); ok {
		orig, err := s.Store.GetLink(r.Context(), currentUser(r).ID, from)
		if err != nil {
			s.fail(w, err)
			return
		}
		l = orig.Link
		l.ID, l.Name = 0, l.Name+" (copy)"
	}
	s.linkForm(w, r, http.StatusOK, linkFormData{Link: l, Form: linkValues(l)})
}

func (s *Server) createLink(w http.ResponseWriter, r *http.Request) {
	l, f := parseLink(r)
	if len(f.Errors) > 0 {
		s.linkForm(w, r, http.StatusUnprocessableEntity, linkFormData{Link: l, Form: f.values(), Errors: f.Errors})
		return
	}
	s.applyAntennaGains(r, &l)
	l.OwnerID = currentUser(r).ID
	id, err := s.Store.CreateLink(r.Context(), l)
	if err != nil {
		s.fail(w, err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/links/%d", id), http.StatusSeeOther)
}

func (s *Server) editLink(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	l, err := s.Store.GetLink(r.Context(), currentUser(r).ID, id)
	if err != nil {
		s.fail(w, err)
		return
	}
	s.linkForm(w, r, http.StatusOK, linkFormData{Link: l.Link, Form: linkValues(l.Link)})
}

func (s *Server) updateLink(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	l, f := parseLink(r)
	l.ID, l.OwnerID = id, currentUser(r).ID
	if len(f.Errors) > 0 {
		s.linkForm(w, r, http.StatusUnprocessableEntity, linkFormData{Link: l, Form: f.values(), Errors: f.Errors})
		return
	}
	s.applyAntennaGains(r, &l)
	if err := s.Store.UpdateLink(r.Context(), l); err != nil {
		s.fail(w, err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/links/%d", id), http.StatusSeeOther)
}

func (s *Server) deleteLink(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := s.Store.DeleteLink(r.Context(), currentUser(r).ID, id); err != nil {
		s.fail(w, err)
		return
	}
	http.Redirect(w, r, "/links", http.StatusSeeOther)
}

func (s *Server) showLink(w http.ResponseWriter, r *http.Request) {
	data, ok := s.linkView(w, r)
	if !ok {
		return
	}
	s.render(w, http.StatusOK, "link_detail", data)
}

func (s *Server) linkReport(w http.ResponseWriter, r *http.Request) {
	data, ok := s.linkView(w, r)
	if !ok {
		return
	}
	s.sendLinkReport(w, r, data)
}

func (s *Server) sendLinkReport(w http.ResponseWriter, r *http.Request, data linkDetailData) {
	a, b := data.Link.SiteA, data.Link.SiteB
	in := report.Link{
		Link: data.Link, Result: data.Result, Error: data.Error,
		TxAntenna: data.TxAntenna, RxAntenna: data.RxAntenna,
		TxAzimuthDeg: data.TxAzimuthDeg, RxAzimuthDeg: data.RxAzimuthDeg,
		ReliabilityPct: data.Reliability, User: data.User, Generated: time.Now(),
	}
	in.View = tiles.Fit(math.Min(a.Lat, b.Lat), math.Min(a.Lon, b.Lon), math.Max(a.Lat, b.Lat), math.Max(a.Lon, b.Lon), report.MapW, report.MapH, 80)
	in.Map = s.reportMap(r, in.View)
	pdf, err := report.LinkPDF(in)
	if err != nil {
		s.fail(w, err)
		return
	}
	servePDF(w, data.Link.Name, pdf)
}

func (s *Server) linkView(w http.ResponseWriter, r *http.Request) (linkDetailData, bool) {
	id, err := pathID(r)
	if err != nil {
		http.NotFound(w, r)
		return linkDetailData{}, false
	}
	l, err := s.Store.GetLink(r.Context(), currentUser(r).ID, id)
	if err != nil {
		s.fail(w, err)
		return linkDetailData{}, false
	}
	data := s.linkAnalysis(r, l)
	data.base = s.base(r)
	data.Path = fmt.Sprintf("/links/%d", l.ID)
	if l.ShareToken != "" {
		data.ShareURL = s.shareURL("/s/l/" + l.ShareToken)
	}
	return data, true
}

func (s *Server) linkAnalysis(r *http.Request, l store.LinkWithSites) linkDetailData {
	data := linkDetailData{Link: l, Reliability: link.ReliabilityPct}
	data.TxAntenna, data.RxAntenna = s.antennaName(r, l.OwnerID, l.TxAntennaID), s.antennaName(r, l.OwnerID, l.RxAntennaID)

	bearing := geo.InitialBearing(geo.Point{Lat: l.SiteA.Lat, Lon: l.SiteA.Lon}, geo.Point{Lat: l.SiteB.Lat, Lon: l.SiteB.Lon})
	data.TxAzimuthDeg = azimuth(l.TxAzimuthDeg, bearing)
	data.RxAzimuthDeg = azimuth(l.RxAzimuthDeg, math.Mod(bearing+180, 360))

	res, err := s.Analyzer.Analyze(r.Context(), link.Input{
		A:             link.Site{Name: l.SiteA.Name, Pos: geo.Point{Lat: l.SiteA.Lat, Lon: l.SiteA.Lon}, AntennaHeightM: l.SiteA.AntennaHeightM},
		B:             link.Site{Name: l.SiteB.Name, Pos: geo.Point{Lat: l.SiteB.Lat, Lon: l.SiteB.Lon}, AntennaHeightM: l.SiteB.AntennaHeightM},
		ClutterLossDB: l.ClutterLossDB,
		Radio: link.Radio{
			FreqMHz: l.FreqMHz, TxPowerDBm: l.TxPowerDBm, TxGainDBi: l.TxGainDBi, RxGainDBi: l.RxGainDBi,
			TxLineLossDB: l.TxLineLossDB, RxLineLossDB: l.RxLineLossDB, RxSensitivityDBm: l.RxSensitivityDBm,
			Polarization: itm.Polarization(l.Polarization),
			TxPattern:    s.pattern(r, l.OwnerID, l.TxAntennaID), TxAzimuthDeg: data.TxAzimuthDeg, TxTiltDeg: l.TxTiltDeg,
			RxPattern: s.pattern(r, l.OwnerID, l.RxAntennaID), RxAzimuthDeg: data.RxAzimuthDeg, RxTiltDeg: l.RxTiltDeg,
		},
	})
	if err != nil {
		s.Log.Warn("link analysis failed", "link", l.ID, "err", err)
		data.Error = err.Error()
	} else {
		data.Result = &res
		data.SVG = template.HTML(plot.SVG(res, l.SiteA.AntennaHeightM, l.SiteB.AntennaHeightM))
		if l.MeasuredRxDBm != nil {
			data.MeasuredDBm = *l.MeasuredRxDBm
			data.MeasuredDeltaDB = data.MeasuredDBm - res.RxLevelDBm
		}
	}
	return data
}

func (s *Server) applyAntennaGains(r *http.Request, l *store.Link) {
	l.TxGainDBi = s.antennaGain(r, l.TxAntennaID, l.TxGainDBi)
	l.RxGainDBi = s.antennaGain(r, l.RxAntennaID, l.RxGainDBi)
}
