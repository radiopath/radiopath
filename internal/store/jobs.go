package store

import (
	"context"
	"time"
)

const (
	JobIdle    = "idle"
	JobQueued  = "queued"
	JobRunning = "running"
	JobDone    = "done"
	JobFailed  = "failed"
)

type Job struct {
	State     string
	QueuedAt  *time.Time
	StartedAt *time.Time
	Worker    string
	Error     string
}

func (s *Store) EnqueueCoverage(ctx context.Context, owner, id int64) error {
	tag, err := s.pool.Exec(ctx, `UPDATE coverages SET job_state = $3, job_queued_at = now(), job_error = NULL, job_worker = NULL
		WHERE owner_id = $1 AND id = $2 AND job_state NOT IN ($3, $4)
		AND (SELECT count(*) FROM coverages WHERE owner_id = $1 AND job_state IN ($3, $4)) < $5`,
		owner, id, JobQueued, JobRunning, MaxJobsPerUser)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		c, err := s.GetCoverage(ctx, owner, id)
		if err != nil {
			return err
		}
		if c.Job.State != JobQueued && c.Job.State != JobRunning {
			return ErrQuota
		}
	}
	return nil
}

func (s *Store) ClaimCoverageJob(ctx context.Context, worker string, stale time.Duration) (CoverageWithSite, error) {
	var owner, id int64
	err := s.pool.QueryRow(ctx, `UPDATE coverages SET job_state = $1, job_started_at = now(), job_worker = $2
		WHERE id = (
			SELECT id FROM coverages
			WHERE job_state = $3 OR (job_state = $1 AND job_started_at < now() - $4::interval)
			ORDER BY job_queued_at LIMIT 1 FOR UPDATE SKIP LOCKED)
		RETURNING owner_id, id`, JobRunning, worker, JobQueued, stale.String()).Scan(&owner, &id)
	if err != nil {
		return CoverageWithSite{}, mapErr(err)
	}
	return s.GetCoverage(ctx, owner, id)
}

func (s *Store) FinishCoverageJob(ctx context.Context, id int64, png, margin []byte, b Bounds, elapsed time.Duration, note string) error {
	tag, err := s.pool.Exec(ctx, `UPDATE coverages SET result_png = $2, result_margin = $10, result_north = $3, result_south = $4,
		result_east = $5, result_west = $6, computed_at = now(), compute_ms = $7, result_note = NULLIF($8, ''),
		job_state = $9, job_error = NULL WHERE id = $1`,
		id, png, b.North, b.South, b.East, b.West, int32(elapsed.Milliseconds()), note, JobDone, margin)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) FailCoverageJob(ctx context.Context, id int64, msg string) error {
	_, err := s.pool.Exec(ctx, `UPDATE coverages SET job_state = $2, job_error = $3 WHERE id = $1`, id, JobFailed, msg)
	return err
}

func (s *Store) RequeueCoverageJob(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx, `UPDATE coverages SET job_state = $2, job_worker = NULL WHERE id = $1 AND job_state = $3`, id, JobQueued, JobRunning)
	return err
}

func (s *Store) CoverageJob(ctx context.Context, owner, id int64) (Job, error) {
	var j Job
	var worker, jobErr *string
	err := s.pool.QueryRow(ctx, `SELECT job_state, job_queued_at, job_started_at, job_worker, job_error FROM coverages WHERE owner_id = $1 AND id = $2`, owner, id).
		Scan(&j.State, &j.QueuedAt, &j.StartedAt, &worker, &jobErr)
	if err != nil {
		return j, mapErr(err)
	}
	if worker != nil {
		j.Worker = *worker
	}
	if jobErr != nil {
		j.Error = *jobErr
	}
	return j, nil
}
