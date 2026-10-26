package web

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/radiopath/radiopath/internal/metrics"
)

// instrument records request count by route, method, status and credential plus the duration for every request passing through next.
func instrument(root *http.ServeMux, subs map[string]*http.ServeMux, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route := routeLabel(root, subs, r)
		rec := &recorder{ResponseWriter: w, code: http.StatusOK}
		start := time.Now()
		next.ServeHTTP(rec, r)

		metrics.HTTPRequests.WithLabelValues(route, r.Method, strconv.Itoa(rec.code), authLabel(r)).Inc()
		metrics.HTTPDuration.WithLabelValues(route).Observe(time.Since(start).Seconds())
	})
}

// authLabel returns the credential r carries, valid or not: token for an Authorization header (which wins, as in
// identify), session for the session cookie, else none.
func authLabel(r *http.Request) string {
	if r.Header.Get("Authorization") != "" {
		return "token"
	}
	if _, err := r.Cookie(sessionCookie); err == nil {
		return "session"
	}
	return "none"
}

// routeLabel returns the matched route pattern without its method as the metric label, so IDs and tokens do not blow up cardinality; unmatched requests, also those of a sub-mux catch-all, give "other".
func routeLabel(root *http.ServeMux, subs map[string]*http.ServeMux, r *http.Request) string {
	_, pattern := root.Handler(r)
	if sub, ok := subs[pattern]; ok {
		// a sub-mux catch-all only answers 404
		if _, pattern = sub.Handler(r); pattern == "/" {
			pattern = ""
		}
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

// WriteHeader remembers the status code for the metrics before passing it on.
func (r *recorder) WriteHeader(code int) {
	r.code = code
	r.ResponseWriter.WriteHeader(code)
}

// Unwrap exposes the wrapped writer so http.ResponseController can reach Flush and friends.
func (r *recorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }
