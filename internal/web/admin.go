package web

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/radiopath/radiopath/internal/store"
)

const (
	adminCookie = "radiopath_admin"
	adminTTL    = 8 * time.Hour
	adminLabel  = "radiopath-admin-v1:"

	adminWindow = 15 * time.Minute
	adminIPMax  = 10
)

func (s *Server) adminEnabled() bool { return s.AdminToken != "" }

func signAdmin(token string, exp time.Time) string {
	unix := strconv.FormatInt(exp.Unix(), 10)
	mac := hmac.New(sha256.New, []byte(token))
	mac.Write([]byte(adminLabel + unix))
	return unix + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func validAdmin(token, value string, now time.Time) bool {
	unix, sig, ok := strings.Cut(value, ".")
	if !ok {
		return false
	}
	exp, err := strconv.ParseInt(unix, 10, 64)
	if err != nil || now.Unix() >= exp || exp-now.Unix() > int64(adminTTL/time.Second) {
		return false
	}
	// Strict: one MAC must have exactly one encoding
	want, err := base64.RawURLEncoding.Strict().DecodeString(sig)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(token))
	mac.Write([]byte(adminLabel + unix))
	return hmac.Equal(mac.Sum(nil), want)
}

func tokenMatches(configured, posted string) bool {
	// hashed first so the compare cannot leak the token length
	a, b := sha256.Sum256([]byte(configured)), sha256.Sum256([]byte(posted))
	return subtle.ConstantTimeCompare(a[:], b[:]) == 1
}

func (s *Server) isAdmin(r *http.Request) bool {
	c, err := r.Cookie(adminCookie)
	return err == nil && validAdmin(s.AdminToken, c.Value, time.Now())
}

func (s *Server) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.isAdmin(r) {
			s.adminLoginForm(w, r, http.StatusUnauthorized, "Session expired. Enter the token again.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

type adminLoginData struct {
	base
	Error string
}

type adminData struct {
	base
	Users       []store.UserStat
	Errors      []string
	Notice      string
	PendingUser string
	NewName     string
	Maintenance bool
}

func (s *Server) adminLoginForm(w http.ResponseWriter, r *http.Request, status int, msg string) {
	w.Header().Set("Cache-Control", "no-store")
	s.render(w, status, "admin_login", adminLoginData{base: base{Admin: true}, Error: msg})
}

func (s *Server) adminIndex(w http.ResponseWriter, r *http.Request) {
	if !s.isAdmin(r) {
		s.adminLoginForm(w, r, http.StatusOK, "")
		return
	}
	s.adminUsers(w, r, http.StatusOK, adminData{})
}

func (s *Server) adminUsers(w http.ResponseWriter, r *http.Request, status int, d adminData) {
	users, err := s.Store.ListUsersWithSessions(r.Context())
	if err != nil {
		s.fail(w, err)
		return
	}
	maintenance, err := s.Store.Maintenance(r.Context())
	if err != nil {
		s.fail(w, err)
		return
	}
	d.base = base{Admin: true}
	d.Users = users
	d.Maintenance = maintenance
	w.Header().Set("Cache-Control", "no-store")
	s.render(w, status, "admin_users", d)
}

func (s *Server) adminLogin(w http.ResponseWriter, r *http.Request) {
	ip := s.clientIP(r)
	// count before checking: RateHit is atomic across replicas
	n, err := s.Store.RateHit(r.Context(), "admin:ip:"+ip, adminWindow)
	if err != nil {
		s.fail(w, err)
		return
	}
	if n > adminIPMax {
		s.Log.Warn("admin login rate limited", "remote", ip)
		s.adminLoginForm(w, r, http.StatusTooManyRequests, tooMany(adminWindow))
		return
	}
	if !tokenMatches(s.AdminToken, r.PostFormValue("token")) {
		s.Log.Warn("admin login failed", "remote", ip)
		s.adminLoginForm(w, r, http.StatusUnauthorized, "Wrong token.")
		return
	}
	exp := time.Now().Add(adminTTL)
	http.SetCookie(w, &http.Cookie{
		Name: adminCookie, Value: signAdmin(s.AdminToken, exp), Path: "/admin",
		HttpOnly: true, Secure: secureCookie(r), SameSite: http.SameSiteStrictMode,
		MaxAge: int(adminTTL.Seconds()),
	})
	s.Log.Info("admin login", "remote", ip)
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (s *Server) adminLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: adminCookie, Value: "", Path: "/admin", HttpOnly: true, MaxAge: -1})
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (s *Server) adminCreateUser(w http.ResponseWriter, r *http.Request) {
	name := strings.ToLower(strings.TrimSpace(r.PostFormValue("name")))
	password := r.PostFormValue("password")
	if errs := ValidateCredentials(name, password); len(errs) > 0 {
		s.adminUsers(w, r, http.StatusUnprocessableEntity, adminData{Errors: errs, NewName: name})
		return
	}
	hash, err := HashPassword(password)
	if err != nil {
		s.fail(w, err)
		return
	}
	if _, err := s.Store.CreateUser(r.Context(), name, hash); errors.Is(err, store.ErrExists) {
		s.adminUsers(w, r, http.StatusUnprocessableEntity,
			adminData{Errors: []string{"User name is already taken"}, NewName: name})
		return
	} else if err != nil {
		s.fail(w, err)
		return
	}
	s.adminLog(r, "user created", name)
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (s *Server) adminPasswordForm(w http.ResponseWriter, r *http.Request) {
	s.adminUsers(w, r, http.StatusOK, adminData{PendingUser: r.PathValue("name")})
}

func (s *Server) adminSetPassword(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	password := r.PostFormValue("password")
	if errs := ValidateCredentials(name, password); len(errs) > 0 {
		s.adminUsers(w, r, http.StatusUnprocessableEntity, adminData{Errors: errs})
		return
	}
	hash, err := HashPassword(password)
	if err != nil {
		s.fail(w, err)
		return
	}
	if err := s.Store.SetPassword(r.Context(), name, hash); errors.Is(err, store.ErrNotFound) {
		s.adminUsers(w, r, http.StatusNotFound, adminData{Errors: []string{"No such user: " + name}})
		return
	} else if err != nil {
		s.fail(w, err)
		return
	}
	s.adminLog(r, "password set", name)
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (s *Server) adminRevokeSessions(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	n, err := s.Store.DeleteUserSessions(r.Context(), name)
	if err != nil {
		s.fail(w, err)
		return
	}
	s.adminLog(r, "sessions revoked", name)
	s.adminUsers(w, r, http.StatusOK, adminData{Notice: name + ": " + strconv.FormatInt(n, 10) + " session(s) revoked"})
}

func (s *Server) adminDeleteUser(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := s.Store.DeleteUser(r.Context(), name); errors.Is(err, store.ErrNotFound) {
		s.adminUsers(w, r, http.StatusNotFound, adminData{Errors: []string{"No such user: " + name}})
		return
	} else if err != nil {
		s.fail(w, err)
		return
	}
	s.adminLog(r, "user deleted", name)
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (s *Server) adminRevokeAllSessions(w http.ResponseWriter, r *http.Request) {
	n, err := s.Store.DeleteAllSessions(r.Context())
	if err != nil {
		s.fail(w, err)
		return
	}
	s.adminLog(r, "all sessions revoked", "*")
	s.adminUsers(w, r, http.StatusOK, adminData{Notice: strconv.FormatInt(n, 10) + " session(s) revoked, everyone is logged out"})
}

func (s *Server) adminMaintenance(w http.ResponseWriter, r *http.Request) {
	on := r.PostFormValue("on") == "true"
	if err := s.Store.SetMaintenance(r.Context(), on); err != nil {
		s.fail(w, err)
		return
	}
	if on {
		s.adminLog(r, "maintenance on", "*")
	} else {
		s.adminLog(r, "maintenance off", "*")
	}
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (s *Server) adminLog(r *http.Request, action, target string) {
	s.Log.Info("admin action", "action", action, "target", target, "remote", s.clientIP(r))
}
