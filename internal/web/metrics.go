package web

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/radiopath/radiopath/internal/metrics"
)

func instrument(root, app *http.ServeMux, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route := routeLabel(root, app, r)
		rec := &recorder{ResponseWriter: w, code: http.StatusOK}
		start := time.Now()
		next.ServeHTTP(rec, r)

		metrics.HTTPRequests.WithLabelValues(route, r.Method, strconv.Itoa(rec.code)).Inc()
		metrics.HTTPDuration.WithLabelValues(route).Observe(time.Since(start).Seconds())
	})
}

func routeLabel(root, app *http.ServeMux, r *http.Request) string {
	_, pattern := root.Handler(r)
	if pattern == "/" {
		_, pattern = app.Handler(r)
	}
	if _, path, ok := strings.Cut(pattern, " "); ok {
		return path
	}
	if pattern == "" {
		return "other"
	}
	return pattern
}

type recorder struct {
	http.ResponseWriter
	code int
}

func (r *recorder) WriteHeader(code int) {
	r.code = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *recorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }
