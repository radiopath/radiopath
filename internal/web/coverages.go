package web

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/radiopath/radiopath/internal/coverage"
	"github.com/radiopath/radiopath/internal/plot"
	"github.com/radiopath/radiopath/internal/report"
	"github.com/radiopath/radiopath/internal/store"
)

type coverageDetailData struct {
	base
	legendData
	Coverage  store.CoverageWithSite
	Others    []overlay
	TxAntenna string
	Error     string

	Path     string
	Shared   bool
	ShareURL string
	ViewURL  string
}

type overlay struct {
	store.CoverageWithSite
	URL     string
	Checked bool
}

type coverageFormData struct {
	base
	Coverage store.Coverage
	Sites    []store.Site
	Antennas []store.Antenna
	Form     map[string]string
	Errors   []string
}

func coverageValues(c store.Coverage) map[string]string {
	f := func(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }
	return map[string]string{
		"frequency_mhz":      f(c.FreqMHz),
		"tx_power_dbm":       f(c.TxPowerDBm),
		"tx_gain_dbi":        f(c.TxGainDBi),
		"tx_line_loss_db":    f(c.TxLineLossDB),
		"rx_height_m":        f(c.RxHeightM),
		"rx_gain_dbi":        f(c.RxGainDBi),
		"rx_line_loss_db":    f(c.RxLineLossDB),
		"rx_sensitivity_dbm": f(c.RxSensitivityDBm),
		"range_km":           f(c.RangeM / 1000),
		"resolution_m":       f(c.ResolutionM),
		"tx_azimuth_deg":     f(c.TxAzimuthDeg),
		"tx_tilt_deg":        f(c.TxTiltDeg),
		"legend":             encodeLegend(c),
	}
}

func parseCoverage(r *http.Request) (store.Coverage, *form) {
	f := &form{r: r}
	c := store.Coverage{
		Name:             f.text("name", "Name"),
		SiteID:           f.id("site_id", "Site"),
		FreqMHz:          f.num("frequency_mhz", "Frequency", 20, 20000),
		TxPowerDBm:       f.num("tx_power_dbm", "TX power", -100, 200),
		TxGainDBi:        f.optNum("tx_gain_dbi", "TX gain", -50, 100),
		TxLineLossDB:     f.optNum("tx_line_loss_db", "TX line loss", 0, 100),
		RxHeightM:        f.num("rx_height_m", "RX height", 0.5, 3000),
		RxGainDBi:        f.optNum("rx_gain_dbi", "RX gain", -50, 100),
		RxLineLossDB:     f.optNum("rx_line_loss_db", "RX line loss", 0, 100),
		RxSensitivityDBm: f.num("rx_sensitivity_dbm", "RX sensitivity", -200, 50),
		Polarization:     int16(f.num("polarization", "Polarization", 0, 1)),
		RangeM:           f.num("range_km", "Range", coverage.MinRangeM/1000, coverage.MaxRangeM/1000) * 1000,
		ResolutionM:      f.num("resolution_m", "Resolution", coverage.MinResolutionM, coverage.MaxResolutionM),
		TxAntennaID:      f.optID("tx_antenna_id", "TX antenna"),
		TxAzimuthDeg:     math.Mod(f.optNum("tx_azimuth_deg", "TX azimuth", 0, 360), 360),
		TxTiltDeg:        f.optNum("tx_tilt_deg", "TX tilt", -90, 90),
	}
	if c.ResolutionM > 0 && c.RangeM/c.ResolutionM > 1000 {
		f.Errors = append(f.Errors, "Range / resolution exceeds 1000 pixels per radius")
	}
	var err error
	if c.LegendDB, c.LegendColors, err = decodeLegend(f.get("legend")); err != nil {
		f.Errors = append(f.Errors, err.Error())
	}
	return c, f
}

func (s *Server) coverageForm(w http.ResponseWriter, r *http.Request, status int, d coverageFormData) {
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
	s.render(w, status, "coverage_form", d)
}

func (s *Server) listCoverages(w http.ResponseWriter, r *http.Request) {
	cs, err := s.Store.ListCoverages(r.Context(), currentUser(r).ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	sites, err := s.Store.CountSites(r.Context(), currentUser(r).ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	s.render(w, http.StatusOK, "coverages_list", struct {
		base
		Coverages []store.CoverageWithSite
		Sites     int
	}{s.base(r), cs, sites})
}

func (s *Server) newCoverage(w http.ResponseWriter, r *http.Request) {
	c := store.Coverage{FreqMHz: 145, TxPowerDBm: 40, RxHeightM: 2, RxSensitivityDBm: -120, Polarization: 1,
		RangeM: 30000, ResolutionM: 100}
	if from, ok := fromID(r); ok {
		orig, err := s.Store.GetCoverage(r.Context(), currentUser(r).ID, from)
		if err != nil {
			s.fail(w, err)
			return
		}
		c = orig.Coverage
		c.ID, c.Name = 0, c.Name+" (copy)"
		c.ComputedAt, c.ComputeMs, c.Note, c.Bounds, c.Job = nil, 0, "", nil, store.Job{}
	}
	s.coverageForm(w, r, http.StatusOK, coverageFormData{Coverage: c, Form: coverageValues(c)})
}

func (s *Server) createCoverage(w http.ResponseWriter, r *http.Request) {
	c, f := parseCoverage(r)
	if len(f.Errors) > 0 {
		s.coverageForm(w, r, http.StatusUnprocessableEntity, coverageFormData{Coverage: c, Form: f.values(), Errors: f.Errors})
		return
	}
	c.TxGainDBi = s.antennaGain(r, c.TxAntennaID, c.TxGainDBi)
	c.OwnerID = currentUser(r).ID
	id, err := s.Store.CreateCoverage(r.Context(), c)
	if errors.Is(err, store.ErrQuota) {
		s.coverageForm(w, r, http.StatusUnprocessableEntity, coverageFormData{Coverage: c, Form: f.values(),
			Errors: []string{fmt.Sprintf("Limit of %d coverages reached", store.MaxCoveragesPerUser)}})
		return
	}
	if err != nil {
		s.fail(w, err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/coverages/%d", id), http.StatusSeeOther)
}

func (s *Server) editCoverage(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		s.notFound(w, r)
		return
	}
	c, err := s.Store.GetCoverage(r.Context(), currentUser(r).ID, id)
	if err != nil {
		s.fail(w, err)
		return
	}
	s.coverageForm(w, r, http.StatusOK, coverageFormData{Coverage: c.Coverage, Form: coverageValues(c.Coverage)})
}

func (s *Server) updateCoverage(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		s.notFound(w, r)
		return
	}
	c, f := parseCoverage(r)
	c.ID, c.OwnerID = id, currentUser(r).ID
	if len(f.Errors) > 0 {
		s.coverageForm(w, r, http.StatusUnprocessableEntity, coverageFormData{Coverage: c, Form: f.values(), Errors: f.Errors})
		return
	}
	c.TxGainDBi = s.antennaGain(r, c.TxAntennaID, c.TxGainDBi)
	if err := s.Store.UpdateCoverage(r.Context(), c); err != nil {
		s.fail(w, err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/coverages/%d", id), http.StatusSeeOther)
}

func (s *Server) deleteCoverage(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		s.notFound(w, r)
		return
	}
	if err := s.Store.DeleteCoverage(r.Context(), currentUser(r).ID, id); err != nil {
		s.fail(w, err)
		return
	}
	http.Redirect(w, r, "/coverages", http.StatusSeeOther)
}

func (s *Server) showCoverage(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		s.notFound(w, r)
		return
	}
	c, err := s.Store.GetCoverage(r.Context(), currentUser(r).ID, id)
	if err != nil {
		s.fail(w, err)
		return
	}
	others, err := s.otherRasters(r, c)
	if err != nil {
		s.fail(w, err)
		return
	}
	data := s.coverageDetail(r, c, fmt.Sprintf("/coverages/%d", id))
	data.base = s.base(r)
	data.Others = others
	data.ViewURL = fmt.Sprintf("/coverages/%d/view", id)
	data.Error = r.URL.Query().Get("error")
	if c.ShareToken != "" {
		data.ShareURL = s.shareURL("/s/c/" + c.ShareToken)
	}
	s.render(w, http.StatusOK, "coverage_detail", data)
}

func (s *Server) coverageDetail(r *http.Request, c store.CoverageWithSite, path string) coverageDetailData {
	return coverageDetailData{Coverage: c, legendData: legendViews(legendOf(c.Coverage)), Path: path,
		TxAntenna: s.antennaName(r, c.OwnerID, c.TxAntennaID)}
}

func (s *Server) coverageReport(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		s.notFound(w, r)
		return
	}
	c, err := s.Store.GetCoverage(r.Context(), currentUser(r).ID, id)
	if err != nil {
		s.fail(w, err)
		return
	}
	s.sendCoverageReport(w, r, c, s.base(r).User)
}

func (s *Server) sendCoverageReport(w http.ResponseWriter, r *http.Request, c store.CoverageWithSite, user string) {
	in := report.Coverage{Coverage: c, TxAntenna: s.antennaName(r, c.OwnerID, c.TxAntennaID),
		User: user, Generated: time.Now(), View: rangeView(c), Legend: legendOf(c.Coverage)}
	in.Map = s.reportMap(r, in.View)
	if c.Bounds != nil {
		ra, err := s.Store.CoverageRaster(r.Context(), c.OwnerID, c.ID)
		if err != nil {
			s.fail(w, err)
			return
		}
		if in.Raster, err = renderRaster(ra); err != nil {
			s.fail(w, err)
			return
		}
	}
	pdf, err := report.CoveragePDF(in)
	if err != nil {
		s.fail(w, err)
		return
	}
	servePDF(w, c.Name, pdf)
}

func (s *Server) otherRasters(r *http.Request, c store.CoverageWithSite) ([]overlay, error) {
	cs, err := s.Store.ListCoverages(r.Context(), currentUser(r).ID)
	if err != nil {
		return nil, err
	}
	out := make([]overlay, 0, len(cs))
	for _, o := range cs {
		if o.ID != c.ID && o.ComputedAt != nil && o.Bounds != nil {
			out = append(out, overlay{CoverageWithSite: o, Checked: slices.Contains(c.ViewOverlays, o.ID),
				URL: fmt.Sprintf("/coverages/%d/image.png?t=%d", o.ID, o.ComputedAt.Unix())})
		}
	}
	return out, nil
}

func (s *Server) setCoverageView(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		s.notFound(w, r)
		return
	}
	opacity, err := strconv.ParseInt(r.FormValue("opacity"), 10, 16)
	if err != nil || opacity < 0 || opacity > 100 {
		http.Error(w, "bad opacity", http.StatusBadRequest)
		return
	}
	circles := r.FormValue("circles") != "0"
	if err := s.Store.SetCoverageView(r.Context(), currentUser(r).ID, id, int16(opacity), viewIDs(r.FormValue("overlays")), circles); err != nil {
		s.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func viewIDs(v string) []int64 {
	var out []int64
	for _, p := range strings.Split(v, ",") {
		n, err := strconv.ParseInt(strings.TrimSpace(p), 10, 64)
		if err != nil || n <= 0 {
			continue
		}
		if out = append(out, n); len(out) == store.MaxCoveragesPerUser {
			break
		}
	}
	return out
}

func (s *Server) computeCoverage(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		s.notFound(w, r)
		return
	}
	err = s.Store.EnqueueCoverage(r.Context(), currentUser(r).ID, id)
	if errors.Is(err, store.ErrQuota) {
		msg := fmt.Sprintf("Limit of %d jobs at a time reached, wait for one to finish.", store.MaxJobsPerUser)
		http.Redirect(w, r, fmt.Sprintf("/coverages/%d?error=%s", id, escapeQuery(msg)), http.StatusSeeOther)
		return
	}
	if err != nil {
		s.fail(w, err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/coverages/%d", id), http.StatusSeeOther)
}

const maxCompositeRasters = 20

func (s *Server) coverageComposite(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		s.notFound(w, r)
		return
	}
	c, err := s.Store.GetCoverage(r.Context(), currentUser(r).ID, id)
	if err != nil {
		s.fail(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	s.composite(w, r, c.OwnerID, withIDs(r.URL.Query().Get("with")), c.ID, legendOf(c.Coverage))
}

func withIDs(v string) []int64 {
	ids := slices.Compact(slices.Sorted(slices.Values(viewIDs(v))))
	return ids[:min(len(ids), maxCompositeRasters)]
}

func (s *Server) composite(w http.ResponseWriter, r *http.Request, owner int64, with []int64, own int64, l plot.Legend) {
	ids := append(slices.Clone(with), own)
	rasters, err := s.Store.CoverageRasters(r.Context(), owner, ids)
	if err != nil {
		s.fail(w, err)
		return
	}
	found := make(map[int64]bool, len(rasters))
	for _, ra := range rasters {
		found[ra.ID] = true
	}
	for _, n := range with {
		if !found[n] {
			s.notFound(w, r)
			return
		}
	}
	if len(rasters) == 0 {
		s.notFound(w, r)
		return
	}
	results := make([]coverage.Result, 0, len(rasters))
	for _, ra := range rasters {
		b := ra.Margin
		if b == nil {
			b = ra.PNG
		}
		res, err := plot.DecodeRaster(b, ra.Bounds.North, ra.Bounds.South, ra.Bounds.East, ra.Bounds.West)
		if err != nil {
			s.fail(w, err)
			return
		}
		results = append(results, res)
	}
	m := coverage.Merge(results)
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("X-Raster-Bounds", fmt.Sprintf("%.6f,%.6f,%.6f,%.6f", m.South, m.West, m.North, m.East))
	w.Write(plot.CompositePNG(m, l))
}

func (s *Server) coverageImage(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		s.notFound(w, r)
		return
	}
	ra, err := s.Store.CoverageRaster(r.Context(), currentUser(r).ID, id)
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
	w.Header().Set("Cache-Control", "no-cache")
	w.Write(png)
}
