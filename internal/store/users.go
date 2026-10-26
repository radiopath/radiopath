package store

import (
	"context"
	"time"
)

type User struct {
	ID        int64
	Name      string
	Email     string
	Verified  bool
	CreatedAt time.Time
}

const userCols = `u.id, u.name, coalesce(u.email, ''), u.verified_at IS NOT NULL, u.created_at`

// scanUser scans a row selected with userCols into u.
func scanUser(row interface{ Scan(...any) error }, u *User) error {
	return row.Scan(&u.ID, &u.Name, &u.Email, &u.Verified, &u.CreatedAt)
}

// ListUsers returns all users ordered by name.
func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+userCols+` FROM users u ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		var u User
		if err := scanUser(rows, &u); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

type UserStat struct {
	User
	Sessions  int
	LastLogin *time.Time
}

// ListUsersWithSessions returns all users with their number of unexpired sessions and the newest session start.
func (s *Store) ListUsersWithSessions(ctx context.Context) ([]UserStat, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+userCols+`, count(s.id), max(s.created_at)
		FROM users u LEFT JOIN sessions s ON s.user_id = u.id AND s.expires_at > now()
		GROUP BY u.id ORDER BY u.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []UserStat
	for rows.Next() {
		var u UserStat
		if err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.Verified, &u.CreatedAt, &u.Sessions, &u.LastLogin); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// SetPassword sets the password hash of the user (name is case-insensitive); ErrNotFound when unknown.
func (s *Store) SetPassword(ctx context.Context, name, passwordHash string) error {
	tag, err := s.pool.Exec(ctx, `UPDATE users SET password_hash = $2, updated_at = now()
		WHERE name = lower($1)`, name, passwordHash)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteUserSessions deletes all sessions of the user and returns how many.
func (s *Store) DeleteUserSessions(ctx context.Context, name string) (int64, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE user_id IN
		(SELECT id FROM users WHERE name = lower($1))`, name)
	return tag.RowsAffected(), err
}

// CreateUser inserts an already verified user with a lowercased name; ErrExists when the name is taken.
func (s *Store) CreateUser(ctx context.Context, name, passwordHash string) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `INSERT INTO users (name, password_hash, verified_at) VALUES (lower($1), $2, now()) RETURNING id`,
		name, passwordHash).Scan(&id)
	return id, mapErr(err)
}

// Register inserts an unverified user; the address waits in pending_email until it is confirmed. ErrExists when the name is taken.
func (s *Store) Register(ctx context.Context, name, email, passwordHash string) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `INSERT INTO users (name, pending_email, password_hash) VALUES (lower($1), $2, $3) RETURNING id`,
		name, email, passwordHash).Scan(&id)
	return id, mapErr(err)
}

// PendingEmail returns the address an unverified user registered with; ErrNotFound when there is none.
func (s *Store) PendingEmail(ctx context.Context, id int64) (string, error) {
	var email string
	err := s.pool.QueryRow(ctx, `SELECT pending_email FROM users WHERE id = $1 AND verified_at IS NULL AND pending_email IS NOT NULL`, id).Scan(&email)
	return email, mapErr(err)
}

// UpsertUser creates a verified user or replaces the password of an existing one.
func (s *Store) UpsertUser(ctx context.Context, name, passwordHash string) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO users (name, password_hash, verified_at) VALUES (lower($1), $2, now())
		ON CONFLICT (name) DO UPDATE SET password_hash = EXCLUDED.password_hash, updated_at = now()`, name, passwordHash)
	return err
}

// DeleteUser deletes the user; ErrNotFound when unknown. Dependent rows go with it by cascade.
func (s *Store) DeleteUser(ctx context.Context, name string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM users WHERE name = lower($1)`, name)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Credentials returns the user and password hash for name; ErrNotFound when unknown.
func (s *Store) Credentials(ctx context.Context, name string) (User, string, error) {
	var u User
	var hash string
	err := s.pool.QueryRow(ctx, `SELECT `+userCols+`, u.password_hash FROM users u WHERE u.name = lower($1)`, name).
		Scan(&u.ID, &u.Name, &u.Email, &u.Verified, &u.CreatedAt, &hash)
	return u, hash, mapErr(err)
}

// UserByEmail returns the user with the email (case-insensitive); ErrNotFound when none.
func (s *Store) UserByEmail(ctx context.Context, email string) (User, error) {
	var u User
	err := scanUser(s.pool.QueryRow(ctx, `SELECT `+userCols+` FROM users u WHERE lower(email) = lower($1)`, email), &u)
	return u, mapErr(err)
}

// SetEmail sets the confirmed email and marks the user verified; ErrNotFound when unknown, ErrExists when the email is taken.
func (s *Store) SetEmail(ctx context.Context, id int64, email string) error {
	tag, err := s.pool.Exec(ctx, `UPDATE users SET email = $2, pending_email = NULL, verified_at = now(), updated_at = now() WHERE id = $1`, id, email)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

const (
	TokenVerify = "verify"
	TokenReset  = "reset"
)

// CreateEmailToken stores a single-purpose token (TokenVerify or TokenReset) for the user and email.
func (s *Store) CreateEmailToken(ctx context.Context, id string, userID int64, email, purpose string, expires time.Time) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO email_tokens (id, user_id, email, purpose, expires_at) VALUES ($1, $2, $3, $4, $5)`,
		id, userID, email, purpose, expires)
	return err
}

// EmailTokenValid reports whether an unexpired token with that purpose exists, without consuming it.
func (s *Store) EmailTokenValid(ctx context.Context, id, purpose string) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM email_tokens WHERE id = $1 AND purpose = $2 AND expires_at > now())`,
		id, purpose).Scan(&ok)
	return ok, err
}

// ConsumeEmailToken deletes a valid token and returns its user and email; ErrNotFound when missing, expired or of another purpose.
func (s *Store) ConsumeEmailToken(ctx context.Context, id, purpose string) (User, string, error) {
	var u User
	var email string
	err := s.pool.QueryRow(ctx, `WITH t AS (
			DELETE FROM email_tokens WHERE id = $1 AND purpose = $2 AND expires_at > now() RETURNING user_id, email)
		SELECT `+userCols+`, t.email FROM t JOIN users u ON u.id = t.user_id`, id, purpose).
		Scan(&u.ID, &u.Name, &u.Email, &u.Verified, &u.CreatedAt, &email)
	return u, email, mapErr(err)
}

// DeleteUnconfirmedUsers deletes unverified users older than maxAge and returns how many.
func (s *Store) DeleteUnconfirmedUsers(ctx context.Context, maxAge time.Duration) (int64, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM users WHERE verified_at IS NULL AND created_at < now() - $1::interval`, maxAge.String())
	return tag.RowsAffected(), err
}

// DeleteExpiredEmailTokens deletes expired tokens and returns how many.
func (s *Store) DeleteExpiredEmailTokens(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM email_tokens WHERE expires_at <= now()`)
	return tag.RowsAffected(), err
}

// CreateSession stores a session for the user that expires at expires and marks the user as seen.
func (s *Store) CreateSession(ctx context.Context, id string, userID int64, expires time.Time) error {
	_, err := s.pool.Exec(ctx, `WITH seen AS (UPDATE users SET last_seen_at = now() WHERE id = $2)
	INSERT INTO sessions (id, user_id, expires_at) VALUES ($1, $2, $3)`, id, userID, expires)
	return err
}

const sessionTarget = `LEAST(now() + make_interval(secs => $2), created_at + make_interval(secs => $3))`

// SessionUser returns the user of a live session and extends its expiry by idle, capped at maxAge from creation; the update is skipped when the expiry is already within an hour of the target. It marks the user as seen, at most once a minute. ErrNotFound when missing or expired.
func (s *Store) SessionUser(ctx context.Context, id string, idle, maxAge time.Duration) (User, error) {
	var u User
	err := scanUser(s.pool.QueryRow(ctx, `WITH touched AS (
		UPDATE sessions SET expires_at = `+sessionTarget+`
		WHERE id = $1 AND expires_at > now()
			AND expires_at NOT BETWEEN `+sessionTarget+` - interval '1 hour' AND `+sessionTarget+`
	), seen AS (
		UPDATE users SET last_seen_at = now()
		WHERE id = (SELECT user_id FROM sessions WHERE id = $1 AND expires_at > now())
			AND (last_seen_at IS NULL OR last_seen_at < now() - interval '1 minute')
	)
	SELECT `+userCols+` FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.id = $1 AND s.expires_at > now()`,
		id, idle.Seconds(), maxAge.Seconds()), &u)
	return u, mapErr(err)
}

// DeleteSession deletes a session; a missing id is not an error.
func (s *Store) DeleteSession(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE id = $1`, id)
	return err
}

// DeleteOtherSessions deletes every session of the user except keepID and returns how many.
func (s *Store) DeleteOtherSessions(ctx context.Context, userID int64, keepID string) (int64, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1 AND id <> $2`, userID, keepID)
	return tag.RowsAffected(), err
}

// DeleteAllSessions deletes every session and returns how many.
func (s *Store) DeleteAllSessions(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM sessions`)
	return tag.RowsAffected(), err
}

// DeleteExpiredSessions deletes expired sessions and returns how many.
func (s *Store) DeleteExpiredSessions(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE expires_at <= now()`)
	return tag.RowsAffected(), err
}
