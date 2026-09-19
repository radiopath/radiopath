package web

import (
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"github.com/radiopath/radiopath/internal/antenna"
	"github.com/radiopath/radiopath/internal/plot"
	"github.com/radiopath/radiopath/internal/store"
)

const maxPatternBytes = 1 << 20

type antennaView struct {
	store.Antenna
	FrontDB float64
	Plot    template.HTML
	PlotV   template.HTML
	Manual  bool
	Form    map[string]string
}

func antennaViews(as []store.Antenna) []antennaView {
	out := make([]antennaView, len(as))
	for i, a := range as {
		p := antenna.Pattern(a.PatternH)
		out[i] = antennaView{Antenna: a, FrontDB: p.Front(), Plot: template.HTML(plot.PatternSVG(p, 96)), Manual: a.Source == manualSource}
		if a.PatternV != nil {
			out[i].PlotV = template.HTML(plot.ElevationSVG(a.PatternV, 96))
		}
		if out[i].Manual {
			out[i].Form = antennaValues(a)
		}
	}
	return out
}

type antennasData struct {
	base
	Antennas []antennaView
	Errors   []string
	Edit     int64
}

func (s *Server) listAntennas(w http.ResponseWriter, r *http.Request) {
	s.renderAntennas(w, r, http.StatusOK, nil)
}

func (s *Server) renderAntennas(w http.ResponseWriter, r *http.Request, status int, errs []string) {
	as, err := s.Store.ListAntennas(r.Context(), currentUser(r).ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	s.render(w, status, "antennas_list", antennasData{base: s.base(r), Antennas: antennaViews(as), Errors: errs})
}

func (s *Server) createAntenna(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 2*maxPatternBytes)
	if err := r.ParseMultipartForm(maxPatternBytes); err != nil {
		s.renderAntennas(w, r, http.StatusUnprocessableEntity, []string{"Upload failed: " + err.Error()})
		return
	}
	f, hdr, err := r.FormFile("file")
	if err != nil {
		s.renderAntennas(w, r, http.StatusUnprocessableEntity, []string{"Pattern file: required"})
		return
	}
	defer f.Close()

	a, err := antenna.ParseMSI(f)
	if err != nil {
		s.renderAntennas(w, r, http.StatusUnprocessableEntity, []string{err.Error()})
		return
	}

	name := strings.TrimSpace(r.PostFormValue("name"))
	if name == "" {
		name = strings.TrimSpace(a.Name)
	}
	if name == "" {
		name = hdr.Filename
	}
	rec := store.Antenna{
		Name: name, GainDBi: a.GainDBi, FreqMHz: a.FreqMHz, PatternH: a.Horizontal, PatternV: a.Vertical,
		Source: hdr.Filename, OwnerID: currentUser(r).ID,
	}
	if _, err := s.Store.CreateAntenna(r.Context(), rec); err != nil {
		s.fail(w, err)
		return
	}
	http.Redirect(w, r, "/antennas", http.StatusSeeOther)
}

const manualSource = "manual"

func parseManualAntenna(r *http.Request) (store.Antenna, *form) {
	f := &form{r: r}
	rec := store.Antenna{
		Name:    f.text("name", "Name"),
		GainDBi: f.num("gain_dbi", "Gain", -50, 100),
		FreqMHz: f.optNum("freq_mhz", "Frequency", 0, 20000),
		Source:  manualSource,
		OwnerID: currentUser(r).ID,
	}
	beam := f.num("beamwidth_deg", "Beamwidth", 1, 360)
	fb := f.optNum("front_back_db", "Front-to-back", 0, 60)
	rec.PatternH = antenna.Sector(beam, fb)
	if vbeam := f.optNum("v_beamwidth_deg", "Vertical beamwidth", 1, 360); vbeam > 0 {
		rec.PatternV = antenna.Sector(vbeam, fb)
	}
	return rec, f
}

func (s *Server) createManualAntenna(w http.ResponseWriter, r *http.Request) {
	rec, f := parseManualAntenna(r)
	if len(f.Errors) > 0 {
		s.renderAntennas(w, r, http.StatusUnprocessableEntity, f.Errors)
		return
	}
	if _, err := s.Store.CreateAntenna(r.Context(), rec); err != nil {
		s.fail(w, err)
		return
	}
	http.Redirect(w, r, "/antennas", http.StatusSeeOther)
}

func antennaValues(a store.Antenna) map[string]string {
	beam, fb := antenna.SectorParams(a.PatternH)
	v := map[string]string{
		"name":          a.Name,
		"gain_dbi":      strconv.FormatFloat(a.GainDBi, 'f', -1, 64),
		"beamwidth_deg": strconv.FormatFloat(beam, 'f', -1, 64),
		"front_back_db": strconv.FormatFloat(fb, 'f', -1, 64),
		"freq_mhz":      optNumValue(nonZero(a.FreqMHz)),
	}
	if a.PatternV != nil {
		vbeam, _ := antenna.SectorParams(a.PatternV)
		v["v_beamwidth_deg"] = strconv.FormatFloat(vbeam, 'f', -1, 64)
	}
	return v
}

func nonZero(v float64) *float64 {
	if v == 0 {
		return nil
	}
	return &v
}

func (s *Server) updateAntenna(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	a, err := s.Store.GetAntenna(r.Context(), currentUser(r).ID, id)
	if err != nil {
		s.fail(w, err)
		return
	}
	if a.Source != manualSource {
		s.notFound(w, r)
		return
	}
	rec, f := parseManualAntenna(r)
	rec.ID = a.ID
	if len(f.Errors) > 0 {
		as, err := s.Store.ListAntennas(r.Context(), currentUser(r).ID)
		if err != nil {
			s.fail(w, err)
			return
		}
		views := antennaViews(as)
		for i := range views {
			if views[i].ID == a.ID {
				views[i].Form = f.values()
			}
		}
		s.render(w, http.StatusUnprocessableEntity, "antennas_list", antennasData{base: s.base(r), Antennas: views, Errors: f.Errors, Edit: a.ID})
		return
	}
	if err := s.Store.UpdateAntenna(r.Context(), rec); err != nil {
		s.fail(w, err)
		return
	}
	http.Redirect(w, r, "/antennas", http.StatusSeeOther)
}
func (s *Server) deleteAntenna(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := s.Store.DeleteAntenna(r.Context(), currentUser(r).ID, id); err != nil {
		s.fail(w, err)
		return
	}
	http.Redirect(w, r, "/antennas", http.StatusSeeOther)
}

func (s *Server) pattern(r *http.Request, owner int64, id *int64) antenna.Lobe {
	if id == nil {
		return antenna.Lobe{}
	}
	a, err := s.Store.GetAntenna(r.Context(), owner, *id)
	if err != nil {
		s.Log.Warn("antenna pattern not loaded", "antenna", *id, "err", err)
		return antenna.Lobe{}
	}
	return antenna.Lobe{H: a.PatternH, V: a.PatternV}
}

func azimuth(deg *float64, fallback float64) float64 {
	if deg == nil {
		return fallback
	}
	return *deg
}

func optNumValue(deg *float64) string {
	if deg == nil {
		return ""
	}
	return strconv.FormatFloat(*deg, 'f', -1, 64)
}

func (s *Server) antennaGain(r *http.Request, id *int64, entered float64) float64 {
	if id == nil {
		return entered
	}
	a, err := s.Store.GetAntenna(r.Context(), currentUser(r).ID, *id)
	if err != nil {
		return entered
	}
	return a.GainDBi
}

func (s *Server) antennaName(r *http.Request, owner int64, id *int64) string {
	if id == nil {
		return ""
	}
	a, err := s.Store.GetAntenna(r.Context(), owner, *id)
	if err != nil {
		return ""
	}
	return a.Name
}
