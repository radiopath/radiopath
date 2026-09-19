package store

import (
	"context"
	"time"
)

func (s *Store) ShareLink(ctx context.Context, owner, id int64, token string, expires time.Time) error {
	return s.share(ctx, `UPDATE links SET share_token = COALESCE(share_token, $3), share_expires_at = $4
		WHERE owner_id = $1 AND id = $2`, owner, id, token, expires)
}

func (s *Store) UnshareLink(ctx context.Context, owner, id int64) error {
	return s.unshare(ctx, `UPDATE links SET share_token = NULL, share_expires_at = NULL
		WHERE owner_id = $1 AND id = $2`, owner, id)
}

func (s *Store) GetSharedLink(ctx context.Context, token string) (LinkWithSites, error) {
	var l LinkWithSites
	err := scanLink(s.pool.QueryRow(ctx, linkQuery+` WHERE l.share_token = $1 AND l.share_expires_at > now()`, token), &l)
	return l, mapErr(err)
}

func (s *Store) ShareCoverage(ctx context.Context, owner, id int64, token string, expires time.Time) error {
	return s.share(ctx, `UPDATE coverages SET share_token = COALESCE(share_token, $3), share_expires_at = $4
		WHERE owner_id = $1 AND id = $2`, owner, id, token, expires)
}

func (s *Store) UnshareCoverage(ctx context.Context, owner, id int64) error {
	return s.unshare(ctx, `UPDATE coverages SET share_token = NULL, share_expires_at = NULL
		WHERE owner_id = $1 AND id = $2`, owner, id)
}

func (s *Store) GetSharedCoverage(ctx context.Context, token string) (CoverageWithSite, error) {
	var c CoverageWithSite
	err := scanCoverage(s.pool.QueryRow(ctx, coverageQuery+` WHERE c.share_token = $1 AND c.share_expires_at > now()`, token), &c)
	return c, mapErr(err)
}

func (s *Store) share(ctx context.Context, sql string, owner, id int64, token string, expires time.Time) error {
	tag, err := s.pool.Exec(ctx, sql, owner, id, token, expires)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) unshare(ctx context.Context, sql string, owner, id int64) error {
	tag, err := s.pool.Exec(ctx, sql, owner, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
