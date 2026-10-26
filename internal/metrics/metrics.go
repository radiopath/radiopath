package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HTTPRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "radiopath_http_requests_total",
		Help: "HTTP requests by route pattern, method, status code and the credential sent: token, session or none.",
	}, []string{"route", "method", "code", "auth"})

	HTTPDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "radiopath_http_request_duration_seconds",
		Help:    "HTTP request duration by route pattern.",
		Buckets: []float64{0.005, 0.025, 0.1, 0.25, 1, 2.5, 10},
	}, []string{"route"})
)

var (
	Jobs = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "radiopath_jobs_total",
		Help: "Finished jobs by kind and result: done, failed or requeued.",
	}, []string{"kind", "result"})

	JobDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "radiopath_job_duration_seconds",
		Help:    "Compute time of a successful job by kind.",
		Buckets: []float64{0.01, 0.05, 0.25, 1, 2.5, 5, 10, 30, 60, 120, 300, 600},
	}, []string{"kind"})
)

var (
	LinkAnalyses = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "radiopath_link_analyses_total",
		Help: "Point-to-point analyses by result: ok or error.",
	}, []string{"result"})

	LinkDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "radiopath_link_analysis_duration_seconds",
		Help:    "Duration of a point-to-point analysis.",
		Buckets: prometheus.DefBuckets,
	})
)

var (
	DEMTiles = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "radiopath_dem_tile_cache_total",
		Help: "Tile cache lookups by kind (dem, canopy) and result (hit, miss).",
	}, []string{"kind", "result"})

	S3Requests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "radiopath_s3_requests_total",
		Help: "GETs against the tile bucket by result: ok, notfound or error.",
	}, []string{"result"})

	TileRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "radiopath_map_tiles_total",
		Help: "Map tiles served by result: hit, miss, limited, error or canceled.",
	}, []string{"result"})
)

var Logins = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "radiopath_logins_total",
	Help: "Login attempts by result.",
}, []string{"result"})

var Events = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "radiopath_events_total",
	Help: "Account, token and share events by name.",
}, []string{"event"})

const (
	EventRegistered     = "registered"
	EventEmailVerified  = "email_verified"
	EventPasswordReset  = "password_reset"
	EventTokenCreated   = "token_created"
	EventTokenRevoked   = "token_revoked"
	EventSharePublished = "share_published" // created or extended
	EventShareRevoked   = "share_revoked"
)

// JobKinds lists every job kind; keep in sync with store.KindCoverage and the kinds in internal/web/jobs.go.
var JobKinds = []string{"coverage", "link-analysis", "hf-prediction", "hf-map"}

// init creates every known label combination at zero, so dashboards see a series before the first event.
func init() {
	for _, k := range JobKinds {
		for _, r := range []string{"done", "failed", "requeued"} {
			Jobs.WithLabelValues(k, r)
		}
	}
	for _, r := range []string{"ok", "error"} {
		LinkAnalyses.WithLabelValues(r)
	}
	for _, k := range []string{"dem", "canopy"} {
		for _, r := range []string{"hit", "miss"} {
			DEMTiles.WithLabelValues(k, r)
		}
	}
	for _, r := range []string{"ok", "notfound", "error"} {
		S3Requests.WithLabelValues(r)
	}
	for _, r := range []string{"hit", "miss", "limited", "error", "canceled"} {
		TileRequests.WithLabelValues(r)
	}
	for _, r := range []string{"success", "failure", "rate_limited"} {
		Logins.WithLabelValues(r)
	}
	for _, e := range []string{EventRegistered, EventEmailVerified, EventPasswordReset, EventTokenCreated,
		EventTokenRevoked, EventSharePublished, EventShareRevoked} {
		Events.WithLabelValues(e)
	}
}
