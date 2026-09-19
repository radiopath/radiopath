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

func scanUser(row interface{ Scan(...any) error }, u *User) error {
	return row.Scan(&u.ID, &u.Name, &u.Email, &u.Verified, &u.CreatedAt)
}

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

func (s *Store) DeleteUserSessions(ctx context.Context, name string) (int64, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE user_id IN
		(SELECT id FROM users WHERE name = lower($1))`, name)
	return tag.RowsAffected(), err
}

func (s *Store) CreateUser(ctx context.Context, name, passwordHash string) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `INSERT INTO users (name, password_hash, verified_at) VALUES (lower($1), $2, now()) RETURNING id`,
		name, passwordHash).Scan(&id)
	return id, mapErr(err)
}

func (s *Store) Register(ctx context.Context, name, email, passwordHash string) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `INSERT INTO users (name, email, password_hash) VALUES (lower($1), $2, $3) RETURNING id`,
		name, email, passwordHash).Scan(&id)
	return id, mapErr(err)
}

func (s *Store) UpsertUser(ctx context.Context, name, passwordHash string) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO users (name, password_hash, verified_at) VALUES (lower($1), $2, now())
		ON CONFLICT (name) DO UPDATE SET password_hash = EXCLUDED.password_hash, updated_at = now()`, name, passwordHash)
	return err
}

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

func (s *Store) Credentials(ctx context.Context, name string) (User, string, error) {
	var u User
	var hash string
	err := s.pool.QueryRow(ctx, `SELECT `+userCols+`, u.password_hash FROM users u WHERE u.name = lower($1)`, name).
		Scan(&u.ID, &u.Name, &u.Email, &u.Verified, &u.CreatedAt, &hash)
	return u, hash, mapErr(err)
}

func (s *Store) UserByEmail(ctx context.Context, email string) (User, error) {
	var u User
	err := scanUser(s.pool.QueryRow(ctx, `SELECT `+userCols+` FROM users u WHERE lower(email) = lower($1)`, email), &u)
	return u, mapErr(err)
}

func (s *Store) SetEmail(ctx context.Context, id int64, email string) error {
	tag, err := s.pool.Exec(ctx, `UPDATE users SET email = $2, verified_at = now(), updated_at = now() WHERE id = $1`, id, email)
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

func (s *Store) CreateEmailToken(ctx context.Context, id string, userID int64, email, purpose string, expires time.Time) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO email_tokens (id, user_id, email, purpose, expires_at) VALUES ($1, $2, $3, $4, $5)`,
		id, userID, email, purpose, expires)
	return err
}

func (s *Store) EmailTokenValid(ctx context.Context, id, purpose string) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM email_tokens WHERE id = $1 AND purpose = $2 AND expires_at > now())`,
		id, purpose).Scan(&ok)
	return ok, err
}

func (s *Store) ConsumeEmailToken(ctx context.Context, id, purpose string) (User, string, error) {
	var u User
	var email string
	err := s.pool.QueryRow(ctx, `WITH t AS (
			DELETE FROM email_tokens WHERE id = $1 AND purpose = $2 AND expires_at > now() RETURNING user_id, email)
		SELECT `+userCols+`, t.email FROM t JOIN users u ON u.id = t.user_id`, id, purpose).
		Scan(&u.ID, &u.Name, &u.Email, &u.Verified, &u.CreatedAt, &email)
	return u, email, mapErr(err)
}

func (s *Store) DeleteUnconfirmedUsers(ctx context.Context, maxAge time.Duration) (int64, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM users WHERE verified_at IS NULL AND created_at < now() - $1::interval`, maxAge.String())
	return tag.RowsAffected(), err
}

func (s *Store) DeleteExpiredEmailTokens(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM email_tokens WHERE expires_at <= now()`)
	return tag.RowsAffected(), err
}

func (s *Store) CreateSession(ctx context.Context, id string, userID int64, expires time.Time) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO sessions (id, user_id, expires_at) VALUES ($1, $2, $3)`, id, userID, expires)
	return err
}

const sessionTarget = `LEAST(now() + make_interval(secs => $2), created_at + make_interval(secs => $3))`

func (s *Store) SessionUser(ctx context.Context, id string, idle, maxAge time.Duration) (User, error) {
	var u User
	err := scanUser(s.pool.QueryRow(ctx, `WITH touched AS (
		UPDATE sessions SET expires_at = `+sessionTarget+`
		WHERE id = $1 AND expires_at > now()
			AND expires_at NOT BETWEEN `+sessionTarget+` - interval '1 hour' AND `+sessionTarget+`
	)
	SELECT `+userCols+` FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.id = $1 AND s.expires_at > now()`,
		id, idle.Seconds(), maxAge.Seconds()), &u)
	return u, mapErr(err)
}

func (s *Store) DeleteSession(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE id = $1`, id)
	return err
}

func (s *Store) DeleteAllSessions(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM sessions`)
	return tag.RowsAffected(), err
}

func (s *Store) DeleteExpiredSessions(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE expires_at <= now()`)
	return tag.RowsAffected(), err
}
