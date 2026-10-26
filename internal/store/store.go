package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound = errors.New("store: not found")
	ErrInUse    = errors.New("store: referenced by other rows")
	ErrExists   = errors.New("store: already exists")
	ErrQuota    = errors.New("store: per-user limit reached")
	ErrBusy     = errors.New("store: job queue full")
)

const (
	MaxSitesPerUser     = 200
	MaxCoveragesPerUser = 50
	MaxJobsPerUser      = 2  // coverage jobs queued or running
	MaxShortJobsPerUser = 20 // every other kind together
	MaxQueuedJobs       = 500
	MaxTokensPerUser    = 20
)

type Store struct {
	pool *pgxpool.Pool
}

// New opens a connection pool for the given PostgreSQL URL.
func New(ctx context.Context, url string) (*Store, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	return &Store{pool: pool}, nil
}

// Close closes the connection pool.
func (s *Store) Close() { s.pool.Close() }

// Ping checks that the database is reachable.
func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

type JobCount struct {
	Kind, State string
	N           int
}

type Stats struct {
	Users, Sites, Links, Sessions, Antennas, Tokens int
	ActiveDay, ActiveWeek, ActiveMonth              int // users seen within the window
	SharedLinks, SharedCoverages                    int
	Maintenance                                     bool
	Coverages                                       map[string]int
	Jobs                                            []JobCount
}

// Stats returns table counts, active sessions, users, tokens and shares, the maintenance flag, coverages per job state
// and jobs per kind and state.
func (s *Store) Stats(ctx context.Context) (Stats, error) {
	var st Stats
	err := s.pool.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM users),
		(SELECT count(*) FROM sites),
		(SELECT count(*) FROM links),
		(SELECT count(*) FROM sessions WHERE expires_at > now()),
		(SELECT count(*) FROM antennas),
		(SELECT count(*) FROM api_tokens WHERE expires_at IS NULL OR expires_at > now()),
		(SELECT count(*) FROM users WHERE last_seen_at > now() - interval '1 day'),
		(SELECT count(*) FROM users WHERE last_seen_at > now() - interval '7 days'),
		(SELECT count(*) FROM users WHERE last_seen_at > now() - interval '30 days'),
		(SELECT count(*) FROM links WHERE share_expires_at > now()),
		(SELECT count(*) FROM coverages WHERE share_expires_at > now()),
		EXISTS (SELECT 1 FROM settings WHERE key = $1 AND value = 'on')`, maintenanceKey).
		Scan(&st.Users, &st.Sites, &st.Links, &st.Sessions, &st.Antennas, &st.Tokens,
			&st.ActiveDay, &st.ActiveWeek, &st.ActiveMonth, &st.SharedLinks, &st.SharedCoverages, &st.Maintenance)
	if err != nil {
		return st, err
	}
	rows, err := s.pool.Query(ctx, `SELECT coalesce(j.state, 'idle'), count(*) FROM coverages c `+coverageJobQuery+` GROUP BY 1`)
	if err != nil {
		return st, err
	}
	defer rows.Close()
	st.Coverages = make(map[string]int, 5)
	for rows.Next() {
		var state string
		var n int
		if err := rows.Scan(&state, &n); err != nil {
			return st, err
		}
		st.Coverages[state] = n
	}
	if err := rows.Err(); err != nil {
		return st, err
	}
	rows, err = s.pool.Query(ctx, `SELECT kind, state, count(*) FROM jobs GROUP BY 1, 2`)
	if err != nil {
		return st, err
	}
	defer rows.Close()
	for rows.Next() {
		var c JobCount
		if err := rows.Scan(&c.Kind, &c.State, &c.N); err != nil {
			return st, err
		}
		st.Jobs = append(st.Jobs, c)
	}
	return st, rows.Err()
}

// mapErr translates no-rows, foreign-key and unique violations into ErrNotFound, ErrInUse and ErrExists; other errors pass through.
func mapErr(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23503":
			return ErrInUse
		case "23505":
			return ErrExists
		}
	}
	return err
}
