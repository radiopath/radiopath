package web

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

const testToken = "0123456789abcdef0123456789abcdef"

func flipAt(value string, i int) string {
	c := "A"
	if value[i] == 'A' {
		c = "B"
	}
	return value[:i] + c + value[i+1:]
}

func TestAdminCookie(t *testing.T) {
	now := time.Now()
	value := signAdmin(testToken, now.Add(adminTTL))

	if !validAdmin(testToken, value, now) {
		t.Fatal("fresh cookie rejected")
	}
	if validAdmin(testToken, value, now.Add(adminTTL+time.Second)) {
		t.Error("expired cookie accepted")
	}
	if validAdmin("another token that is long enough", value, now) {
		t.Error("cookie accepted under a different token")
	}

	bad := []string{
		"",
		"nodot",
		"notanumber.AAAA",
		value[:len(value)-1] + "x",
		flipAt(value, len(value)-20),
		strings.SplitN(value, ".", 2)[0] + ".",
		signAdmin(testToken, now.Add(adminTTL))[1:],
		strconv.FormatInt(now.Add(time.Hour).Unix(), 10) + ".AAAA",
	}
	for _, v := range bad {
		if validAdmin(testToken, v, now) {
			t.Errorf("accepted malformed cookie %q", v)
		}
	}

	far := signAdmin(testToken, now.Add(100*24*time.Hour))
	if validAdmin(testToken, far, now) {
		t.Error("accepted a cookie living longer than adminTTL")
	}
}

func TestTokenMatches(t *testing.T) {
	if !tokenMatches(testToken, testToken) {
		t.Error("identical tokens did not match")
	}
	for _, wrong := range []string{"", testToken + "x", testToken[:len(testToken)-1], strings.ToUpper(testToken)} {
		if tokenMatches(testToken, wrong) {
			t.Errorf("token %q matched", wrong)
		}
	}
}

func TestAdminDisabled(t *testing.T) {
	h := (&Server{Log: slog.Default()}).Handler()
	get := func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		return rec
	}
	admin, unknown := get("/admin"), get("/does-not-exist")
	loc := func(rec *httptest.ResponseRecorder) string {
		path, _, _ := strings.Cut(rec.Header().Get("Location"), "?")
		return path
	}
	if admin.Code != unknown.Code || loc(admin) != loc(unknown) {
		t.Errorf("/admin answered %d %q, unknown path answered %d %q", admin.Code,
			admin.Header().Get("Location"), unknown.Code, unknown.Header().Get("Location"))
	}
	if body := admin.Body.String(); strings.Contains(body, "Token") {
		t.Error("the login form was served without a configured token")
	}
}

func TestAdminRequiresCookie(t *testing.T) {
	h := (&Server{Log: slog.Default(), AdminToken: testToken}).Handler()
	for _, c := range []struct{ method, path string }{
		{http.MethodGet, "/admin"},
		{http.MethodGet, "/admin/users/hb9hil/password"},
		{http.MethodPost, "/admin/maintenance"},
		{http.MethodPost, "/admin/sessions/delete"},
		{http.MethodPost, "/admin/users"},
		{http.MethodPost, "/admin/users/hb9hil/password"},
		{http.MethodPost, "/admin/users/hb9hil/sessions"},
		{http.MethodPost, "/admin/users/hb9hil/delete"},
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(c.method, c.path, nil)
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		h.ServeHTTP(rec, req)
		if c.method == http.MethodGet && c.path == "/admin" {
			if rec.Code != http.StatusOK {
				t.Errorf("GET /admin = %d, want 200 with the login form", rec.Code)
			}
		} else if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s = %d, want 401", c.method, c.path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "/admin/login") {
			t.Errorf("%s %s did not render the login form", c.method, c.path)
		}
		if rec.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("%s %s: Cache-Control = %q", c.method, c.path, rec.Header().Get("Cache-Control"))
		}
	}
}

func TestAdminUnknownPath(t *testing.T) {
	h := (&Server{Log: slog.Default(), AdminToken: testToken}).Handler()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/nope", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("GET /admin/nope = %d, want 404", rec.Code)
	}
}
