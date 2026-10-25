package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	"github.com/radiopath/radiopath/internal/store"
)

// init makes empty lists [] instead of null in every response and schema.
func init() {
	huma.DefaultArrayNullable = false
}

// Scopes name what an operation needs: a session grants all of them, an API token only those it was created with.
// scopeSession marks operations no API token may call.
const (
	scopeAccountRead = "account:read"
	scopeSession     = "session"
)

// api builds the JSON API on its own mux; Handler mounts it under /api/ behind authenticate.
func (s *Server) api() *http.ServeMux {
	mux := http.NewServeMux()
	s.registerAPI(humago.New(mux, s.apiConfig()))
	return mux
}

// OpenAPI returns the API description as JSON without needing a database.
func (s *Server) OpenAPI() ([]byte, error) {
	api := humago.New(http.NewServeMux(), s.apiConfig())
	s.registerAPI(api)
	return api.OpenAPI().MarshalJSON()
}

const apiIntro = `Radio link and coverage planning: sites, antennas, point-to-point links with their path analysis and
area coverages. The web app is built on this API; other programs use it with a personal API token.

## Authentication

Create a token under **API tokens** in the web app and send it with every request:

    Authorization: Bearer rp_...

A token reaches the operations whose scope it was given (shown at each operation, for example ` + "`sites:read`" + `);
a write scope includes the read scope of the same resource. Account settings and token management need a login in
the web app. A token without the scope gets 403, a wrong, expired or revoked one 401.

## Errors

Errors are problem details (RFC 9457) with ` + "`status`" + ` and ` + "`detail`" + `; rejected input lists each field in
` + "`errors[]`" + ` with its ` + "`location`" + `, such as ` + "`body.lat`" + `.

## Limits

300 requests per token and minute, 60 path analyses, PDF reports and raster merges per user and minute; above that the
answer is 429. Rasters, PDF reports and job events live outside ` + "`/api`" + ` (` + "`/links/{id}/report.pdf`" + `,
` + "`/coverages/{id}/image.png`" + ` and so on) and take the same token.`

// apiConfig returns the huma configuration with the API paths and the session scheme.
func (s *Server) apiConfig() huma.Config {
	// no version: it would tell an attacker which known bugs apply
	c := huma.DefaultConfig("Radiopath API", "")
	c.OpenAPIPath = "/api/openapi"
	// no docs page: it loads its scripts from a CDN; the description itself is for logged-in users only (privateSpec)
	c.DocsPath = ""
	c.SchemasPath = "/api/schemas"
	c.CreateHooks = nil
	c.Info.Description = apiIntro
	c.OpenAPI.OnAddOperation = append(c.OpenAPI.OnAddOperation, describeAccess)
	c.Tags = []*huma.Tag{
		{Name: "Sites"}, {Name: "Antennas"}, {Name: "Links"}, {Name: "Coverages"},
		{Name: "HF", Description: "Ionospheric predictions (ITU-R P.533) between two sites, reliability maps and the sunspot numbers they use; nothing is stored."},
		{Name: "Shared pages", Description: "Read-only views of shared links and coverages; anyone with the share token may call them."},
		{Name: "Account", Description: "Login, registration and the account itself; changes need a session, not a token."},
		{Name: "API tokens", Description: "Managing personal API tokens needs a session; a token cannot create or list tokens."},
	}
	c.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		"session": {Type: "apiKey", In: "cookie", Name: sessionCookie, Description: "The session cookie set by POST /api/session."},
		"bearer": {Type: "http", Scheme: "bearer", Description: "A personal API token (`rp_...`), created under API tokens in the web app. " +
			"It may call the operations whose scope it was given; account settings and token management need a session."},
	}
	return c
}

// registerAPI registers the middleware and all API operations.
func (s *Server) registerAPI(api huma.API) {
	api.UseMiddleware(s.gate(api))
	s.registerConfig(api)
	s.registerAuth(api)
	s.registerMailFlows(api)
	s.registerSites(api)
	s.registerAntennas(api)
	s.registerLinks(api)
	s.registerCoverages(api)
	s.registerHF(api)
	s.registerHFMap(api)
	s.registerHFSunspot(api)
	s.registerAccount(api)
	s.registerTokens(api)
}

// op returns an operation; a non-empty scope requires a logged-in user or, unless it is scopeSession, an API token
// with that scope; an empty one makes it public.
func op(id, method, path, scope, summary string) huma.Operation {
	o := huma.Operation{OperationID: id, Method: method, Path: path, Summary: summary, Tags: []string{tagOf(path)}}
	if scope != "" {
		o.Security = []map[string][]string{{"session": {}}}
		if scope != scopeSession {
			// bearer first: it is what other programs use, so the reference shows it in the examples
			o.Security = []map[string][]string{{"bearer": {scope}}, {"session": {}}}
		}
		o.Errors = []int{http.StatusUnauthorized, http.StatusForbidden}
	}
	return o
}

// tagOf groups an operation in the API reference by the resource of its path.
func tagOf(path string) string {
	for _, t := range []struct{ prefix, tag string }{
		{"/api/sites", "Sites"}, {"/api/antennas", "Antennas"}, {"/api/links", "Links"}, {"/api/coverages", "Coverages"},
		{"/api/hf", "HF"}, {"/api/share/", "Shared pages"}, {"/api/me/tokens", "API tokens"},
	} {
		if strings.HasPrefix(path, t.prefix) {
			return t.tag
		}
	}
	return "Account"
}

// describeAccess appends to an operation's description who may call it, so the API reference shows the scope.
func describeAccess(_ *huma.OpenAPI, o *huma.Operation) {
	var who string
	switch sc := tokenScope(o); {
	case sc != "":
		who = "API token scope: `" + sc + "`."
	case len(o.Security) > 0:
		who = "Needs a session; API tokens cannot call it."
	default:
		return
	}
	if o.Description != "" {
		who = o.Description + "\n\n" + who
	}
	o.Description = who
}

// tokenScope returns the scope an API token needs for o, or "" when no token may call it.
func tokenScope(o *huma.Operation) string {
	for _, req := range o.Security {
		if s := req["bearer"]; len(s) > 0 {
			return s[0]
		}
	}
	return ""
}

// gate answers 401 for operations with security when nobody is logged in, and 403 when an API token may not call them.
func (s *Server) gate(api huma.API) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		o := ctx.Operation()
		if len(o.Security) == 0 {
			next(ctx)
			return
		}
		if userOf(ctx.Context()).ID == 0 {
			huma.WriteErr(api, ctx, http.StatusUnauthorized, "Not logged in")
			return
		}
		if msg := tokenDenied(ctx.Context(), tokenScope(o)); msg != "" {
			huma.WriteErr(api, ctx, http.StatusForbidden, msg)
			return
		}
		next(ctx)
	}
}

type TokenPath struct {
	Token string `path:"token"`
}

type requestInfo struct {
	ip     string
	secure bool
	cookie string
}

const reqKey ctxKey = 1

// privateSpec answers 401 for the API description (/api/openapi.*, /api/schemas/...) unless a user is logged in.
func privateSpec(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if (strings.HasPrefix(p, "/api/openapi") || strings.HasPrefix(p, "/api/schemas")) && userOf(r.Context()).ID == 0 {
			http.Error(w, "not logged in", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// authenticate puts the request info and the caller, if any, into the context. Without credentials it never
// rejects, gate does; a bad API token is answered here, as problem JSON.
func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info := requestInfo{ip: s.clientIP(r), secure: secureCookie(r)}
		if c, err := r.Cookie(sessionCookie); err == nil && r.Header.Get("Authorization") == "" {
			info.cookie = c.Value
		}
		ctx := context.WithValue(r.Context(), reqKey, info)
		u, tok, err := s.identify(r)
		var ae *authError
		switch {
		case err == nil:
			ctx = context.WithValue(ctx, userKey, u)
			if tok != nil {
				ctx = context.WithValue(ctx, tokenKey, tok)
			}
		case errors.As(err, &ae):
			ae.header(w)
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(ae.status)
			json.NewEncoder(w).Encode(huma.ErrorModel{Title: http.StatusText(ae.status), Status: ae.status, Detail: ae.msg})
			return
		case !errors.Is(err, store.ErrNotFound):
			s.Log.Error("session lookup", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// reqInfo returns what authenticate stored about the request.
func reqInfo(ctx context.Context) requestInfo {
	i, _ := ctx.Value(reqKey).(requestInfo)
	return i
}

// userOf returns the logged-in user of ctx, or the zero value.
func userOf(ctx context.Context) store.User {
	u, _ := ctx.Value(userKey).(store.User)
	return u
}

// check collects the invalid fields of an API input; err returns them as one 422.
type check struct{ errs []error }

// add records a problem with field; an empty field means the input as a whole.
func (c *check) add(field, msg string) {
	loc := "body"
	if field != "" {
		loc += "." + field
	}
	c.errs = append(c.errs, &huma.ErrorDetail{Location: loc, Message: msg})
}

// num checks that v lies within lo..hi and returns it.
func (c *check) num(field, label string, v, lo, hi float64) float64 {
	if math.IsNaN(v) || v < lo || v > hi {
		c.add(field, fmt.Sprintf("%s: must be between %g and %g", label, lo, hi))
	}
	return v
}

// optNum is num for an optional value; nil passes.
func (c *check) optNum(field, label string, v *float64, lo, hi float64) *float64 {
	if v != nil {
		c.num(field, label, *v, lo, hi)
	}
	return v
}

// deg checks an optional bearing in 0..360 and wraps 360 to 0; nil passes.
func (c *check) deg(field, label string, v *float64) *float64 {
	if v == nil {
		return nil
	}
	d := math.Mod(c.num(field, label, *v, 0, 360), 360)
	return &d
}

// text checks a required value of at most maxTextLen characters and returns it.
func (c *check) text(field, label, v string) string {
	if v == "" {
		c.add(field, label+": required")
	} else if utf8.RuneCountInString(v) > maxTextLen {
		c.add(field, fmt.Sprintf("%s: at most %d characters", label, maxTextLen))
	}
	return v
}

// err returns the collected problems as a 422, or nil.
func (c *check) err() error {
	if len(c.errs) == 0 {
		return nil
	}
	return huma.Error422UnprocessableEntity("Invalid input", c.errs...)
}

// storeErr maps store errors to problem responses and logs anything unexpected as a 500.
func (s *Server) storeErr(err error) error {
	switch {
	case errors.Is(err, store.ErrNotFound):
		return huma.Error404NotFound("Not found")
	default:
		s.Log.Error("request failed", "err", err)
		return huma.Error500InternalServerError("Internal error")
	}
}

type Config struct {
	Registration  bool `json:"registration" doc:"Whether new accounts can register"`
	PasswordReset bool `json:"password_reset" doc:"Whether forgotten passwords can be reset by mail"`
	Maintenance   bool `json:"maintenance" doc:"Logins are disabled while true"`
}

type ConfigOutput struct{ Body Config }

// registerConfig registers GET /api/config, what the login pages need to know before anyone is logged in.
func (s *Server) registerConfig(api huma.API) {
	huma.Register(api, op("get-config", http.MethodGet, "/api/config", "", "The features the login pages offer"),
		func(ctx context.Context, _ *struct{}) (*ConfigOutput, error) {
			on, err := s.Store.Maintenance(ctx)
			if err != nil {
				return nil, s.storeErr(err)
			}
			return &ConfigOutput{Body: Config{Registration: s.Registration, PasswordReset: s.Mail != nil, Maintenance: on}}, nil
		})
}
