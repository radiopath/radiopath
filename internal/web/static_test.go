package web

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStaticETag(t *testing.T) {
	h := (&Server{Log: slog.Default()}).Handler()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/static/app.css", nil))
	etag := rec.Header().Get("ETag")
	if rec.Code != 200 || etag == "" || rec.Header().Get("Cache-Control") != "no-cache" || rec.Body.Len() == 0 {
		t.Fatalf("code %d etag %q headers %v", rec.Code, etag, rec.Header())
	}

	req := httptest.NewRequest(http.MethodGet, "/static/app.css", nil)
	req.Header.Set("If-None-Match", etag)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 304 || rec.Body.Len() != 0 || rec.Header().Get("ETag") != etag {
		t.Errorf("conditional: code %d body %d", rec.Code, rec.Body.Len())
	}

	v := assetURL("/static/app.css")
	if v != "/static/app.css?v="+etag[1:len(etag)-1] {
		t.Errorf("assetURL: %q", v)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, v, nil))
	if rec.Code != 200 || rec.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" {
		t.Errorf("versioned: code %d cache-control %q", rec.Code, rec.Header().Get("Cache-Control"))
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/static/app.css?v=stale", nil))
	if rec.Header().Get("Cache-Control") != "no-cache" {
		t.Errorf("stale version: cache-control %q", rec.Header().Get("Cache-Control"))
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/static/nope.css", nil))
	if rec.Code != 404 {
		t.Errorf("unknown asset: code %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/static/leaflet/images/marker-icon.png", nil))
	if rec.Code != 200 || rec.Header().Get("ETag") == "" {
		t.Errorf("nested asset: code %d", rec.Code)
	}
}
