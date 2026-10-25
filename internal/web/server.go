package web

import (
	"bytes"
	"context"
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/radiopath/radiopath/internal/hf"
	"github.com/radiopath/radiopath/internal/link"
	"github.com/radiopath/radiopath/internal/mail"
	"github.com/radiopath/radiopath/internal/store"
	"github.com/radiopath/radiopath/internal/sunspot"
	"github.com/radiopath/radiopath/internal/tiles"
)

//go:embed templates/*.html
var templateFS embed.FS

// The admin panel is the only server-rendered HTML; everything else is the SPA.
var pages = []string{"admin_login", "admin_users"}

type base struct {
	Admin bool
}

type Server struct {
	Store        *store.Store
	Analyzer     link.Analyzer
	HF           *hf.Data        // nil without the HF data set (RADIOPATH_DATASET_HF); the HF operations answer 503
	Sunspots     *sunspot.Source // nil without HF data or with RADIOPATH_SUNSPOT_URL empty; the lookup answers 404
	Tiles        *tiles.Proxy
	DEMReady     func() error
	Registration bool
	Mail         mail.Mailer
	BaseURL      string
	TrustProxy   bool
	AdminToken   string
	Version      string // shown to logged-in users only (/api/me)
	Log          *slog.Logger
	// UI holds the built SPA (webui.FS()), served for every path no route claims.
	UI fs.FS

	tmpl     map[string]*template.Template
	adminCSP string

	streamsMu sync.Mutex
	streams   map[int64]int // open job event streams per user
	waits     jobWaits      // requests waiting for a job, woken by JobDone
}

// Handler parses the admin templates and builds the route tree: health, tiles, the JSON API, the binary routes of
// rasters, reports and job events, the optional admin panel, and the SPA for every other path.
func (s *Server) Handler() http.Handler {
	s.tmpl = make(map[string]*template.Template, len(pages))
	for _, p := range pages {
		s.tmpl[p] = template.Must(template.ParseFS(templateFS, "templates/admin_layout.html", "templates/"+p+".html"))
	}
	// hash the rendered page, not the template: html/template drops comments inside scripts
	var login bytes.Buffer
	if err := s.tmpl["admin_login"].Execute(&login, adminLoginData{base: base{Admin: true}}); err != nil {
		panic(err)
	}
	s.adminCSP = csp(login.Bytes(), "", "default-src 'none'; style-src 'unsafe-inline'; img-src 'self'; font-src 'self'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'")

	if s.Tiles != nil {
		s.Tiles.ClientIP = s.clientIP
	}

	root := http.NewServeMux()
	links := func(h http.HandlerFunc) http.Handler { return s.requireUser(scopeLinksRead, h) }
	coverages := func(h http.HandlerFunc) http.Handler { return s.requireUser(scopeCoveragesRead, h) }
	root.Handle("GET /links/{id}/report.pdf", links(s.computeLimited(s.linkReport)))
	root.Handle("GET /coverages/{id}/report.pdf", coverages(s.computeLimited(s.coverageReport)))
	root.Handle("GET /coverages/{id}/image.png", coverages(s.coverageImage))
	root.Handle("GET /coverages/{id}/composite.png", coverages(s.computeLimited(s.coverageComposite)))
	root.Handle("GET /coverages/{id}/events", coverages(s.coverageEvents))

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
	spa := s.spa()
	// Shared pages are SPA routes; the API request behind each one counts against the token's limit.
	sharedPage := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Robots-Tag", "noindex")
		spa.ServeHTTP(w, r)
	})
	root.Handle("GET /s/l/{token}", sharedPage)
	root.Handle("GET /s/c/{token}", sharedPage)
	share := http.NewServeMux()
	share.HandleFunc("GET /s/l/{token}/report.pdf", s.sharedLinkReport)
	share.HandleFunc("GET /s/c/{token}/report.pdf", s.sharedCoverageReport)
	share.HandleFunc("GET /s/c/{token}/image.png", s.sharedCoverageImage)
	share.HandleFunc("GET /s/c/{token}/composite.png", s.sharedCoverageComposite)
	share.HandleFunc("/", s.notFound)
	root.Handle("/s/", s.shareLimit(share))
	root.Handle("GET /tiles/{z}/{x}/{y}", s.Tiles)
	root.HandleFunc("GET /robots.txt", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte("User-agent: *\nDisallow: /tiles/\n"))
	})
	root.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok\n")) })
	root.HandleFunc("GET /readyz", s.readyz)
	api := s.api()
	root.Handle("/api/", s.authenticate(privateSpec(api)))
	root.Handle("/", spa)

	return secureHeaders(instrument(root, map[string]*http.ServeMux{"/api/": api}, http.NewCrossOriginProtection().Handler(root)))
}

// secureHeaders adds the nosniff, frame and referrer headers to every response.
func secureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

var inlineScript = regexp.MustCompile(`(?s)<script>(.*?)</script>`)

// csp returns a Content-Security-Policy whose script-src allows sources plus, by hash, exactly the inline scripts of
// page, followed by the other directives in rest. Pages only carry fixed scripts, so the hashes hold for every response.
func csp(page []byte, sources, rest string) string {
	src := "script-src"
	if sources != "" {
		src += " " + sources
	}
	for _, m := range inlineScript.FindAllSubmatch(page, -1) {
		sum := sha256.Sum256(m[1])
		src += " 'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
	}
	return src + "; " + rest
}

// spa serves the SvelteKit build: existing files as they are, any other path without a file extension the index.html fallback.
// Unknown API paths, _app assets and files answer 404, so a stale bundle fails loudly instead of loading HTML.
func (s *Server) spa() http.Handler {
	files := http.FileServerFS(s.UI)
	var index []byte
	if s.UI != nil { // tests of other routes build the handler without a UI
		index, _ = fs.ReadFile(s.UI, "index.html")
	}
	// Leaflet and Svelte transitions set style attributes; rasters arrive as blob: URLs, tiles through our own proxy.
	policy := csp(index, "'self'", "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; "+
		"connect-src 'self'; font-src 'self'; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") || (r.Method != http.MethodGet && r.Method != http.MethodHead) {
			http.NotFound(w, r)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name == "" {
			name = "index.html"
		}
		if _, err := fs.Stat(s.UI, name); err != nil {
			if strings.HasPrefix(name, "_app/") || strings.Contains(path.Base(name), ".") {
				http.NotFound(w, r)
				return
			}
			r = r.Clone(r.Context())
			r.URL.Path = "/"
		}
		w.Header().Set("Content-Security-Policy", policy)
		if strings.HasPrefix(name, "_app/immutable/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		files.ServeHTTP(w, r)
	})
}

const (
	computeWindow  = time.Minute
	computeUserMax = 60
)

// computeHit counts one expensive request of user (path analysis, report, raster merge) and answers 429 over
// computeUserMax per minute, so a single account cannot keep the CPUs busy.
func (s *Server) computeHit(ctx context.Context, user int64) error {
	n, err := s.Store.RateHit(ctx, fmt.Sprintf("compute:user:%d", user), computeWindow)
	if err != nil {
		return s.storeErr(err)
	}
	if n > computeUserMax {
		s.Log.Warn("compute rate limited", "user", user)
		return huma.Error429TooManyRequests("Too many calculations, try again in a minute.")
	}
	return nil
}

// computeLimited puts computeHit in front of a binary route of the logged-in user.
func (s *Server) computeLimited(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := s.computeHit(r.Context(), currentUser(r).ID); err != nil {
			var se huma.StatusError
			if errors.As(err, &se) && se.GetStatus() == http.StatusTooManyRequests {
				http.Error(w, "too many requests", http.StatusTooManyRequests)
				return
			}
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		next(w, r)
	}
}

// readyz answers GET /readyz with 200 only if the database, the DEM data and the tile cache are usable, otherwise 503.
// The failing part and its error go to the log only: the endpoint is public and errors name hosts and buckets.
func (s *Server) readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2e9)
	defer cancel()
	type part struct {
		name string
		err  func() error
	}
	parts := []part{
		{"database", func() error { return s.Store.Ping(ctx) }},
		{"dem", s.DEMReady},
	}
	if s.HF != nil {
		parts = append(parts, part{"hf data", s.HF.Ready})
	}
	parts = append(parts, part{"tile cache", func() error { return s.Tiles.Ready(ctx) }})
	for _, c := range parts {
		if err := c.err(); err != nil {
			s.Log.Warn("not ready", "part", c.name, "err", err)
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
	}
	w.Write([]byte("ok\n"))
}

// render executes the named page template with the given status.
func (s *Server) render(w http.ResponseWriter, status int, page string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", s.adminCSP)
	w.WriteHeader(status)
	if err := s.tmpl[page].Execute(w, data); err != nil {
		s.Log.Error("render", "page", page, "err", err)
	}
}

// notFound answers 404 in plain text; the pages people see are the SPA's.
func (s *Server) notFound(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	http.Error(w, "404 page not found", http.StatusNotFound)
}

// fail answers 404 for store.ErrNotFound and logs anything else, answering 500 without details.
func (s *Server) fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		s.notFound(w, nil)
	default:
		s.Log.Error("request failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// pathID parses the {id} path value.
func pathID(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("id"), 10, 64)
}

const maxTextLen = 100
