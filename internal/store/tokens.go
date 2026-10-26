package store

import (
	"context"
	"time"
)

type Token struct {
	ID         int64
	Name       string
	Prefix     string
	Scopes     []string
	CreatedAt  time.Time
	ExpiresAt  *time.Time
	LastUsedAt *time.Time
}

const tokenCols = `id, name, prefix, scopes, created_at, expires_at, last_used_at`

// scanToken scans a row selected with tokenCols into t.
func scanToken(row interface{ Scan(...any) error }, t *Token) error {
	return row.Scan(&t.ID, &t.Name, &t.Prefix, &t.Scopes, &t.CreatedAt, &t.ExpiresAt, &t.LastUsedAt)
}

// CreateToken stores a token of owner by its hash and returns it; ErrQuota at MaxTokensPerUser. A nil expires never expires.
func (s *Store) CreateToken(ctx context.Context, owner int64, name, prefix, hash string, scopes []string, expires *time.Time) (Token, error) {
	var n int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM api_tokens WHERE user_id = $1`, owner).Scan(&n); err != nil {
		return Token{}, err
	}
	if n >= MaxTokensPerUser {
		return Token{}, ErrQuota
	}
	var t Token
	err := scanToken(s.pool.QueryRow(ctx, `INSERT INTO api_tokens (user_id, name, prefix, token_hash, scopes, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING `+tokenCols, owner, name, prefix, hash, scopes, expires), &t)
	return t, err
}

// ListTokens returns the owner's tokens, expired ones included, newest first.
func (s *Store) ListTokens(ctx context.Context, owner int64) ([]Token, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+tokenCols+` FROM api_tokens WHERE user_id = $1 ORDER BY created_at DESC, id DESC`, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Token, 0)
	for rows.Next() {
		var t Token
		if err := scanToken(rows, &t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// DeleteToken deletes the owner's token; ErrNotFound when not owned.
func (s *Store) DeleteToken(ctx context.Context, owner, id int64) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM api_tokens WHERE id = $1 AND user_id = $2`, id, owner)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

// TokenUser returns the user, token id and scopes of a live token and records its use on the token and the user, at
// most once a minute each;
// ErrNotFound when missing or expired.
func (s *Store) TokenUser(ctx context.Context, hash string) (User, int64, []string, error) {
	var u User
	var id int64
	var scopes []string
	err := s.pool.QueryRow(ctx, `WITH touched AS (
		UPDATE api_tokens SET last_used_at = now()
		WHERE token_hash = $1 AND (expires_at IS NULL OR expires_at > now())
			AND (last_used_at IS NULL OR last_used_at < now() - interval '1 minute')
	), seen AS (
		UPDATE users SET last_seen_at = now()
		WHERE id = (SELECT user_id FROM api_tokens WHERE token_hash = $1 AND (expires_at IS NULL OR expires_at > now()))
			AND (last_seen_at IS NULL OR last_seen_at < now() - interval '1 minute')
	)
	SELECT `+userCols+`, t.id, t.scopes FROM api_tokens t JOIN users u ON u.id = t.user_id
		WHERE t.token_hash = $1 AND (t.expires_at IS NULL OR t.expires_at > now())`, hash).
		Scan(&u.ID, &u.Name, &u.Email, &u.Verified, &u.CreatedAt, &id, &scopes)
	return u, id, scopes, mapErr(err)
}

// DeleteExpiredTokens deletes tokens that expired more than keep ago and returns how many; until then the owner
// sees them as expired.
func (s *Store) DeleteExpiredTokens(ctx context.Context, keep time.Duration) (int64, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM api_tokens WHERE expires_at <= now() - make_interval(secs => $1)`, keep.Seconds())
	return tag.RowsAffected(), err
}
