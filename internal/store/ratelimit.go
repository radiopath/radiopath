package store

import (
	"context"
	"time"
)

func (s *Store) RateHit(ctx context.Context, key string, window time.Duration) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, `INSERT INTO rate_limits (key, count, window_start) VALUES ($1, 1, now())
		ON CONFLICT (key) DO UPDATE SET
			count = CASE WHEN rate_limits.window_start < now() - $2::interval THEN 1 ELSE rate_limits.count + 1 END,
			window_start = CASE WHEN rate_limits.window_start < now() - $2::interval THEN now() ELSE rate_limits.window_start END
		RETURNING count`, key, window.String()).Scan(&count)
	return count, err
}

func (s *Store) RateCount(ctx context.Context, key string, window time.Duration) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, `SELECT COALESCE((SELECT count FROM rate_limits
		WHERE key = $1 AND window_start >= now() - $2::interval), 0)`, key, window.String()).Scan(&count)
	return count, err
}

func (s *Store) DeleteOldRateLimits(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM rate_limits WHERE window_start < now() - interval '1 day'`)
	return err
}
