package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HTTPRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "radiopath_http_requests_total",
		Help: "HTTP requests by route pattern, method and status code.",
	}, []string{"route", "method", "code"})

	HTTPDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "radiopath_http_request_duration_seconds",
		Help:    "HTTP request duration by route pattern.",
		Buckets: []float64{0.005, 0.025, 0.1, 0.25, 1, 2.5, 10},
	}, []string{"route"})
)

var (
	CoverageJobs = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "radiopath_coverage_jobs_total",
		Help: "Finished coverage jobs by result: done, failed or requeued.",
	}, []string{"result"})

	CoverageJobDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "radiopath_coverage_job_duration_seconds",
		Help:    "Compute time of a successful coverage job.",
		Buckets: []float64{1, 2.5, 5, 10, 30, 60, 120, 300, 600},
	})
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
