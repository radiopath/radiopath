package web

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestSafeNext(t *testing.T) {
	for in, want := range map[string]string{
		"":                 "/",
		"/links/3":         "/links/3",
		"//evil.example":   "/",
		`/\evil.example`:   "/",
		"https://evil.com": "/",
		"links":            "/",
	} {
		if got := safeNext(in); got != want {
			t.Errorf("safeNext(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValidateCredentialsLength(t *testing.T) {
	if errs := ValidateCredentials("hb9hil", strings.Repeat("x", 72)); len(errs) != 0 {
		t.Errorf("72 bytes rejected: %v", errs)
	}
	if errs := ValidateCredentials("hb9hil", strings.Repeat("x", 73)); len(errs) != 1 {
		t.Errorf("73 bytes: want one error, got %v", errs)
	}
}

func TestFormTextLength(t *testing.T) {
	long := strings.Repeat("ä", maxTextLen+1)
	for _, tc := range []struct {
		value string
		errs  int
	}{{"HB9HIL", 0}, {strings.Repeat("ä", maxTextLen), 0}, {long, 1}, {"", 1}} {
		r := httptest.NewRequest("POST", "/sites", strings.NewReader(url.Values{"name": {tc.value}}.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		f := &form{r: r}
		f.text("name", "Name")
		if len(f.Errors) != tc.errs {
			t.Errorf("text(%d runes): errors %v, want %d", len([]rune(tc.value)), f.Errors, tc.errs)
		}
	}
}

func TestSecureHeaders(t *testing.T) {
	h := (&Server{Log: slog.Default()}).Handler()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/healthz", nil))
	for k, want := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "same-origin",
	} {
		if got := w.Header().Get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
	if w.Code != http.StatusOK {
		t.Errorf("healthz: %d", w.Code)
	}
}

func TestValidEmail(t *testing.T) {
	for in, want := range map[string]string{
		"op@hb9hil.example":                      "op@hb9hil.example",
		"  op@hb9hil.example ":                   "op@hb9hil.example",
		"Op <op@hb9hil.example>":                 "",
		"op@hb9hil.example, x@y.z":               "",
		"not-an-address":                         "",
		"":                                       "",
		"op@" + strings.Repeat("a", 250) + ".ch": "",
	} {
		if got := validEmail(in); got != want {
			t.Errorf("validEmail(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMailRoutesNeedMailer(t *testing.T) {
	h := (&Server{Log: slog.Default()}).Handler()
	for _, path := range []string{"/forgot", "/reset/abc", "/verify/abc"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != http.StatusSeeOther || !strings.HasPrefix(w.Header().Get("Location"), "/login") {
			t.Errorf("%s without mailer: %d %s", path, w.Code, w.Header().Get("Location"))
		}
	}
}
