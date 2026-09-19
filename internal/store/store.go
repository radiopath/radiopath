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
)

const (
	MaxSitesPerUser     = 200
	MaxCoveragesPerUser = 50
	MaxJobsPerUser      = 2
)

type Store struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, url string) (*Store, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() { s.pool.Close() }

func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

type Stats struct {
	Users, Sites, Links, Sessions int
	Coverages                     map[string]int
}

func (s *Store) Stats(ctx context.Context) (Stats, error) {
	var st Stats
	err := s.pool.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM users),
		(SELECT count(*) FROM sites),
		(SELECT count(*) FROM links),
		(SELECT count(*) FROM sessions WHERE expires_at > now())`).
		Scan(&st.Users, &st.Sites, &st.Links, &st.Sessions)
	if err != nil {
		return st, err
	}
	rows, err := s.pool.Query(ctx, `SELECT job_state, count(*) FROM coverages GROUP BY job_state`)
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
	return st, rows.Err()
}

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
