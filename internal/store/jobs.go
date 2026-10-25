package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Job states; JobIdle is the state of a coverage without a job.
const (
	JobIdle    = "idle"
	JobQueued  = "queued"
	JobRunning = "running"
	JobDone    = "done"
	JobFailed  = "failed"
)

// KindCoverage is the kind of the coverage workers; every other kind is a short job.
const KindCoverage = "coverage"

type Job struct {
	ID         int64
	OwnerID    int64
	CoverageID *int64
	Kind       string
	Input      []byte // JSON, the snapshot the worker computes from
	State      string
	Attempts   int // claims so far
	Error      string
	Worker     string
	Result     []byte // JSON, short jobs only
	QueuedAt   *time.Time
	StartedAt  *time.Time
	FinishedAt *time.Time
	ComputeMs  int32
}

const jobCols = `id, owner_id, coverage_id, kind, input, state, attempts, coalesce(error, ''), coalesce(worker, ''), result,
	queued_at, started_at, finished_at, coalesce(compute_ms, 0)`

// scanJob scans a row selected with jobCols.
func scanJob(row pgx.Row) (Job, error) {
	var j Job
	err := row.Scan(&j.ID, &j.OwnerID, &j.CoverageID, &j.Kind, &j.Input, &j.State, &j.Attempts, &j.Error, &j.Worker, &j.Result,
		&j.QueuedAt, &j.StartedAt, &j.FinishedAt, &j.ComputeMs)
	return j, mapErr(err)
}

// jobHash identifies a job by its kind and input; the same text in SQL lives in migration 0023.
func jobHash(kind string, input []byte) string {
	h := sha256.Sum256(append([]byte(kind+"\n"), input...))
	return hex.EncodeToString(h[:])
}

// EnqueueJob queues a job for the owner, or returns the job the owner already has for the same kind and input;
// a failed one is queued again. ErrQuota when the owner has the limit of the kind active, ErrBusy at MaxQueuedJobs.
func (s *Store) EnqueueJob(ctx context.Context, owner int64, coverage *int64, kind string, input []byte) (Job, error) {
	hash := jobHash(kind, input)
	existing := func() (Job, error) {
		return scanJob(s.pool.QueryRow(ctx, `SELECT `+jobCols+` FROM jobs WHERE owner_id = $1 AND input_hash = $2`, owner, hash))
	}
	j, err := existing()
	if err == nil && j.State != JobFailed {
		return j, nil
	}
	if err != nil && !errors.Is(err, ErrNotFound) {
		return Job{}, err
	}
	limit := MaxShortJobsPerUser
	if kind == KindCoverage {
		limit = MaxJobsPerUser
	}
	var active, queued int
	err = s.pool.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM jobs WHERE owner_id = $1 AND state IN ($2, $3) AND (kind = $4) = ($5 = $4)),
		(SELECT count(*) FROM jobs WHERE state = $2)`, owner, JobQueued, JobRunning, KindCoverage, kind).Scan(&active, &queued)
	if err != nil {
		return Job{}, err
	}
	if active >= limit {
		return Job{}, ErrQuota
	}
	if queued >= MaxQueuedJobs {
		return Job{}, ErrBusy
	}
	j, err = scanJob(s.pool.QueryRow(ctx, `INSERT INTO jobs (owner_id, coverage_id, kind, input, input_hash, state)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (owner_id, input_hash) DO UPDATE SET state = $6, attempts = 0, error = NULL, worker = NULL, result = NULL,
			queued_at = now(), started_at = NULL, finished_at = NULL, compute_ms = NULL
		WHERE jobs.state = $7
		RETURNING `+jobCols, owner, coverage, kind, input, hash, JobQueued, JobFailed))
	if errors.Is(err, ErrNotFound) {
		// another request queued the same job in the meantime
		return existing()
	}
	return j, err
}

// ClaimJob takes the oldest queued job of one of the kinds, or a running one started more than stale ago, marks it
// running for worker and counts the attempt; ErrNotFound when there is none.
func (s *Store) ClaimJob(ctx context.Context, worker string, kinds []string, stale time.Duration) (Job, error) {
	return scanJob(s.pool.QueryRow(ctx, `UPDATE jobs SET state = $1, started_at = now(), worker = $2, attempts = attempts + 1
		WHERE id = (
			SELECT id FROM jobs
			WHERE kind = ANY($3) AND (state = $4 OR (state = $1 AND started_at < now() - $5::interval))
			ORDER BY queued_at LIMIT 1 FOR UPDATE SKIP LOCKED)
		RETURNING `+jobCols, JobRunning, worker, kinds, JobQueued, stale.String()))
}

// FinishJob stores the result of a short job and marks it done.
func (s *Store) FinishJob(ctx context.Context, id int64, result []byte, elapsed time.Duration) error {
	_, err := s.pool.Exec(ctx, `UPDATE jobs SET state = $2, result = $3, finished_at = now(), compute_ms = $4, error = NULL WHERE id = $1`,
		id, JobDone, result, int32(elapsed.Milliseconds()))
	return err
}

// FailJob marks the job failed with msg.
func (s *Store) FailJob(ctx context.Context, id int64, msg string) error {
	_, err := s.pool.Exec(ctx, `UPDATE jobs SET state = $2, error = $3, finished_at = now() WHERE id = $1`, id, JobFailed, msg)
	return err
}

// RequeueJob puts a running job back to queued, e.g. on shutdown.
func (s *Store) RequeueJob(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx, `UPDATE jobs SET state = $2, worker = NULL WHERE id = $1 AND state = $3`, id, JobQueued, JobRunning)
	return err
}

// EnqueueCoverage queues the owner's coverage; a job already queued or running stays, a finished one is replaced.
// ErrQuota at MaxJobsPerUser active coverage jobs, ErrNotFound when not owned.
func (s *Store) EnqueueCoverage(ctx context.Context, owner, id int64) error {
	var ok bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM coverages WHERE owner_id = $1 AND id = $2)`, owner, id).Scan(&ok); err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	if _, err := s.pool.Exec(ctx, `DELETE FROM jobs WHERE coverage_id = $1 AND state IN ($2, $3)`, id, JobDone, JobFailed); err != nil {
		return err
	}
	_, err := s.EnqueueJob(ctx, owner, &id, KindCoverage, []byte(fmt.Sprintf(`{"coverage_id":%d}`, id)))
	return err
}

// FinishCoverageJob stores the result image, bounds, duration and note on the coverage and marks the job done;
// ErrNotFound when the coverage is gone.
func (s *Store) FinishCoverageJob(ctx context.Context, job, id int64, png, margin []byte, b Bounds, elapsed time.Duration, note string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE coverages SET result_png = $2, result_margin = $9, result_north = $3, result_south = $4,
		result_east = $5, result_west = $6, computed_at = now(), compute_ms = $7, result_note = NULLIF($8, '') WHERE id = $1`,
		id, png, b.North, b.South, b.East, b.West, int32(elapsed.Milliseconds()), note, margin)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if _, err := tx.Exec(ctx, `UPDATE jobs SET state = $2, finished_at = now(), compute_ms = $3, error = NULL WHERE id = $1`,
		job, JobDone, int32(elapsed.Milliseconds())); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// coverageJobQuery joins the newest job of a coverage; idle without one.
const coverageJobQuery = `LEFT JOIN LATERAL (SELECT state, queued_at, started_at, worker, error FROM jobs
	WHERE jobs.coverage_id = c.id ORDER BY jobs.id DESC LIMIT 1) j ON true`

// CoverageJob returns the state of the owner's coverage job; ErrNotFound when not owned.
func (s *Store) CoverageJob(ctx context.Context, owner, id int64) (Job, error) {
	var j Job
	var worker, jobErr *string
	err := s.pool.QueryRow(ctx, `SELECT coalesce(j.state, 'idle'), j.queued_at, j.started_at, j.worker, j.error FROM coverages c `+
		coverageJobQuery+` WHERE c.owner_id = $1 AND c.id = $2`, owner, id).
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
