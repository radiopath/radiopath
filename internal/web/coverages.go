package web

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/radiopath/radiopath/internal/coverage"
	"github.com/radiopath/radiopath/internal/metrics"
	"github.com/radiopath/radiopath/internal/plot"
	"github.com/radiopath/radiopath/internal/report"
	"github.com/radiopath/radiopath/internal/store"
)

const (
	scopeCoveragesRead  = "coverages:read"
	scopeCoveragesWrite = "coverages:write"
)

type CoverageInput struct {
	Name             string  `json:"name"`
	SiteID           int64   `json:"site_id" doc:"Transmitting site"`
	FreqMHz          float64 `json:"frequency_mhz"`
	Polarization     string  `json:"polarization" enum:"horizontal,vertical"`
	TxPowerDBm       float64 `json:"tx_power_dbm"`
	TxGainDBi        float64 `json:"tx_gain_dbi,omitempty" doc:"Ignored with tx_antenna_id: the antenna's gain is used"`
	TxLineLossDB     float64 `json:"tx_line_loss_db,omitempty"`
	TxAntennaID      *int64  `json:"tx_antenna_id,omitempty" doc:"Antenna pattern; omnidirectional without"`
	TxAzimuthDeg     float64 `json:"tx_azimuth_deg,omitempty" doc:"Main lobe azimuth, 0 is north"`
	TxTiltDeg        float64 `json:"tx_tilt_deg,omitempty" doc:"Mechanical tilt, positive downwards"`
	RxHeightM        float64 `json:"rx_height_m" doc:"Receiving antenna above ground"`
	RxGainDBi        float64 `json:"rx_gain_dbi,omitempty"`
	RxLineLossDB     float64 `json:"rx_line_loss_db,omitempty"`
	RxSensitivityDBm float64 `json:"rx_sensitivity_dbm"`
	RangeKm          float64 `json:"range_km"`
	ResolutionM      float64 `json:"resolution_m" doc:"Pixel size; range / resolution must not exceed 1000"`
}

type NewCoverageInput struct {
	CoverageInput
	Legend []LegendBin `json:"legend,omitempty" doc:"Custom legend; the default without. PUT .../legend changes it later"`
}

type CoverageJob struct {
	State     string     `json:"state" enum:"idle,queued,running,done,failed"`
	QueuedAt  *time.Time `json:"queued_at" nullable:"true"`
	StartedAt *time.Time `json:"started_at" nullable:"true"`
	Error     string     `json:"error"`
}

type Bounds struct {
	North float64 `json:"north"`
	South float64 `json:"south"`
	East  float64 `json:"east"`
	West  float64 `json:"west"`
}

type CoverageView struct {
	Opacity  int     `json:"opacity" minimum:"0" maximum:"100" doc:"Raster opacity in percent"`
	Overlays []int64 `json:"overlays" doc:"Other computed coverages merged into the map"`
	Circles  bool    `json:"circles" doc:"Whether the range circles are drawn"`
}

type Coverage struct {
	ID int64 `json:"id"`
	CoverageInput
	Site         Site         `json:"site"`
	Legend       []LegendRow  `json:"legend" doc:"The legend in use, from high to low threshold"`
	LegendFloor  string       `json:"legend_floor" doc:"Label of margins below the lowest threshold"`
	CustomLegend bool         `json:"custom_legend" doc:"False while the default legend is in use"`
	Job          CoverageJob  `json:"job"`
	ComputedAt   *time.Time   `json:"computed_at" nullable:"true"`
	ComputeTime  string       `json:"compute_time" doc:"How long the last computation took, human readable"`
	Note         string       `json:"note" doc:"A remark of the last computation"`
	Bounds       *Bounds      `json:"bounds,omitempty" doc:"Extent of the raster; missing until computed"`
	ImageURL     string       `json:"image_url,omitempty" doc:"The raster as PNG; missing until computed"`
	View         CoverageView `json:"view"`
	Share        *Share       `json:"share,omitempty" doc:"The public read-only link; missing when not shared"`
}

type Overlay struct {
	ID       int64   `json:"id"`
	Name     string  `json:"name"`
	Site     Site    `json:"site"`
	FreqMHz  float64 `json:"frequency_mhz"`
	RangeKm  float64 `json:"range_km"`
	Bounds   Bounds  `json:"bounds"`
	ImageURL string  `json:"image_url"`
	Checked  bool    `json:"checked"`
}

type CoverageDetail struct {
	Coverage
	TxAntenna    string    `json:"tx_antenna" doc:"Name of the antenna; empty for omnidirectional"`
	CompositeURL string    `json:"composite_url" doc:"Merges this raster with ?with=<ids> of overlays into one PNG"`
	Overlays     []Overlay `json:"overlays" doc:"Rasters that can be shown on top: the other computed coverages, or the released ones of a shared view"`
}

type CoverageDetailOutput struct{ Body CoverageDetail }

// toCoverage returns the API view of a stored coverage; base is its URL path (/coverages/<id> or /s/c/<token>).
func (s *Server) toCoverage(c store.CoverageWithSite, base string) Coverage {
	l := legendOf(c.Coverage)
	out := Coverage{ID: c.ID, Site: toSite(c.Site), CoverageInput: CoverageInput{
		Name: c.Name, SiteID: c.SiteID, FreqMHz: c.FreqMHz, Polarization: polarizations[c.Polarization],
		TxPowerDBm: c.TxPowerDBm, TxGainDBi: c.TxGainDBi, TxLineLossDB: c.TxLineLossDB, TxAntennaID: c.TxAntennaID,
		TxAzimuthDeg: c.TxAzimuthDeg, TxTiltDeg: c.TxTiltDeg, RxHeightM: c.RxHeightM, RxGainDBi: c.RxGainDBi,
		RxLineLossDB: c.RxLineLossDB, RxSensitivityDBm: c.RxSensitivityDBm, RangeKm: c.RangeM / 1000, ResolutionM: c.ResolutionM,
	},
		Legend: legendRows(l), LegendFloor: l.Floor(), CustomLegend: len(c.LegendDB) > 0,
		Job:        CoverageJob{State: c.Job.State, QueuedAt: c.Job.QueuedAt, StartedAt: c.Job.StartedAt, Error: c.Job.Error},
		ComputedAt: c.ComputedAt, Note: c.Note,
		View: CoverageView{Opacity: int(c.ViewOpacity), Overlays: c.ViewOverlays, Circles: c.ViewCircles},
	}
	if out.View.Overlays == nil {
		out.View.Overlays = []int64{}
	}
	if c.ComputedAt != nil {
		out.ComputeTime = c.ComputeTime()
		if c.Bounds != nil {
			out.Bounds = &Bounds{c.Bounds.North, c.Bounds.South, c.Bounds.East, c.Bounds.West}
			out.ImageURL = fmt.Sprintf("%s/image.png?t=%d", base, c.ComputedAt.Unix())
		}
	}
	if c.ShareToken != "" && c.ShareExpires != nil {
		out.Share = &Share{URL: s.shareURL("/s/c/" + c.ShareToken), ExpiresAt: *c.ShareExpires}
	}
	return out
}

// toOverlay returns a computed coverage as an overlay with its image at url.
func toOverlay(c store.CoverageWithSite, url string, checked bool) Overlay {
	return Overlay{ID: c.ID, Name: c.Name, Site: toSite(c.Site), FreqMHz: c.FreqMHz, RangeKm: c.RangeM / 1000,
		Bounds: Bounds{c.Bounds.North, c.Bounds.South, c.Bounds.East, c.Bounds.West}, ImageURL: url, Checked: checked}
}

// coverageDetail returns the API view of a coverage with its antenna name, composite URL and overlays.
func (s *Server) coverageDetail(ctx context.Context, c store.CoverageWithSite, base string, overlays []Overlay) CoverageDetail {
	return CoverageDetail{Coverage: s.toCoverage(c, base), TxAntenna: s.antennaName(ctx, c.OwnerID, c.TxAntennaID),
		CompositeURL: base + "/composite.png", Overlays: overlays}
}

// parseCoverage validates a coverage input of owner, including the pixel limit (range / resolution).
func parseCoverage(in CoverageInput, owner int64) (store.Coverage, *check) {
	c := &check{}
	cov := store.Coverage{
		OwnerID:          owner,
		Name:             c.text("name", "Name", in.Name),
		SiteID:           in.SiteID,
		FreqMHz:          c.num("frequency_mhz", "Frequency", in.FreqMHz, 20, 20000),
		TxPowerDBm:       c.num("tx_power_dbm", "TX power", in.TxPowerDBm, -100, 200),
		TxGainDBi:        c.num("tx_gain_dbi", "TX gain", in.TxGainDBi, -50, 100),
		TxLineLossDB:     c.num("tx_line_loss_db", "TX line loss", in.TxLineLossDB, 0, 100),
		RxHeightM:        c.num("rx_height_m", "RX height", in.RxHeightM, 0.5, 3000),
		RxGainDBi:        c.num("rx_gain_dbi", "RX gain", in.RxGainDBi, -50, 100),
		RxLineLossDB:     c.num("rx_line_loss_db", "RX line loss", in.RxLineLossDB, 0, 100),
		RxSensitivityDBm: c.num("rx_sensitivity_dbm", "RX sensitivity", in.RxSensitivityDBm, -200, 50),
		RangeM:           c.num("range_km", "Range", in.RangeKm, coverage.MinRangeM/1000, coverage.MaxRangeM/1000) * 1000,
		ResolutionM:      c.num("resolution_m", "Resolution", in.ResolutionM, coverage.MinResolutionM, coverage.MaxResolutionM),
		TxAntennaID:      in.TxAntennaID,
		TxAzimuthDeg:     math.Mod(c.num("tx_azimuth_deg", "TX azimuth", in.TxAzimuthDeg, 0, 360), 360),
		TxTiltDeg:        c.num("tx_tilt_deg", "TX tilt", in.TxTiltDeg, -90, 90),
	}
	if in.Polarization == "vertical" {
		cov.Polarization = 1
	}
	if cov.ResolutionM > 0 && cov.RangeM/cov.ResolutionM > 1000 {
		c.add("", "Range / resolution exceeds 1000 pixels per radius")
	}
	return cov, c
}

// registerCoverages registers the coverage operations and the public view of a shared coverage.
func (s *Server) registerCoverages(api huma.API) {
	huma.Register(api, op("list-coverages", http.MethodGet, "/api/coverages", scopeCoveragesRead, "The user's coverages with their sites"),
		func(ctx context.Context, _ *struct{}) (*struct{ Body []Coverage }, error) {
			cs, err := s.Store.ListCoverages(ctx, userOf(ctx).ID)
			if err != nil {
				return nil, s.storeErr(err)
			}
			out := make([]Coverage, 0, len(cs))
			for _, c := range cs {
				out = append(out, s.toCoverage(c, fmt.Sprintf("/coverages/%d", c.ID)))
			}
			return &struct{ Body []Coverage }{out}, nil
		})

	o := op("create-coverage", http.MethodPost, "/api/coverages", scopeCoveragesWrite, "Create a coverage")
	o.DefaultStatus = http.StatusCreated
	huma.Register(api, o, func(ctx context.Context, in *struct{ Body NewCoverageInput }) (*struct{ Body Coverage }, error) {
		owner := userOf(ctx).ID
		cov, c := parseCoverage(in.Body.CoverageInput, owner)
		if len(in.Body.Legend) > 0 {
			l, err := newLegend(in.Body.Legend)
			if err != nil {
				c.add("legend", err.Error())
			} else {
				cov.LegendDB, cov.LegendColors = l.Mins(), l.Hex()
			}
		}
		if err := c.err(); err != nil {
			return nil, err
		}
		cov.TxGainDBi = s.antennaGain(ctx, owner, cov.TxAntennaID, cov.TxGainDBi)
		id, err := s.Store.CreateCoverage(ctx, cov)
		if errors.Is(err, store.ErrQuota) {
			return nil, huma.Error422UnprocessableEntity(fmt.Sprintf("Limit of %d coverages reached", store.MaxCoveragesPerUser))
		}
		if err != nil {
			return nil, s.storeErr(err)
		}
		return s.coverageOut(ctx, owner, id)
	})

	huma.Register(api, op("get-coverage", http.MethodGet, "/api/coverages/{id}", scopeCoveragesRead, "One coverage with its job state and the rasters that can be shown on top"),
		func(ctx context.Context, in *IDPath) (*CoverageDetailOutput, error) {
			owner := userOf(ctx).ID
			c, err := s.Store.GetCoverage(ctx, owner, in.ID)
			if err != nil {
				return nil, s.storeErr(err)
			}
			cs, err := s.Store.ListCoverages(ctx, owner)
			if err != nil {
				return nil, s.storeErr(err)
			}
			overlays := make([]Overlay, 0, len(cs))
			for _, o := range cs {
				if o.ID != c.ID && o.ComputedAt != nil && o.Bounds != nil {
					overlays = append(overlays, toOverlay(o, fmt.Sprintf("/coverages/%d/image.png?t=%d", o.ID, o.ComputedAt.Unix()),
						slices.Contains(c.ViewOverlays, o.ID)))
				}
			}
			return &CoverageDetailOutput{s.coverageDetail(ctx, c, fmt.Sprintf("/coverages/%d", c.ID), overlays)}, nil
		})

	o = op("update-coverage", http.MethodPut, "/api/coverages/{id}", scopeCoveragesWrite, "Replace a coverage")
	o.Description = "Clears the computed result; the legend stays."
	huma.Register(api, o, func(ctx context.Context, in *struct {
		IDPath
		Body CoverageInput
	}) (*struct{ Body Coverage }, error) {
		owner := userOf(ctx).ID
		cov, c := parseCoverage(in.Body, owner)
		if err := c.err(); err != nil {
			return nil, err
		}
		cov.ID = in.ID
		cov.TxGainDBi = s.antennaGain(ctx, owner, cov.TxAntennaID, cov.TxGainDBi)
		if err := s.Store.UpdateCoverage(ctx, cov); err != nil {
			return nil, s.storeErr(err)
		}
		return s.coverageOut(ctx, owner, in.ID)
	})

	o = op("delete-coverage", http.MethodDelete, "/api/coverages/{id}", scopeCoveragesWrite, "Delete a coverage")
	o.DefaultStatus = http.StatusNoContent
	huma.Register(api, o, func(ctx context.Context, in *IDPath) (*struct{}, error) {
		if err := s.Store.DeleteCoverage(ctx, userOf(ctx).ID, in.ID); err != nil {
			return nil, s.storeErr(err)
		}
		return nil, nil
	})

	o = op("compute-coverage", http.MethodPost, "/api/coverages/{id}/compute", scopeCoveragesWrite, "Queue the coverage for computation")
	o.DefaultStatus = http.StatusAccepted
	o.Errors = append(o.Errors, http.StatusUnprocessableEntity)
	o.Description = "A job already queued or running stays as it is. GET /coverages/{id}/events streams the job state as server-sent events."
	huma.Register(api, o, func(ctx context.Context, in *IDPath) (*struct{}, error) {
		err := s.Store.EnqueueCoverage(ctx, userOf(ctx).ID, in.ID)
		if errors.Is(err, store.ErrQuota) {
			return nil, huma.Error422UnprocessableEntity(fmt.Sprintf("Limit of %d jobs at a time reached, wait for one to finish.", store.MaxJobsPerUser))
		}
		if err != nil {
			return nil, s.storeErr(err)
		}
		return nil, nil
	})

	o = op("set-coverage-view", http.MethodPut, "/api/coverages/{id}/view", scopeCoveragesWrite, "Save how the map shows the coverage")
	o.DefaultStatus = http.StatusNoContent
	o.Description = "Overlays that are not other computed coverages of the user are dropped. A shared view shows the map the same way."
	huma.Register(api, o, func(ctx context.Context, in *struct {
		IDPath
		Body CoverageView
	}) (*struct{}, error) {
		ids := in.Body.Overlays[:min(len(in.Body.Overlays), store.MaxCoveragesPerUser)]
		if err := s.Store.SetCoverageView(ctx, userOf(ctx).ID, in.ID, int16(in.Body.Opacity), ids, in.Body.Circles); err != nil {
			return nil, s.storeErr(err)
		}
		return nil, nil
	})

	o = op("share-coverage", http.MethodPut, "/api/coverages/{id}/share", scopeCoveragesWrite, "Publish a coverage read-only for 30 days, or extend it")
	o.Description = "An existing public URL is kept; its expiry moves to 30 days from now. The shared view shows the overlays of the saved map view."
	huma.Register(api, o, func(ctx context.Context, in *IDPath) (*struct{ Body Share }, error) {
		owner := userOf(ctx).ID
		if err := s.Store.ShareCoverage(ctx, owner, in.ID, randomToken(), time.Now().Add(shareTTL)); err != nil {
			return nil, s.storeErr(err)
		}
		metrics.Events.WithLabelValues(metrics.EventSharePublished).Inc()
		out, err := s.coverageOut(ctx, owner, in.ID)
		if err != nil {
			return nil, err
		}
		return &struct{ Body Share }{*out.Body.Share}, nil
	})

	o = op("unshare-coverage", http.MethodDelete, "/api/coverages/{id}/share", scopeCoveragesWrite, "Revoke the public URL of a coverage")
	o.DefaultStatus = http.StatusNoContent
	huma.Register(api, o, func(ctx context.Context, in *IDPath) (*struct{}, error) {
		if err := s.Store.UnshareCoverage(ctx, userOf(ctx).ID, in.ID); err != nil {
			return nil, s.storeErr(err)
		}
		metrics.Events.WithLabelValues(metrics.EventShareRevoked).Inc()
		return nil, nil
	})

	o = op("get-shared-coverage", http.MethodGet, "/api/share/coverages/{token}", "", "A shared coverage with its released overlays, for anyone with the token")
	o.Errors = []int{http.StatusNotFound, http.StatusTooManyRequests}
	huma.Register(api, o, func(ctx context.Context, in *TokenPath) (*CoverageDetailOutput, error) {
		if err := s.shareHit(ctx, "c", in.Token); err != nil {
			return nil, err
		}
		c, err := s.Store.GetSharedCoverage(ctx, in.Token)
		if err != nil {
			return nil, s.storeErr(err)
		}
		extra, err := s.Store.CoveragesByID(ctx, c.OwnerID, c.ViewOverlays)
		if err != nil {
			return nil, s.storeErr(err)
		}
		overlays := make([]Overlay, 0, len(extra))
		for _, o := range extra {
			overlays = append(overlays, toOverlay(o, fmt.Sprintf("/s/c/%s/image.png?id=%d&t=%d", in.Token, o.ID, o.ComputedAt.Unix()), true))
		}
		return &CoverageDetailOutput{s.coverageDetail(ctx, c, "/s/c/"+in.Token, overlays)}, nil
	})

	s.registerLegend(api)
}

// coverageOut loads the owner's coverage as the response of a write.
func (s *Server) coverageOut(ctx context.Context, owner, id int64) (*struct{ Body Coverage }, error) {
	c, err := s.Store.GetCoverage(ctx, owner, id)
	if err != nil {
		return nil, s.storeErr(err)
	}
	return &struct{ Body Coverage }{s.toCoverage(c, fmt.Sprintf("/coverages/%d", id))}, nil
}

// coverageReport serves the owner's coverage as a PDF report.
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
	s.sendCoverageReport(w, r, c, currentUser(r).Name)
}

// sendCoverageReport renders the PDF with map and raster (when computed) and writes it to the response.
func (s *Server) sendCoverageReport(w http.ResponseWriter, r *http.Request, c store.CoverageWithSite, user string) {
	in := report.Coverage{Coverage: c, TxAntenna: s.antennaName(r.Context(), c.OwnerID, c.TxAntennaID),
		User: user, Generated: time.Now(), View: rangeView(c), Legend: legendOf(c.Coverage)}
	in.Map = s.reportMap(r.Context(), in.View)
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

// viewIDs parses a comma-separated ID list, skipping invalid entries and stopping at the per-user coverage limit.
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

const maxCompositeRasters = 20

// coverageComposite serves a PNG merging this coverage with the rasters named in the with parameter.
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

// withIDs parses, sorts and de-duplicates the with parameter, capped at maxCompositeRasters.
func withIDs(v string) []int64 {
	ids := slices.Compact(slices.Sorted(slices.Values(viewIDs(v))))
	return ids[:min(len(ids), maxCompositeRasters)]
}

// composite merges the owner's rasters (with plus own) into one PNG using legend l, with bounds in X-Raster-Bounds; a missing raster gives 404.
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

// coverageImage serves the coverage's raster as PNG.
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
