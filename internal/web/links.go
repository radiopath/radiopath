package web

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/radiopath/radiopath/internal/geo"
	"github.com/radiopath/radiopath/internal/itm"
	"github.com/radiopath/radiopath/internal/link"
	"github.com/radiopath/radiopath/internal/metrics"
	"github.com/radiopath/radiopath/internal/plot"
	"github.com/radiopath/radiopath/internal/report"
	"github.com/radiopath/radiopath/internal/store"
	"github.com/radiopath/radiopath/internal/tiles"
)

const (
	scopeLinksRead  = "links:read"
	scopeLinksWrite = "links:write"
)

type LinkInput struct {
	Name          string  `json:"name"`
	SiteAID       int64   `json:"site_a_id" doc:"Transmitting site"`
	SiteBID       int64   `json:"site_b_id" doc:"Receiving site"`
	FreqMHz       float64 `json:"frequency_mhz"`
	Polarization  string  `json:"polarization" enum:"horizontal,vertical"`
	ClutterLossDB float64 `json:"clutter_loss_db,omitempty" doc:"Extra path loss on top of the model"`

	TxPowerDBm   float64  `json:"tx_power_dbm"`
	TxGainDBi    float64  `json:"tx_gain_dbi,omitempty" doc:"Ignored with tx_antenna_id: the antenna's gain is used"`
	TxLineLossDB float64  `json:"tx_line_loss_db,omitempty"`
	TxAntennaID  *int64   `json:"tx_antenna_id,omitempty" doc:"Antenna pattern at A; omnidirectional without"`
	TxAzimuthDeg *float64 `json:"tx_azimuth_deg,omitempty" doc:"Main lobe azimuth at A; without it the antenna aims at B"`
	TxTiltDeg    float64  `json:"tx_tilt_deg,omitempty" doc:"Mechanical tilt, positive downwards"`

	RxGainDBi        float64  `json:"rx_gain_dbi,omitempty" doc:"Ignored with rx_antenna_id: the antenna's gain is used"`
	RxLineLossDB     float64  `json:"rx_line_loss_db,omitempty"`
	RxSensitivityDBm float64  `json:"rx_sensitivity_dbm"`
	RxAntennaID      *int64   `json:"rx_antenna_id,omitempty" doc:"Antenna pattern at B; omnidirectional without"`
	RxAzimuthDeg     *float64 `json:"rx_azimuth_deg,omitempty" doc:"Main lobe azimuth at B; without it the antenna aims at A"`
	RxTiltDeg        float64  `json:"rx_tilt_deg,omitempty" doc:"Mechanical tilt, positive downwards"`

	MeasuredRxDBm *float64 `json:"measured_rx_dbm,omitempty" doc:"Level actually read at B"`
}

type Share struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
}

type Link struct {
	ID int64 `json:"id"`
	LinkInput
	SiteA Site   `json:"site_a"`
	SiteB Site   `json:"site_b"`
	Share *Share `json:"share,omitempty" doc:"The public read-only link; missing when not shared"`
}

type LinkAnalysis struct {
	DistanceM           float64  `json:"distance_m"`
	BearingDeg          float64  `json:"bearing_deg"`
	TerrainAM           float64  `json:"terrain_a_m" doc:"Ground elevation at A"`
	TerrainBM           float64  `json:"terrain_b_m" doc:"Ground elevation at B"`
	FreeSpaceLossDB     float64  `json:"free_space_loss_db"`
	PathLossDB          float64  `json:"path_loss_db" doc:"ITM path loss, median"`
	PathLossRelDB       float64  `json:"path_loss_rel_db" doc:"ITM path loss at reliability_pct of the time"`
	Mode                string   `json:"mode" doc:"ITM propagation mode"`
	Warnings            []string `json:"warnings"`
	EIRPDBm             float64  `json:"eirp_dbm"`
	RxLevelDBm          float64  `json:"rx_level_dbm"`
	MarginDB            float64  `json:"margin_db" doc:"Link margin, median"`
	MarginRelDB         float64  `json:"margin_rel_db" doc:"Link margin at reliability_pct of the time"`
	TxTakeoffDeg        float64  `json:"tx_takeoff_deg"`
	RxTakeoffDeg        float64  `json:"rx_takeoff_deg"`
	TxOffBoresightDeg   float64  `json:"tx_off_boresight_deg"`
	RxOffBoresightDeg   float64  `json:"rx_off_boresight_deg"`
	TxPatternLossDB     float64  `json:"tx_pattern_loss_db"`
	RxPatternLossDB     float64  `json:"rx_pattern_loss_db"`
	ClutterLossDB       float64  `json:"clutter_loss_db"`
	HasCanopy           bool     `json:"has_canopy"`
	CanopyPathAM        float64  `json:"canopy_path_a_m"`
	CanopyPathBM        float64  `json:"canopy_path_b_m"`
	VegetationLossDB    float64  `json:"vegetation_loss_db" doc:"ITU-R P.833"`
	WorstClearancePct   float64  `json:"worst_clearance_pct" doc:"Worst first Fresnel zone clearance"`
	WorstClearanceDistM float64  `json:"worst_clearance_dist_m"`
	ProfileSVG          string   `json:"profile_svg" doc:"Terrain profile as SVG, styled by the classes of the SPA"`
}

type LinkDetail struct {
	Link
	TxAntenna      string        `json:"tx_antenna" doc:"Name of the antenna at A; empty for omnidirectional"`
	RxAntenna      string        `json:"rx_antenna" doc:"Name of the antenna at B; empty for omnidirectional"`
	TxAimDeg       float64       `json:"tx_aim_deg" doc:"Azimuth the antenna at A points to"`
	RxAimDeg       float64       `json:"rx_aim_deg" doc:"Azimuth the antenna at B points to"`
	ReliabilityPct float64       `json:"reliability_pct"`
	Analysis       *LinkAnalysis `json:"analysis,omitempty" doc:"Missing when the calculation failed, see error"`
	Error          string        `json:"error,omitempty"`
}

type LinkDetailOutput struct{ Body LinkDetail }

// linkData is a link with its analysis, the input of the detail view and the PDF report.
type linkData struct {
	Link         store.LinkWithSites
	Result       *link.Result
	Error        string
	TxAntenna    string
	RxAntenna    string
	TxAzimuthDeg float64
	RxAzimuthDeg float64
}

// polarizations maps the stored polarization to its API name.
var polarizations = []string{"horizontal", "vertical"}

// toLink returns the API view of a stored link; shareBase is the public share URL prefix.
func toLink(l store.LinkWithSites, shareBase string) Link {
	out := Link{ID: l.ID, SiteA: toSite(l.SiteA), SiteB: toSite(l.SiteB), LinkInput: LinkInput{
		Name: l.Name, SiteAID: l.SiteAID, SiteBID: l.SiteBID, FreqMHz: l.FreqMHz, Polarization: polarizations[l.Polarization],
		ClutterLossDB: l.ClutterLossDB,
		TxPowerDBm:    l.TxPowerDBm, TxGainDBi: l.TxGainDBi, TxLineLossDB: l.TxLineLossDB,
		TxAntennaID: l.TxAntennaID, TxAzimuthDeg: l.TxAzimuthDeg, TxTiltDeg: l.TxTiltDeg,
		RxGainDBi: l.RxGainDBi, RxLineLossDB: l.RxLineLossDB, RxSensitivityDBm: l.RxSensitivityDBm,
		RxAntennaID: l.RxAntennaID, RxAzimuthDeg: l.RxAzimuthDeg, RxTiltDeg: l.RxTiltDeg,
		MeasuredRxDBm: l.MeasuredRxDBm,
	}}
	if shareBase != "" && l.ShareToken != "" && l.ShareExpires != nil {
		out.Share = &Share{URL: shareBase + l.ShareToken, ExpiresAt: *l.ShareExpires}
	}
	return out
}

// toLinkDetail returns the API view of a link with its analysis.
func toLinkDetail(d linkData, shareBase string) LinkDetail {
	out := LinkDetail{Link: toLink(d.Link, shareBase), TxAntenna: d.TxAntenna, RxAntenna: d.RxAntenna,
		TxAimDeg: d.TxAzimuthDeg, RxAimDeg: d.RxAzimuthDeg, ReliabilityPct: link.ReliabilityPct, Error: d.Error}
	if r := d.Result; r != nil {
		warnings := r.Warnings
		if warnings == nil {
			warnings = []string{}
		}
		out.Analysis = &LinkAnalysis{
			DistanceM: r.DistanceM, BearingDeg: r.BearingDeg, TerrainAM: r.TerrainAM, TerrainBM: r.TerrainBM,
			FreeSpaceLossDB: r.FreeSpaceLossDB, PathLossDB: r.PathLossDB, PathLossRelDB: r.PathLossRelDB,
			Mode: r.Mode.String(), Warnings: warnings, EIRPDBm: r.EIRPdBm, RxLevelDBm: r.RxLevelDBm,
			MarginDB: r.MarginDB, MarginRelDB: r.MarginRelDB,
			TxTakeoffDeg: r.TxTakeoffDeg, RxTakeoffDeg: r.RxTakeoffDeg,
			TxOffBoresightDeg: r.TxOffBoresightDeg, RxOffBoresightDeg: r.RxOffBoresightDeg,
			TxPatternLossDB: r.TxPatternLossDB, RxPatternLossDB: r.RxPatternLossDB, ClutterLossDB: r.ClutterLossDB,
			HasCanopy: r.HasCanopy, CanopyPathAM: r.CanopyPathAM, CanopyPathBM: r.CanopyPathBM, VegetationLossDB: r.VegetationLossDB,
			WorstClearancePct: r.WorstClearancePct, WorstClearanceDistM: r.WorstClearanceDistM,
			ProfileSVG: string(plot.SVG(*r, d.Link.SiteA.AntennaHeightM, d.Link.SiteB.AntennaHeightM)),
		}
	}
	return out
}

// parseLink validates a link input of owner; site A and B must differ.
func parseLink(in LinkInput, owner int64) (store.Link, error) {
	var c check
	l := store.Link{
		OwnerID:          owner,
		Name:             c.text("name", "Name", in.Name),
		SiteAID:          in.SiteAID,
		SiteBID:          in.SiteBID,
		FreqMHz:          c.num("frequency_mhz", "Frequency", in.FreqMHz, 20, 20000),
		TxPowerDBm:       c.num("tx_power_dbm", "TX power", in.TxPowerDBm, -100, 200),
		TxGainDBi:        c.num("tx_gain_dbi", "TX gain", in.TxGainDBi, -50, 100),
		TxLineLossDB:     c.num("tx_line_loss_db", "TX line loss", in.TxLineLossDB, 0, 100),
		RxGainDBi:        c.num("rx_gain_dbi", "RX gain", in.RxGainDBi, -50, 100),
		RxLineLossDB:     c.num("rx_line_loss_db", "RX line loss", in.RxLineLossDB, 0, 100),
		RxSensitivityDBm: c.num("rx_sensitivity_dbm", "RX sensitivity", in.RxSensitivityDBm, -200, 50),
		TxAntennaID:      in.TxAntennaID,
		TxAzimuthDeg:     c.deg("tx_azimuth_deg", "TX azimuth", in.TxAzimuthDeg),
		TxTiltDeg:        c.num("tx_tilt_deg", "TX tilt", in.TxTiltDeg, -90, 90),
		RxAntennaID:      in.RxAntennaID,
		RxAzimuthDeg:     c.deg("rx_azimuth_deg", "RX azimuth", in.RxAzimuthDeg),
		RxTiltDeg:        c.num("rx_tilt_deg", "RX tilt", in.RxTiltDeg, -90, 90),
		ClutterLossDB:    c.num("clutter_loss_db", "Extra path loss", in.ClutterLossDB, 0, 200),
		MeasuredRxDBm:    c.optNum("measured_rx_dbm", "Measured RX level", in.MeasuredRxDBm, -200, 50),
	}
	if in.Polarization == "horizontal" {
		l.Polarization = 0
	} else {
		l.Polarization = 1
	}
	if l.SiteAID == l.SiteBID {
		c.add("", "Site A and B must differ")
	}
	return l, c.err()
}

// registerLinks registers the link operations and the public view of a shared link.
func (s *Server) registerLinks(api huma.API) {
	huma.Register(api, op("list-links", http.MethodGet, "/api/links", scopeLinksRead, "The user's links with their sites"),
		func(ctx context.Context, _ *struct{}) (*struct{ Body []Link }, error) {
			links, err := s.Store.ListLinks(ctx, userOf(ctx).ID)
			if err != nil {
				return nil, s.storeErr(err)
			}
			out := make([]Link, 0, len(links))
			for _, l := range links {
				out = append(out, toLink(l, s.shareURL("/s/l/")))
			}
			return &struct{ Body []Link }{out}, nil
		})

	o := op("create-link", http.MethodPost, "/api/links", scopeLinksWrite, "Create a link")
	o.DefaultStatus = http.StatusCreated
	huma.Register(api, o, func(ctx context.Context, in *struct{ Body LinkInput }) (*struct{ Body Link }, error) {
		owner := userOf(ctx).ID
		l, err := parseLink(in.Body, owner)
		if err != nil {
			return nil, err
		}
		s.applyAntennaGains(ctx, &l)
		id, err := s.Store.CreateLink(ctx, l)
		if err != nil {
			return nil, s.storeErr(err)
		}
		return s.linkOut(ctx, owner, id)
	})

	o = op("get-link", http.MethodGet, "/api/links/{id}", scopeLinksRead, "One link with its path analysis")
	o.Errors = append(o.Errors, http.StatusTooManyRequests)
	o.Description = "A worker computes the analysis; the result is kept for an hour and served again while the link, its sites and antennas stay the same. 60 analyses, reports and raster merges per user and minute."
	huma.Register(api, o,
		func(ctx context.Context, in *IDPath) (*LinkDetailOutput, error) {
			if err := s.computeHit(ctx, userOf(ctx).ID); err != nil {
				return nil, err
			}
			l, err := s.Store.GetLink(ctx, userOf(ctx).ID, in.ID)
			if err != nil {
				return nil, s.storeErr(err)
			}
			return &LinkDetailOutput{toLinkDetail(s.linkAnalysis(ctx, l), s.shareURL("/s/l/"))}, nil
		})

	huma.Register(api, op("update-link", http.MethodPut, "/api/links/{id}", scopeLinksWrite, "Replace a link"),
		func(ctx context.Context, in *struct {
			IDPath
			Body LinkInput
		}) (*struct{ Body Link }, error) {
			owner := userOf(ctx).ID
			l, err := parseLink(in.Body, owner)
			if err != nil {
				return nil, err
			}
			l.ID = in.ID
			s.applyAntennaGains(ctx, &l)
			if err := s.Store.UpdateLink(ctx, l); err != nil {
				return nil, s.storeErr(err)
			}
			return s.linkOut(ctx, owner, in.ID)
		})

	o = op("delete-link", http.MethodDelete, "/api/links/{id}", scopeLinksWrite, "Delete a link")
	o.DefaultStatus = http.StatusNoContent
	huma.Register(api, o, func(ctx context.Context, in *IDPath) (*struct{}, error) {
		if err := s.Store.DeleteLink(ctx, userOf(ctx).ID, in.ID); err != nil {
			return nil, s.storeErr(err)
		}
		return nil, nil
	})

	o = op("share-link", http.MethodPut, "/api/links/{id}/share", scopeLinksWrite, "Publish a link read-only for 30 days, or extend it")
	o.Description = "An existing public URL is kept; its expiry moves to 30 days from now."
	huma.Register(api, o, func(ctx context.Context, in *IDPath) (*struct{ Body Share }, error) {
		owner := userOf(ctx).ID
		if err := s.Store.ShareLink(ctx, owner, in.ID, randomToken(), time.Now().Add(shareTTL)); err != nil {
			return nil, s.storeErr(err)
		}
		metrics.Events.WithLabelValues(metrics.EventSharePublished).Inc()
		out, err := s.linkOut(ctx, owner, in.ID)
		if err != nil {
			return nil, err
		}
		return &struct{ Body Share }{*out.Body.Share}, nil
	})

	o = op("unshare-link", http.MethodDelete, "/api/links/{id}/share", scopeLinksWrite, "Revoke the public URL of a link")
	o.DefaultStatus = http.StatusNoContent
	huma.Register(api, o, func(ctx context.Context, in *IDPath) (*struct{}, error) {
		if err := s.Store.UnshareLink(ctx, userOf(ctx).ID, in.ID); err != nil {
			return nil, s.storeErr(err)
		}
		metrics.Events.WithLabelValues(metrics.EventShareRevoked).Inc()
		return nil, nil
	})

	o = op("get-shared-link", http.MethodGet, "/api/share/links/{token}", "", "A shared link with its path analysis, for anyone with the token")
	o.Errors = []int{http.StatusNotFound, http.StatusTooManyRequests}
	huma.Register(api, o, func(ctx context.Context, in *TokenPath) (*LinkDetailOutput, error) {
		if err := s.shareHit(ctx, "l", in.Token); err != nil {
			return nil, err
		}
		l, err := s.Store.GetSharedLink(ctx, in.Token)
		if err != nil {
			return nil, s.storeErr(err)
		}
		return &LinkDetailOutput{toLinkDetail(s.linkAnalysis(ctx, l), s.shareURL("/s/l/"))}, nil
	})
}

// linkOut loads the owner's link as the response of a write.
func (s *Server) linkOut(ctx context.Context, owner, id int64) (*struct{ Body Link }, error) {
	l, err := s.Store.GetLink(ctx, owner, id)
	if err != nil {
		return nil, s.storeErr(err)
	}
	return &struct{ Body Link }{toLink(l, s.shareURL("/s/l/"))}, nil
}

// linkReport serves the owner's link analysis as a PDF report.
func (s *Server) linkReport(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	u := currentUser(r)
	l, err := s.Store.GetLink(r.Context(), u.ID, id)
	if err != nil {
		s.fail(w, err)
		return
	}
	s.sendLinkReport(w, r, s.linkAnalysis(r.Context(), l), u.Name)
}

// sendLinkReport renders the PDF with a map fitted to both sites and writes it to the response; user names who printed it, empty for a shared link.
func (s *Server) sendLinkReport(w http.ResponseWriter, r *http.Request, data linkData, user string) {
	a, b := data.Link.SiteA, data.Link.SiteB
	in := report.Link{
		Link: data.Link, Result: data.Result, Error: data.Error,
		TxAntenna: data.TxAntenna, RxAntenna: data.RxAntenna,
		TxAzimuthDeg: data.TxAzimuthDeg, RxAzimuthDeg: data.RxAzimuthDeg,
		ReliabilityPct: link.ReliabilityPct, User: user, Generated: time.Now(),
	}
	in.View = tiles.Fit(math.Min(a.Lat, b.Lat), math.Min(a.Lon, b.Lon), math.Max(a.Lat, b.Lat), math.Max(a.Lon, b.Lon), report.MapW, report.MapH, 80)
	in.Map = s.reportMap(r.Context(), in.View)
	pdf, err := report.LinkPDF(in)
	if err != nil {
		s.fail(w, err)
		return
	}
	servePDF(w, data.Link.Name, pdf)
}

// linkAnalysis has the path analysis of l computed by a worker (or takes the result a worker already has for the
// same input); a failed analysis is returned in Error instead of Result.
func (s *Server) linkAnalysis(ctx context.Context, l store.LinkWithSites) linkData {
	data := linkData{Link: l}
	data.TxAntenna, data.RxAntenna = s.antennaName(ctx, l.OwnerID, l.TxAntennaID), s.antennaName(ctx, l.OwnerID, l.RxAntennaID)

	bearing := geo.InitialBearing(geo.Point{Lat: l.SiteA.Lat, Lon: l.SiteA.Lon}, geo.Point{Lat: l.SiteB.Lat, Lon: l.SiteB.Lon})
	data.TxAzimuthDeg = azimuth(l.TxAzimuthDeg, bearing)
	data.RxAzimuthDeg = azimuth(l.RxAzimuthDeg, math.Mod(bearing+180, 360))

	input, _ := json.Marshal(link.Input{
		A:             link.Site{Name: l.SiteA.Name, Pos: geo.Point{Lat: l.SiteA.Lat, Lon: l.SiteA.Lon}, AntennaHeightM: l.SiteA.AntennaHeightM},
		B:             link.Site{Name: l.SiteB.Name, Pos: geo.Point{Lat: l.SiteB.Lat, Lon: l.SiteB.Lon}, AntennaHeightM: l.SiteB.AntennaHeightM},
		ClutterLossDB: l.ClutterLossDB,
		Radio: link.Radio{
			FreqMHz: l.FreqMHz, TxPowerDBm: l.TxPowerDBm, TxGainDBi: l.TxGainDBi, RxGainDBi: l.RxGainDBi,
			TxLineLossDB: l.TxLineLossDB, RxLineLossDB: l.RxLineLossDB, RxSensitivityDBm: l.RxSensitivityDBm,
			Polarization: itm.Polarization(l.Polarization),
			TxPattern:    s.pattern(ctx, l.OwnerID, l.TxAntennaID), TxAzimuthDeg: data.TxAzimuthDeg, TxTiltDeg: l.TxTiltDeg,
			RxPattern: s.pattern(ctx, l.OwnerID, l.RxAntennaID), RxAzimuthDeg: data.RxAzimuthDeg, RxTiltDeg: l.RxTiltDeg,
		},
	})
	j, err := s.runJob(ctx, l.OwnerID, kindLinkAnalysis, input)
	switch {
	case err != nil:
		if ctx.Err() == nil {
			s.Log.Warn("link analysis job", "link", l.ID, "err", err)
		}
		data.Error = jobReason(err)
	case j.State == store.JobFailed:
		data.Error = j.Error
	default:
		var res link.Result
		if err := json.Unmarshal(j.Result, &res); err != nil {
			s.Log.Error("link analysis result", "link", l.ID, "job", j.ID, "err", err)
			data.Error = jobReason(err)
		} else {
			data.Result = &res
		}
	}
	return data
}

// applyAntennaGains replaces the entered TX and RX gains by those of the selected antennas.
func (s *Server) applyAntennaGains(ctx context.Context, l *store.Link) {
	l.TxGainDBi = s.antennaGain(ctx, l.OwnerID, l.TxAntennaID, l.TxGainDBi)
	l.RxGainDBi = s.antennaGain(ctx, l.OwnerID, l.RxAntennaID, l.RxGainDBi)
}
