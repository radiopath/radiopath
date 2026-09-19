package web

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/radiopath/radiopath/internal/link"
	"github.com/radiopath/radiopath/internal/mail"
	"github.com/radiopath/radiopath/internal/store"
	"github.com/radiopath/radiopath/internal/tiles"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

var pages = []string{"sites_list", "site_form", "links_list", "link_form", "link_detail",
	"coverages_list", "coverage_form", "coverage_detail", "antennas_list", "login",
	"register", "account", "forgot", "reset", "admin_login", "admin_users", "notfound"}

type base struct {
	User  string
	Admin bool
}

func (s *Server) base(r *http.Request) base { return base{User: currentUser(r).Name} }

type Server struct {
	Store        *store.Store
	Analyzer     link.Analyzer
	Tiles        *tiles.Proxy
	DEMReady     func() error
	Registration bool
	Mail         mail.Mailer
	BaseURL      string
	TrustProxy   bool
	AdminToken   string
	Version      string
	Log          *slog.Logger

	tmpl map[string]*template.Template
}

func (s *Server) Handler() http.Handler {
	funcs := template.FuncMap{
		"divKm":        func(m float64) float64 { return m / 1000 },
		"offBoresight": link.OffBoresight,
		"asset":        assetURL,
		"isID":         func(id int64, ref *int64) bool { return ref != nil && *ref == id },
		"version":      func() string { return s.Version },
	}
	s.tmpl = make(map[string]*template.Template, len(pages))
	for _, p := range pages {
		s.tmpl[p] = template.Must(template.New("layout.html").Funcs(funcs).
			ParseFS(templateFS, "templates/layout.html", "templates/icons.html", "templates/partials.html",
				"templates/"+p+".html"))
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		n, err := s.Store.CountSites(r.Context(), currentUser(r).ID)
		if err != nil {
			s.fail(w, err)
			return
		}
		if n == 0 {
			http.Redirect(w, r, "/sites", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/links", http.StatusSeeOther)
	})
	mux.HandleFunc("POST /logout", s.logout)

	mux.HandleFunc("GET /sites", s.listSites)
	mux.HandleFunc("GET /sites/new", s.newSite)
	mux.HandleFunc("POST /sites", s.createSite)
	mux.HandleFunc("GET /sites/{id}", s.editSite)
	mux.HandleFunc("POST /sites/{id}", s.updateSite)
	mux.HandleFunc("POST /sites/{id}/delete", s.deleteSite)

	mux.HandleFunc("GET /antennas", s.listAntennas)
	mux.HandleFunc("POST /antennas", s.createAntenna)
	mux.HandleFunc("POST /antennas/manual", s.createManualAntenna)
	mux.HandleFunc("POST /antennas/{id}", s.updateAntenna)
	mux.HandleFunc("POST /antennas/{id}/delete", s.deleteAntenna)

	mux.HandleFunc("GET /links", s.listLinks)
	mux.HandleFunc("GET /links/new", s.newLink)
	mux.HandleFunc("POST /links", s.createLink)
	mux.HandleFunc("GET /links/{id}", s.showLink)
	mux.HandleFunc("GET /links/{id}/edit", s.editLink)
	mux.HandleFunc("GET /links/{id}/report.pdf", s.linkReport)
	mux.HandleFunc("POST /links/{id}", s.updateLink)
	mux.HandleFunc("POST /links/{id}/delete", s.deleteLink)
	mux.HandleFunc("POST /links/{id}/share", s.shareLink)
	mux.HandleFunc("POST /links/{id}/share/delete", s.unshareLink)

	mux.HandleFunc("GET /account", s.account)
	mux.HandleFunc("POST /account/email", s.accountEmail)
	mux.HandleFunc("POST /account/password", s.accountPassword)

	mux.HandleFunc("GET /coverages", s.listCoverages)
	mux.HandleFunc("GET /coverages/new", s.newCoverage)
	mux.HandleFunc("POST /coverages", s.createCoverage)
	mux.HandleFunc("GET /coverages/{id}", s.showCoverage)
	mux.HandleFunc("GET /coverages/{id}/edit", s.editCoverage)
	mux.HandleFunc("GET /coverages/{id}/report.pdf", s.coverageReport)
	mux.HandleFunc("POST /coverages/{id}", s.updateCoverage)
	mux.HandleFunc("POST /coverages/{id}/delete", s.deleteCoverage)
	mux.HandleFunc("POST /coverages/{id}/compute", s.computeCoverage)
	mux.HandleFunc("GET /coverages/{id}/image.png", s.coverageImage)
	mux.HandleFunc("GET /coverages/{id}/composite.png", s.coverageComposite)
	mux.HandleFunc("GET /coverages/{id}/events", s.coverageEvents)
	mux.HandleFunc("POST /coverages/{id}/view", s.setCoverageView)
	mux.HandleFunc("POST /coverages/{id}/legend", s.setCoverageLegend)
	mux.HandleFunc("POST /coverages/{id}/share", s.shareCoverage)
	mux.HandleFunc("POST /coverages/{id}/share/delete", s.unshareCoverage)

	root := http.NewServeMux()
	if s.adminEnabled() {
		admin := http.NewServeMux()
		admin.HandleFunc("GET /admin/{$}", s.adminIndex)
		admin.HandleFunc("POST /admin/login", s.adminLogin)
		admin.HandleFunc("POST /admin/logout", s.adminLogout)
		admin.Handle("GET /admin/users/{name}/password", s.requireAdmin(http.HandlerFunc(s.adminPasswordForm)))
		admin.Handle("POST /admin/maintenance", s.requireAdmin(http.HandlerFunc(s.adminMaintenance)))
		admin.Handle("POST /admin/sessions/delete", s.requireAdmin(http.HandlerFunc(s.adminRevokeAllSessions)))
		admin.Handle("POST /admin/users", s.requireAdmin(http.HandlerFunc(s.adminCreateUser)))
		admin.Handle("POST /admin/users/{name}/password", s.requireAdmin(http.HandlerFunc(s.adminSetPassword)))
		admin.Handle("POST /admin/users/{name}/sessions", s.requireAdmin(http.HandlerFunc(s.adminRevokeSessions)))
		admin.Handle("POST /admin/users/{name}/delete", s.requireAdmin(http.HandlerFunc(s.adminDeleteUser)))
		admin.HandleFunc("/", s.notFound)
		root.HandleFunc("GET /admin", s.adminIndex)
		root.Handle("/admin/", admin)
	}
	share := http.NewServeMux()
	share.HandleFunc("GET /s/l/{token}", s.sharedLink)
	share.HandleFunc("GET /s/l/{token}/report.pdf", s.sharedLinkReport)
	share.HandleFunc("GET /s/c/{token}", s.sharedCoverage)
	share.HandleFunc("GET /s/c/{token}/report.pdf", s.sharedCoverageReport)
	share.HandleFunc("GET /s/c/{token}/image.png", s.sharedCoverageImage)
	share.HandleFunc("GET /s/c/{token}/composite.png", s.sharedCoverageComposite)
	share.HandleFunc("/", s.notFound)
	root.Handle("/s/", s.shareLimit(share))
	root.Handle("GET /tiles/{z}/{x}/{y}", s.Tiles)
	root.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok\n")) })
	root.HandleFunc("GET /readyz", s.readyz)
	root.Handle("GET /static/", staticHandler())
	root.HandleFunc("GET /login", s.loginForm)
	root.HandleFunc("POST /login", s.login)
	if s.Registration {
		root.HandleFunc("GET /register", s.registerForm)
		root.HandleFunc("POST /register", s.register)
	}
	if s.Mail != nil {
		root.HandleFunc("GET /forgot", s.forgotForm)
		root.HandleFunc("POST /forgot", s.forgot)
		root.HandleFunc("GET /reset/{token}", s.resetForm)
		root.HandleFunc("POST /reset/{token}", s.reset)
		root.HandleFunc("GET /verify/{token}", s.verify)
		root.HandleFunc("POST /verify/resend", s.resendVerification)
	}
	root.Handle("/", s.requireAuth(mux))

	return secureHeaders(instrument(root, mux, http.NewCrossOriginProtection().Handler(root)))
}

func secureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2e9)
	defer cancel()
	if err := s.Store.Ping(ctx); err != nil {
		http.Error(w, "database: "+err.Error(), http.StatusServiceUnavailable)
		return
	}
	if err := s.DEMReady(); err != nil {
		http.Error(w, "dem: "+err.Error(), http.StatusServiceUnavailable)
		return
	}
	if err := s.Tiles.Ready(ctx); err != nil {
		http.Error(w, "tile cache: "+err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.Write([]byte("ok\n"))
}

var staticETags = func() map[string]string {
	etags := map[string]string{}
	fs.WalkDir(staticFS, "static", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := staticFS.ReadFile(path)
		if err != nil {
			return err
		}
		etags["/"+path] = tiles.ETag(b)
		return nil
	})
	return etags
}()

func assetURL(path string) string {
	etag, ok := staticETags[path]
	if !ok {
		return path
	}
	return path + "?v=" + strings.Trim(etag, `"`)
}

func staticHandler() http.Handler {
	files := http.StripPrefix("/", http.FileServerFS(staticFS))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		etag, ok := staticETags[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("v") == strings.Trim(etag, `"`) {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		w.Header().Set("ETag", etag)
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		files.ServeHTTP(w, r)
	})
}

func (s *Server) render(w http.ResponseWriter, status int, page string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := s.tmpl[page].Execute(w, data); err != nil {
		s.Log.Error("render", "page", page, "err", err)
	}
}

func (s *Server) notFound(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	s.render(w, http.StatusNotFound, "notfound", base{})
}

func (s *Server) fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		s.notFound(w, nil)
	default:
		s.Log.Error("request failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func escapeQuery(v string) string { return url.QueryEscape(v) }

func pathID(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("id"), 10, 64)
}

func fromID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.URL.Query().Get("from"), 10, 64)
	return id, err == nil && id > 0
}

const maxTextLen = 100

type form struct {
	r      *http.Request
	Errors []string
}

func (f *form) get(name string) string { return f.r.PostFormValue(name) }

func (f *form) num(name, label string, lo, hi float64) float64 {
	v, err := strconv.ParseFloat(f.get(name), 64)
	if err != nil {
		f.Errors = append(f.Errors, label+": not a number")
		return 0
	}
	if v < lo || v > hi {
		f.Errors = append(f.Errors, fmt.Sprintf("%s: must be between %g and %g", label, lo, hi))
	}
	return v
}

func (f *form) optNum(name, label string, lo, hi float64) float64 {
	if f.get(name) == "" {
		return 0
	}
	return f.num(name, label, lo, hi)
}

func (f *form) text(name, label string) string {
	v := f.get(name)
	if v == "" {
		f.Errors = append(f.Errors, label+": required")
	} else if utf8.RuneCountInString(v) > maxTextLen {
		f.Errors = append(f.Errors, fmt.Sprintf("%s: at most %d characters", label, maxTextLen))
	}
	return v
}

func (f *form) optID(name, label string) *int64 {
	v := f.get(name)
	if v == "" {
		return nil
	}
	id, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		f.Errors = append(f.Errors, label+": invalid")
		return nil
	}
	return &id
}

func (f *form) optNumP(name, label string, lo, hi float64) *float64 {
	if f.get(name) == "" {
		return nil
	}
	v := f.num(name, label, lo, hi)
	return &v
}

func (f *form) optDeg(name, label string) *float64 {
	if f.get(name) == "" {
		return nil
	}
	v := math.Mod(f.num(name, label, 0, 360), 360)
	return &v
}

func (f *form) id(name, label string) int64 {
	v, err := strconv.ParseInt(f.get(name), 10, 64)
	if err != nil {
		f.Errors = append(f.Errors, label+": invalid")
	}
	return v
}

func (f *form) values() map[string]string {
	out := make(map[string]string, len(f.r.PostForm))
	for k := range f.r.PostForm {
		out[k] = f.r.PostFormValue(k)
	}
	return out
}
