package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	netmail "net/mail"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"golang.org/x/crypto/bcrypt"

	"github.com/radiopath/radiopath/internal/metrics"
	"github.com/radiopath/radiopath/internal/store"
)

const (
	verifyTTL   = 24 * time.Hour
	resetTTL    = time.Hour
	maxEmailLen = 254
	mailTimeout = 30 * time.Second

	mailWindow    = time.Hour
	forgotIPMax   = 5
	forgotMailMax = 3
	verifyUserMax = 3
)

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
	changedMail = `Hi %s,

the address of your Radiopath account was changed to another one. If that
was not you, reset your password at

%s

and contact the operator.
`
	knownMail = `Hi %s,

somebody tried to register a new Radiopath account with this address, which
already belongs to your account. If that was you and you forgot the
password, reset it at

%s

Otherwise ignore this mail.
`
	nameTakenMail = `Hello,

somebody tried to register a Radiopath account named %q with this address,
but that user name is taken. Choose another one at

%s

If that was not you, ignore this mail.
`
)

// validEmail returns s trimmed if it is a plain address (no display name, max 254 bytes), otherwise an empty string.
func validEmail(s string) string {
	s = strings.TrimSpace(s)
	a, err := netmail.ParseAddress(s)
	if err != nil || a.Name != "" || a.Address != s || len(s) > maxEmailLen {
		return ""
	}
	return s
}

// sendLink stores a one-time token for purpose and mails the matching confirm or reset link to email. The token sits
// in the fragment, which browsers never send, so it stays out of access logs and proxies.
func (s *Server) sendLink(ctx context.Context, u store.User, email, purpose string) error {
	token, id := newSessionToken()
	ttl, path, subject, text := resetTTL, "/reset#", "Reset your Radiopath password", resetMail
	if purpose == store.TokenVerify {
		ttl, path, subject, text = verifyTTL, "/verify#", "Confirm your Radiopath address", verifyMail
	}
	if err := s.Store.CreateEmailToken(ctx, id, u.ID, email, purpose, time.Now().Add(ttl)); err != nil {
		return err
	}
	s.mailLater(email, subject, fmt.Sprintf(text, u.Name, s.BaseURL+path+token))
	return nil
}

// mailLater sends in the background, so the response time of the public mail flows does not tell whether an address
// or name exists; failures are logged. Without a mailer it only logs: a panic in the goroutine would end the process.
func (s *Server) mailLater(to, subject, text string) {
	if s.Mail == nil {
		s.Log.Warn("mail not configured, not sent", "subject", subject)
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), mailTimeout)
		defer cancel()
		if err := s.Mail.Send(ctx, to, subject, text); err != nil {
			s.Log.Error("mail", "subject", subject, "err", err)
		}
	}()
}

// sendVerification mails a confirmation link, limited per user; sent is false when the limit was hit.
func (s *Server) sendVerification(ctx context.Context, u store.User, email string) (bool, error) {
	n, err := s.Store.RateHit(ctx, fmt.Sprintf("verify:user:%d", u.ID), mailWindow)
	if err != nil {
		return false, err
	}
	if n > verifyUserMax {
		s.Log.Warn("confirmation mail rate limited", "user", u.Name)
		return false, nil
	}
	return true, s.sendLink(ctx, u, email, store.TokenVerify)
}

type PasswordInput struct {
	Body struct {
		Current  string `json:"current" doc:"The password in use"`
		Password string `json:"password" doc:"The new password"`
	}
}

type EmailChangeInput struct {
	Body struct {
		Email    string `json:"email"`
		Password string `json:"password" doc:"The password in use, so a stolen session cannot take over the account"`
	}
}

type EmailOutput struct {
	Status int
}

// registerAccount registers the changes of the logged-in user's address, password and sessions; all need a session, not a token.
func (s *Server) registerAccount(api huma.API) {
	o := op("set-my-email", http.MethodPut, "/api/me/email", scopeSession, "Change the address of the account")
	o.DefaultStatus = http.StatusAccepted
	o.Errors = append(o.Errors, http.StatusNotFound, http.StatusTooManyRequests)
	o.Description = "202: a confirmation link went to the new address, which takes effect once it is opened; the old address gets a notice then. 204: the address is the same. 404: mail is not configured on this server."
	huma.Register(api, o, func(ctx context.Context, in *EmailChangeInput) (*EmailOutput, error) {
		if s.Mail == nil {
			return nil, huma.Error404NotFound("Mail is not configured on this server, so the address cannot be confirmed.")
		}
		u := userOf(ctx)
		if err := s.checkPassword(ctx, u, in.Body.Password, "body.password"); err != nil {
			return nil, err
		}
		email := validEmail(in.Body.Email)
		if email == "" {
			return nil, huma.Error422UnprocessableEntity("Invalid input",
				&huma.ErrorDetail{Location: "body.email", Message: "Email: not a valid address", Value: in.Body.Email})
		}
		if strings.EqualFold(email, u.Email) {
			return &EmailOutput{Status: http.StatusNoContent}, nil
		}
		sent, err := s.sendVerification(ctx, u, email)
		if err != nil {
			return nil, s.storeErr(err)
		}
		if !sent {
			return nil, huma.Error429TooManyRequests(tooMany(mailWindow))
		}
		s.Log.Info("email change requested", "user", u.Name)
		return &EmailOutput{Status: http.StatusAccepted}, nil
	})

	o = op("set-my-password", http.MethodPut, "/api/me/password", scopeSession, "Change the password of the account")
	o.DefaultStatus = http.StatusNoContent
	o.Description = "Logs out every session of the account and sets a fresh cookie for this one."
	huma.Register(api, o, func(ctx context.Context, in *PasswordInput) (*SessionOutput, error) {
		u := userOf(ctx)
		if err := s.checkPassword(ctx, u, in.Body.Current, "body.current"); err != nil {
			return nil, err
		}
		var errs []error
		for _, m := range validatePassword(in.Body.Password) {
			errs = append(errs, &huma.ErrorDetail{Location: "body.password", Message: m})
		}
		if len(errs) > 0 {
			return nil, huma.Error422UnprocessableEntity("Invalid input", errs...)
		}
		if err := s.setPassword(ctx, u, in.Body.Password); err != nil {
			return nil, s.storeErr(err)
		}
		c, err := s.newSession(ctx, u.ID)
		if err != nil {
			return nil, s.storeErr(err)
		}
		s.Log.Info("password changed", "user", u.Name)
		return &SessionOutput{SetCookie: c}, nil
	})

	huma.Register(api, op("delete-my-other-sessions", http.MethodDelete, "/api/me/sessions", scopeSession, "Log out every other session of the account"),
		func(ctx context.Context, _ *struct{}) (*SessionsOutput, error) {
			u := userOf(ctx)
			n, err := s.Store.DeleteOtherSessions(ctx, u.ID, sessionID(reqInfo(ctx).cookie))
			if err != nil {
				return nil, s.storeErr(err)
			}
			s.Log.Info("other sessions revoked", "user", u.Name, "count", n)
			out := &SessionsOutput{}
			out.Body.Count = n
			return out, nil
		})
}

type SessionsOutput struct {
	Body struct {
		Count int64 `json:"count" doc:"How many sessions were logged out"`
	}
}

// checkPassword returns a 422 at field unless password is the one of u.
func (s *Server) checkPassword(ctx context.Context, u store.User, password, field string) error {
	_, hash, err := s.Store.Credentials(ctx, u.Name)
	if err != nil {
		return s.storeErr(err)
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return huma.Error422UnprocessableEntity("Invalid input", &huma.ErrorDetail{Location: field, Message: "Current password is wrong"})
	}
	return nil
}

// setPassword stores the new hash for u and revokes all of its sessions.
func (s *Server) setPassword(ctx context.Context, u store.User, password string) error {
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	if err := s.Store.SetPassword(ctx, u.Name, hash); err != nil {
		return err
	}
	_, err = s.Store.DeleteUserSessions(ctx, u.Name)
	return err
}

type NameInput struct {
	Body struct {
		Name string `json:"name"`
	}
}

type EmailInput struct {
	Body struct {
		Email string `json:"email"`
	}
}

type TokenInput struct {
	Body struct {
		Token string `json:"token" doc:"The token from the mailed link"`
	}
}

type ResetInput struct {
	Body struct {
		Token    string `json:"token" doc:"The token from the mailed link"`
		Password string `json:"password"`
	}
}

const staleLink = "This link has expired or was already used. Request a new one."

// registerMailFlows registers address confirmation and password reset; without a mailer every one answers 404.
func (s *Server) registerMailFlows(api huma.API) {
	noMail := huma.Error404NotFound("Mail is not configured")

	o := op("verify-email", http.MethodPost, "/api/verify", "", "Confirm an address with the token from the mail")
	o.DefaultStatus = http.StatusNoContent
	o.Errors = []int{http.StatusNotFound, http.StatusConflict, http.StatusGone}
	o.Description = "410 for an expired or used token, 409 if another account owns the address."
	huma.Register(api, o, func(ctx context.Context, in *TokenInput) (*struct{}, error) {
		if s.Mail == nil {
			return nil, noMail
		}
		u, email, err := s.Store.ConsumeEmailToken(ctx, sessionID(in.Body.Token), store.TokenVerify)
		if errors.Is(err, store.ErrNotFound) {
			return nil, huma.Error410Gone("This confirmation link has expired or was already used. Log in to get a new one.")
		}
		if err != nil {
			return nil, s.storeErr(err)
		}
		err = s.Store.SetEmail(ctx, u.ID, email)
		if errors.Is(err, store.ErrExists) {
			return nil, huma.Error409Conflict("That address is already used by another account.")
		}
		if err != nil {
			return nil, s.storeErr(err)
		}
		if u.Email != "" && !strings.EqualFold(u.Email, email) {
			s.mailLater(u.Email, "Your Radiopath address was changed", fmt.Sprintf(changedMail, u.Name, s.BaseURL+"/forgot"))
		}
		s.Log.Info("email confirmed", "user", u.Name)
		metrics.Events.WithLabelValues(metrics.EventEmailVerified).Inc()
		return nil, nil
	})

	o = op("resend-verification", http.MethodPost, "/api/verify/resend", "", "Mail a new confirmation link to an unconfirmed account")
	o.DefaultStatus = http.StatusNoContent
	o.Description = "Answers the same for unknown and confirmed names, so accounts cannot be probed."
	huma.Register(api, o, func(ctx context.Context, in *NameInput) (*struct{}, error) {
		if s.Mail == nil {
			return nil, noMail
		}
		u, _, err := s.Store.Credentials(ctx, strings.ToLower(strings.TrimSpace(in.Body.Name)))
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			return nil, s.storeErr(err)
		}
		if err == nil && !u.Verified {
			email, err := s.Store.PendingEmail(ctx, u.ID)
			if errors.Is(err, store.ErrNotFound) {
				return nil, nil
			}
			if err != nil {
				return nil, s.storeErr(err)
			}
			if _, err := s.sendVerification(ctx, u, email); err != nil {
				return nil, s.storeErr(err)
			}
			s.Log.Info("confirmation resent", "user", u.Name)
		}
		return nil, nil
	})

	o = op("forgot-password", http.MethodPost, "/api/password/forgot", "", "Mail a password reset link")
	o.DefaultStatus = http.StatusNoContent
	o.Errors = []int{http.StatusNotFound, http.StatusTooManyRequests}
	o.Description = "Answers the same whether the address belongs to an account or not."
	huma.Register(api, o, func(ctx context.Context, in *EmailInput) (*struct{}, error) {
		if s.Mail == nil {
			return nil, noMail
		}
		email := validEmail(in.Body.Email)
		if email == "" {
			return nil, huma.Error422UnprocessableEntity("Invalid input",
				&huma.ErrorDetail{Location: "body.email", Message: "Email: not a valid address", Value: in.Body.Email})
		}
		ip := reqInfo(ctx).ip
		for _, l := range []struct {
			key string
			max int
		}{{"forgot:ip:" + ip, forgotIPMax}, {"forgot:mail:" + strings.ToLower(email), forgotMailMax}} {
			n, err := s.Store.RateHit(ctx, l.key, mailWindow)
			if err != nil {
				return nil, s.storeErr(err)
			}
			if n > l.max {
				s.Log.Warn("reset rate limited", "remote", ip)
				return nil, huma.Error429TooManyRequests(tooMany(mailWindow))
			}
		}
		u, err := s.Store.UserByEmail(ctx, email)
		if errors.Is(err, store.ErrNotFound) {
			s.Log.Info("reset requested for unknown address", "remote", ip)
			return nil, nil
		}
		if err != nil {
			return nil, s.storeErr(err)
		}
		if err := s.sendLink(ctx, u, email, store.TokenReset); err != nil {
			return nil, s.storeErr(err)
		}
		s.Log.Info("reset link sent", "user", u.Name)
		return nil, nil
	})

	o = op("check-reset-token", http.MethodPost, "/api/password/reset/check", "", "Check that a reset token is still valid")
	o.DefaultStatus = http.StatusNoContent
	o.Errors = []int{http.StatusNotFound, http.StatusGone}
	o.Description = "The token goes in the body, never in a URL, so it stays out of access logs."
	huma.Register(api, o, func(ctx context.Context, in *TokenInput) (*struct{}, error) {
		if s.Mail == nil {
			return nil, noMail
		}
		ok, err := s.Store.EmailTokenValid(ctx, sessionID(in.Body.Token), store.TokenReset)
		if err != nil {
			return nil, s.storeErr(err)
		}
		if !ok {
			return nil, huma.Error410Gone(staleLink)
		}
		return nil, nil
	})

	o = op("reset-password", http.MethodPost, "/api/password/reset", "", "Set a new password with the token from the mail")
	o.DefaultStatus = http.StatusNoContent
	o.Errors = []int{http.StatusNotFound, http.StatusGone}
	o.Description = "Logs out every session of the account."
	huma.Register(api, o, func(ctx context.Context, in *ResetInput) (*struct{}, error) {
		if s.Mail == nil {
			return nil, noMail
		}
		var errs []error
		for _, m := range validatePassword(in.Body.Password) {
			errs = append(errs, &huma.ErrorDetail{Location: "body.password", Message: m})
		}
		if len(errs) > 0 {
			return nil, huma.Error422UnprocessableEntity("Invalid input", errs...)
		}
		u, _, err := s.Store.ConsumeEmailToken(ctx, sessionID(in.Body.Token), store.TokenReset)
		if errors.Is(err, store.ErrNotFound) {
			return nil, huma.Error410Gone(staleLink)
		}
		if err != nil {
			return nil, s.storeErr(err)
		}
		if err := s.setPassword(ctx, u, in.Body.Password); err != nil {
			return nil, s.storeErr(err)
		}
		s.Log.Info("password reset", "user", u.Name)
		metrics.Events.WithLabelValues(metrics.EventPasswordReset).Inc()
		return nil, nil
	})
}
