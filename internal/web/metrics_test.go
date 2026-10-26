package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRouteLabel(t *testing.T) {
	nop := func(http.ResponseWriter, *http.Request) {}
	app := http.NewServeMux()
	app.HandleFunc("GET /sites/{id}", nop)
	api := http.NewServeMux()
	api.HandleFunc("GET /api/password/reset/{token}", nop)
	root := http.NewServeMux()
	root.HandleFunc("GET /healthz", nop)
	root.Handle("/api/", api)
	root.HandleFunc("GET /tiles/{z}/{x}/{y}", nop)
	share := http.NewServeMux()
	share.HandleFunc("GET /s/l/{token}/report.pdf", nop)
	share.HandleFunc("/", nop)
	root.Handle("/s/", share)
	root.Handle("/", app)

	for _, tc := range []struct{ path, want string }{
		{"/healthz", "/healthz"},
		{"/api/password/reset/s3cret-token", "/api/password/reset/{token}"},
		{"/api/nothing", "other"},
		{"/sites/42", "/sites/{id}"},
		{"/tiles/12/2145/1436.png", "/tiles/{z}/{x}/{y}"},
		{"/s/l/s3cret-token/report.pdf", "/s/l/{token}/report.pdf"},
		{"/s/x/s3cret-token", "other"},
		{"/nothing/here", "other"},
	} {
		got := routeLabel(root, map[string]*http.ServeMux{"/": app, "/api/": api, "/s/": share}, httptest.NewRequest("GET", tc.path, nil))
		if got != tc.want {
			t.Errorf("routeLabel(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

func TestAuthLabel(t *testing.T) {
	for _, tc := range []struct {
		header, cookie bool
		want           string
	}{
		{false, false, "none"},
		{false, true, "session"},
		{true, false, "token"},
		{true, true, "token"},
	} {
		r := httptest.NewRequest("GET", "/api/me", nil)
		if tc.header {
			r.Header.Set("Authorization", "Bearer rp_x")
		}
		if tc.cookie {
			r.AddCookie(&http.Cookie{Name: sessionCookie, Value: "x"})
		}
		if got := authLabel(r); got != tc.want {
			t.Errorf("authLabel(header %v, cookie %v) = %q, want %q", tc.header, tc.cookie, got, tc.want)
		}
	}
}
