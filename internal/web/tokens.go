package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/radiopath/radiopath/internal/metrics"
	"github.com/radiopath/radiopath/internal/store"
)

const (
	tokenPrefix = "rp_"
	tokenWindow = 15 * time.Minute
	tokenIPMax  = 20  // bad tokens per IP and window
	tokenMax    = 300 // requests per token and minute
)

// tokenScopes are the scopes a token can be given, in the order the web app and the mails list them.
var tokenScopes = []string{
	scopeSitesRead, scopeSitesWrite, scopeAntennasRead, scopeAntennasWrite, scopeLinksRead, scopeLinksWrite,
	scopeCoveragesRead, scopeCoveragesWrite, scopeHFRead, scopeAccountRead,
}

var tokenRe = regexp.MustCompile(`^rp_[A-Za-z0-9_-]{43}$`)

const tokenCreatedMail = `Hi %s,

a new API token was created for your Radiopath account:

Name:    %s
Scopes:  %s
Expires: %s

If that was not you, revoke it at

%s

and change your password.
`

// tokenAuth is what authenticate stores when a request carries an API token instead of a session cookie.
type tokenAuth struct {
	id     int64
	scopes []string
}

const tokenKey ctxKey = 2

// authError is a rejected API token: the status and the message for the client.
type authError struct {
	status int
	msg    string
}

// Error returns the message for the client.
func (e *authError) Error() string { return e.msg }

// header sets the challenge a 401 must carry.
func (e *authError) header(w http.ResponseWriter) {
	if e.status == http.StatusUnauthorized {
		w.Header().Set("WWW-Authenticate", `Bearer realm="radiopath"`)
	}
}

// identify returns the caller of r: the owner of the Bearer token in the Authorization header, or else the session
// user. A request with that header never falls back to the cookie. ErrNotFound means nobody; an *authError a token
// that is refused (bad, rate limited, maintenance); any other error comes from the store.
func (s *Server) identify(r *http.Request) (store.User, *tokenAuth, error) {
	h := r.Header.Get("Authorization")
	if h == "" {
		u, err := s.sessionUser(r)
		return u, nil, err
	}
	ctx := r.Context()
	failKey := "token:ip:" + s.clientIP(r)
	n, err := s.Store.RateCount(ctx, failKey, tokenWindow)
	if err != nil {
		return store.User{}, nil, err
	}
	if n >= tokenIPMax {
		return store.User{}, nil, &authError{http.StatusTooManyRequests, tooMany(tokenWindow)}
	}
	var u store.User
	var t tokenAuth
	err = store.ErrNotFound
	if secret, ok := strings.CutPrefix(h, "Bearer "); ok && tokenRe.MatchString(secret) {
		u, t.id, t.scopes, err = s.Store.TokenUser(ctx, sessionID(secret))
	}
	if errors.Is(err, store.ErrNotFound) {
		if _, err := s.Store.RateHit(ctx, failKey, tokenWindow); err != nil {
			return store.User{}, nil, err
		}
		return store.User{}, nil, &authError{http.StatusUnauthorized, "Invalid, expired or revoked API token."}
	}
	if err != nil {
		return store.User{}, nil, err
	}
	// a token logs in with every request, so maintenance mode stops it like a login
	on, err := s.Store.Maintenance(ctx)
	if err != nil {
		return store.User{}, nil, err
	}
	if on {
		return store.User{}, nil, &authError{http.StatusServiceUnavailable, maintenanceText}
	}
	n, err = s.Store.RateHit(ctx, fmt.Sprintf("token:%d", t.id), time.Minute)
	if err != nil {
		return store.User{}, nil, err
	}
	if n > tokenMax {
		return store.User{}, nil, &authError{http.StatusTooManyRequests, "Too many requests with this token, try again in a minute."}
	}
	return u, &t, nil
}

// tokenDenied returns why the API token in ctx may not do what needs scope ("" = sessions only), or "" when it may or
// the caller has a session.
func tokenDenied(ctx context.Context, scope string) string {
	t, ok := ctx.Value(tokenKey).(*tokenAuth)
	switch {
	case !ok:
		return ""
	case scope == "":
		return "API tokens cannot do this; log in to the web app."
	case !slices.Contains(t.scopes, scope):
		return "This token lacks the scope " + scope + "."
	}
	return ""
}

type Token struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix" doc:"The start of the token, to tell tokens apart"`
	Scopes     []string   `json:"scopes"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty" doc:"Missing for a token that never expires"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty" doc:"Missing until the token is used; updated at most once a minute"`
	Expired    bool       `json:"expired" doc:"Expired tokens stay listed for 30 days, then they are deleted"`
}

type CreatedToken struct {
	Token
	Secret string `json:"token" doc:"The token itself; it is shown only in this answer"`
}

type NewTokenInput struct {
	Name      string     `json:"name"`
	Scopes    []string   `json:"scopes" doc:"A write scope includes the read scope of the same resource" enum:"sites:read,sites:write,antennas:read,antennas:write,links:read,links:write,coverages:read,coverages:write,hf:read,account:read"`
	ExpiresAt *time.Time `json:"expires_at,omitempty" doc:"Omit for a token that never expires"`
	Password  string     `json:"password" doc:"The password in use, so a stolen session cannot create a lasting token"`
}

// toToken returns the API view of a stored token.
func toToken(t store.Token) Token {
	return Token{ID: t.ID, Name: t.Name, Prefix: t.Prefix, Scopes: t.Scopes, CreatedAt: t.CreatedAt, ExpiresAt: t.ExpiresAt,
		LastUsedAt: t.LastUsedAt, Expired: t.ExpiresAt != nil && !t.ExpiresAt.After(time.Now())}
}

// parseScopes checks the requested scopes and returns them in tokenScopes order, each write scope with its read scope.
func parseScopes(c *check, in []string) []string {
	want := map[string]bool{}
	for _, sc := range in {
		if !slices.Contains(tokenScopes, sc) {
			c.add("scopes", fmt.Sprintf("Scopes: unknown scope %q", sc))
			continue
		}
		want[sc] = true
		if res, ok := strings.CutSuffix(sc, ":write"); ok {
			want[res+":read"] = true
		}
	}
	if len(in) == 0 {
		c.add("scopes", "Scopes: choose at least one")
	}
	out := make([]string, 0, len(want))
	for _, sc := range tokenScopes {
		if want[sc] {
			out = append(out, sc)
		}
	}
	return out
}

// registerTokens registers the management of the user's API tokens; it needs a session, a token cannot mint tokens.
func (s *Server) registerTokens(api huma.API) {
	huma.Register(api, op("list-my-tokens", http.MethodGet, "/api/me/tokens", scopeSession, "The user's API tokens, newest first"),
		func(ctx context.Context, _ *struct{}) (*struct{ Body []Token }, error) {
			list, err := s.Store.ListTokens(ctx, userOf(ctx).ID)
			if err != nil {
				return nil, s.storeErr(err)
			}
			out := make([]Token, 0, len(list))
			for _, t := range list {
				out = append(out, toToken(t))
			}
			return &struct{ Body []Token }{out}, nil
		})

	o := op("create-my-token", http.MethodPost, "/api/me/tokens", scopeSession, "Create an API token")
	o.DefaultStatus = http.StatusCreated
	o.Description = "The answer carries the token itself, the only time it is shown. The confirmed address of the account gets a notice."
	huma.Register(api, o, func(ctx context.Context, in *struct{ Body NewTokenInput }) (*struct{ Body CreatedToken }, error) {
		u := userOf(ctx)
		var c check
		name := c.text("name", "Name", strings.TrimSpace(in.Body.Name))
		scopes := parseScopes(&c, in.Body.Scopes)
		if e := in.Body.ExpiresAt; e != nil && !e.After(time.Now()) {
			c.add("expires_at", "Expires: must be in the future")
		}
		if err := c.err(); err != nil {
			return nil, err
		}
		if err := s.checkPassword(ctx, u, in.Body.Password, "body.password"); err != nil {
			return nil, err
		}
		secret := tokenPrefix + randomToken()
		t, err := s.Store.CreateToken(ctx, u.ID, name, secret[:len(tokenPrefix)+8], sessionID(secret), scopes, in.Body.ExpiresAt)
		if errors.Is(err, store.ErrQuota) {
			return nil, huma.Error422UnprocessableEntity(fmt.Sprintf("Limit of %d tokens reached; revoke one first", store.MaxTokensPerUser))
		}
		if err != nil {
			return nil, s.storeErr(err)
		}
		s.Log.Info("API token created", "user", u.Name, "token", t.ID, "scopes", strings.Join(scopes, " "))
		metrics.Events.WithLabelValues(metrics.EventTokenCreated).Inc()
		if s.Mail != nil && u.Email != "" {
			expires := "never"
			if t.ExpiresAt != nil {
				expires = t.ExpiresAt.UTC().Format("2 Jan 2006 15:04 UTC")
			}
			s.mailLater(u.Email, "New Radiopath API token", fmt.Sprintf(tokenCreatedMail, u.Name, name, strings.Join(scopes, ", "), expires, s.BaseURL+"/account/tokens"))
		}
		return &struct{ Body CreatedToken }{CreatedToken{Token: toToken(t), Secret: secret}}, nil
	})

	o = op("delete-my-token", http.MethodDelete, "/api/me/tokens/{id}", scopeSession, "Revoke an API token")
	o.DefaultStatus = http.StatusNoContent
	huma.Register(api, o, func(ctx context.Context, in *IDPath) (*struct{}, error) {
		u := userOf(ctx)
		if err := s.Store.DeleteToken(ctx, u.ID, in.ID); err != nil {
			return nil, s.storeErr(err)
		}
		s.Log.Info("API token revoked", "user", u.Name, "token", in.ID)
		metrics.Events.WithLabelValues(metrics.EventTokenRevoked).Inc()
		return nil, nil
	})
}
