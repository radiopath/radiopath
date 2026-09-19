package web

import (
	"errors"
	"fmt"
	"net/http"
	netmail "net/mail"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/radiopath/radiopath/internal/store"
)

const (
	verifyTTL   = 24 * time.Hour
	resetTTL    = time.Hour
	maxEmailLen = 254

	mailWindow    = time.Hour
	forgotIPMax   = 5
	forgotMailMax = 3
	verifyUserMax = 3
)

var notices = map[string]string{
	"registered": "Almost done: open the confirmation link we sent to your address, then log in.",
	"unverified": "Your address is not confirmed yet. Open the link from the mail we sent you, or request a new one.",
	"resent":     "If that account is waiting for confirmation, a new link is on its way.",
	"verified":   "Address confirmed.",
	"forgot":     "If that address belongs to an account, a reset link is on its way.",
	"reset":      "Password changed. Log in with the new one.",
	"sent":       "Confirmation link sent to the new address. It takes effect once you open it.",
	"password":   "Password changed. Other devices have been logged out.",
}

func notice(r *http.Request) string { return notices[r.URL.Query().Get("msg")] }

const (
	verifyMail = `Hi %s,

confirm this address for your Radiopath account by opening

%s

The link is valid for 24 hours. If you did not ask for this, ignore this mail.
`
	resetMail = `Hi %s,

somebody asked to reset the password of your Radiopath account. Open

%s

to choose a new one; the link is valid for one hour. If that was not you,
ignore this mail, your password stays as it is.
`
)

func validEmail(s string) string {
	s = strings.TrimSpace(s)
	a, err := netmail.ParseAddress(s)
	if err != nil || a.Name != "" || a.Address != s || len(s) > maxEmailLen {
		return ""
	}
	return s
}

func (s *Server) sendLink(r *http.Request, u store.User, email, purpose string) error {
	token, id := newSessionToken()
	ttl, path, subject, text := resetTTL, "/reset/", "Reset your Radiopath password", resetMail
	if purpose == store.TokenVerify {
		ttl, path, subject, text = verifyTTL, "/verify/", "Confirm your Radiopath address", verifyMail
	}
	if err := s.Store.CreateEmailToken(r.Context(), id, u.ID, email, purpose, time.Now().Add(ttl)); err != nil {
		return err
	}
	return s.Mail.Send(r.Context(), email, subject, fmt.Sprintf(text, u.Name, s.BaseURL+path+token))
}

func (s *Server) sendVerification(r *http.Request, u store.User, email string) (bool, error) {
	n, err := s.Store.RateHit(r.Context(), fmt.Sprintf("verify:user:%d", u.ID), mailWindow)
	if err != nil {
		return false, err
	}
	if n > verifyUserMax {
		s.Log.Warn("confirmation mail rate limited", "user", u.Name)
		return false, nil
	}
	return true, s.sendLink(r, u, email, store.TokenVerify)
}

func (s *Server) resendVerification(w http.ResponseWriter, r *http.Request) {
	name := strings.ToLower(strings.TrimSpace(r.PostFormValue("name")))
	u, _, err := s.Store.Credentials(r.Context(), name)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		s.fail(w, err)
		return
	}
	if err == nil && !u.Verified {
		if _, err := s.sendVerification(r, u, u.Email); err != nil {
			s.fail(w, err)
			return
		}
		s.Log.Info("confirmation resent", "user", u.Name)
	}
	http.Redirect(w, r, "/login?msg=resent", http.StatusSeeOther)
}

type accountData struct {
	base
	Email    string
	Verified bool
	Notice   string
	Errors   []string
}

func (s *Server) accountPage(w http.ResponseWriter, r *http.Request, status int, errs []string) {
	u := currentUser(r)
	s.render(w, status, "account", accountData{base: s.base(r), Email: u.Email, Verified: u.Verified, Notice: notice(r), Errors: errs})
}

func (s *Server) account(w http.ResponseWriter, r *http.Request) {
	s.accountPage(w, r, http.StatusOK, nil)
}

func (s *Server) accountEmail(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	email := validEmail(r.PostFormValue("email"))
	if email == "" {
		s.accountPage(w, r, http.StatusUnprocessableEntity, []string{"Email: not a valid address"})
		return
	}
	if strings.EqualFold(email, u.Email) {
		http.Redirect(w, r, "/account", http.StatusSeeOther)
		return
	}
	sent, err := s.sendVerification(r, u, email)
	if err != nil {
		s.fail(w, err)
		return
	}
	if !sent {
		s.accountPage(w, r, http.StatusTooManyRequests, []string{tooMany(mailWindow)})
		return
	}
	s.Log.Info("email change requested", "user", u.Name)
	http.Redirect(w, r, "/account?msg=sent", http.StatusSeeOther)
}

func (s *Server) accountPassword(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	_, hash, err := s.Store.Credentials(r.Context(), u.Name)
	if err != nil {
		s.fail(w, err)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(r.PostFormValue("current"))) != nil {
		s.accountPage(w, r, http.StatusUnprocessableEntity, []string{"Current password is wrong"})
		return
	}
	password := r.PostFormValue("password")
	errs := validatePassword(password)
	if password != r.PostFormValue("password2") {
		errs = append(errs, "Passwords do not match")
	}
	if len(errs) > 0 {
		s.accountPage(w, r, http.StatusUnprocessableEntity, errs)
		return
	}
	if err := s.setPassword(r, u, password); err != nil {
		s.fail(w, err)
		return
	}
	if err := s.startSession(w, r, u.ID); err != nil {
		s.fail(w, err)
		return
	}
	s.Log.Info("password changed", "user", u.Name)
	http.Redirect(w, r, "/account?msg=password", http.StatusSeeOther)
}

func (s *Server) setPassword(r *http.Request, u store.User, password string) error {
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	if err := s.Store.SetPassword(r.Context(), u.Name, hash); err != nil {
		return err
	}
	_, err = s.Store.DeleteUserSessions(r.Context(), u.Name)
	return err
}

type forgotData struct {
	base
	Notice string
	Errors []string
}

func (s *Server) forgotForm(w http.ResponseWriter, r *http.Request) {
	s.render(w, http.StatusOK, "forgot", forgotData{Notice: notice(r)})
}

func (s *Server) forgot(w http.ResponseWriter, r *http.Request) {
	email := validEmail(r.PostFormValue("email"))
	if email == "" {
		s.render(w, http.StatusUnprocessableEntity, "forgot", forgotData{Errors: []string{"Email: not a valid address"}})
		return
	}
	for _, l := range []struct {
		key string
		max int
	}{{"forgot:ip:" + s.clientIP(r), forgotIPMax}, {"forgot:mail:" + strings.ToLower(email), forgotMailMax}} {
		n, err := s.Store.RateHit(r.Context(), l.key, mailWindow)
		if err != nil {
			s.fail(w, err)
			return
		}
		if n > l.max {
			s.Log.Warn("reset rate limited", "remote", s.clientIP(r))
			s.render(w, http.StatusTooManyRequests, "forgot", forgotData{Errors: []string{tooMany(mailWindow)}})
			return
		}
	}
	u, err := s.Store.UserByEmail(r.Context(), email)
	if errors.Is(err, store.ErrNotFound) {
		s.Log.Info("reset requested for unknown address", "remote", s.clientIP(r))
		http.Redirect(w, r, "/forgot?msg=forgot", http.StatusSeeOther)
		return
	}
	if err != nil {
		s.fail(w, err)
		return
	}
	if err := s.sendLink(r, u, email, store.TokenReset); err != nil {
		s.fail(w, err)
		return
	}
	s.Log.Info("reset link sent", "user", u.Name)
	http.Redirect(w, r, "/forgot?msg=forgot", http.StatusSeeOther)
}

type resetData struct {
	base
	Token  string
	Errors []string
}

const staleLink = "This link has expired or was already used. Request a new one."

func (s *Server) resetForm(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	ok, err := s.Store.EmailTokenValid(r.Context(), sessionID(token), store.TokenReset)
	if err != nil {
		s.fail(w, err)
		return
	}
	if !ok {
		s.render(w, http.StatusGone, "reset", resetData{Errors: []string{staleLink}})
		return
	}
	s.render(w, http.StatusOK, "reset", resetData{Token: token})
}

func (s *Server) reset(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	password := r.PostFormValue("password")
	errs := validatePassword(password)
	if password != r.PostFormValue("password2") {
		errs = append(errs, "Passwords do not match")
	}
	if len(errs) > 0 {
		s.render(w, http.StatusUnprocessableEntity, "reset", resetData{Token: token, Errors: errs})
		return
	}
	u, _, err := s.Store.ConsumeEmailToken(r.Context(), sessionID(token), store.TokenReset)
	if errors.Is(err, store.ErrNotFound) {
		s.render(w, http.StatusGone, "reset", resetData{Errors: []string{staleLink}})
		return
	}
	if err != nil {
		s.fail(w, err)
		return
	}
	if err := s.setPassword(r, u, password); err != nil {
		s.fail(w, err)
		return
	}
	s.Log.Info("password reset", "user", u.Name)
	http.Redirect(w, r, "/login?msg=reset", http.StatusSeeOther)
}

func (s *Server) verify(w http.ResponseWriter, r *http.Request) {
	u, email, err := s.Store.ConsumeEmailToken(r.Context(), sessionID(r.PathValue("token")), store.TokenVerify)
	if errors.Is(err, store.ErrNotFound) {
		d := s.loginPage("/", "This confirmation link has expired or was already used. Log in to get a new one.")
		s.render(w, http.StatusGone, "login", d)
		return
	}
	if err != nil {
		s.fail(w, err)
		return
	}
	err = s.Store.SetEmail(r.Context(), u.ID, email)
	if errors.Is(err, store.ErrExists) {
		s.render(w, http.StatusConflict, "login", s.loginPage("/", "That address is already used by another account."))
		return
	}
	if err != nil {
		s.fail(w, err)
		return
	}
	s.Log.Info("email confirmed", "user", u.Name)
	if _, err := s.sessionUser(r); err == nil {
		http.Redirect(w, r, "/account?msg=verified", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/login?msg=verified", http.StatusSeeOther)
}
