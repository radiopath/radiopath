package main

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/radiopath/radiopath/internal/store"
)

const statsTimeout = 5 * time.Second

var (
	usersDesc     = prometheus.NewDesc("radiopath_users", "User accounts.", nil, nil)
	sitesDesc     = prometheus.NewDesc("radiopath_sites", "Stored sites.", nil, nil)
	linksDesc     = prometheus.NewDesc("radiopath_links", "Stored links.", nil, nil)
	sessionsDesc  = prometheus.NewDesc("radiopath_sessions_active", "Sessions that have not expired.", nil, nil)
	coveragesDesc = prometheus.NewDesc("radiopath_coverages", "Coverages by job state.", []string{"job_state"}, nil)
)

type dbCollector struct {
	store *store.Store
	log   *slog.Logger
}

func (c dbCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- usersDesc
	ch <- sitesDesc
	ch <- linksDesc
	ch <- sessionsDesc
	ch <- coveragesDesc
}

func (c dbCollector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), statsTimeout)
	defer cancel()
	st, err := c.store.Stats(ctx)
	if err != nil {
		c.log.Warn("metrics: db stats", "err", err)
		return
	}
	g := func(d *prometheus.Desc, v int, labels ...string) {
		ch <- prometheus.MustNewConstMetric(d, prometheus.GaugeValue, float64(v), labels...)
	}
	g(usersDesc, st.Users)
	g(sitesDesc, st.Sites)
	g(linksDesc, st.Links)
	g(sessionsDesc, st.Sessions)
	for _, state := range []string{store.JobIdle, store.JobQueued, store.JobRunning, store.JobDone, store.JobFailed} {
		g(coveragesDesc, st.Coverages[state], state)
	}
}

func metricsServer(addr string, st *store.Store, log *slog.Logger) *http.Server {
	prometheus.MustRegister(dbCollector{store: st, log: log})
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.Handler())
	return &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
}
