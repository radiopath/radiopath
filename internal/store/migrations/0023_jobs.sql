CREATE TABLE jobs (
  id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  owner_id    bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  coverage_id bigint REFERENCES coverages(id) ON DELETE CASCADE,
  kind        text NOT NULL,
  input       jsonb NOT NULL,
  input_hash  text NOT NULL,
  state       text NOT NULL CHECK (state IN ('queued', 'running', 'done', 'failed')),
  attempts    integer NOT NULL DEFAULT 0,
  error       text,
  worker      text,
  result      jsonb,
  queued_at   timestamptz NOT NULL DEFAULT now(),
  started_at  timestamptz,
  finished_at timestamptz,
  compute_ms  integer,
  UNIQUE (owner_id, input_hash)
);
CREATE INDEX jobs_active_idx ON jobs (kind, queued_at) WHERE state IN ('queued', 'running');
CREATE INDEX jobs_coverage_idx ON jobs (coverage_id);

-- Coverage jobs move from the coverages row into the table; the hash is the one EnqueueCoverage computes.
INSERT INTO jobs (owner_id, coverage_id, kind, input, input_hash, state, error, worker, queued_at, started_at, finished_at, compute_ms)
  SELECT owner_id, id, 'coverage', jsonb_build_object('coverage_id', id),
    encode(sha256(convert_to('coverage' || E'\n' || '{"coverage_id":' || id || '}', 'UTF8')), 'hex'),
    job_state, job_error, job_worker, coalesce(job_queued_at, computed_at, now()), job_started_at,
    CASE WHEN job_state IN ('done', 'failed') THEN coalesce(computed_at, now()) END, compute_ms
  FROM coverages WHERE owner_id IS NOT NULL AND job_state <> 'idle';

ALTER TABLE coverages
  DROP COLUMN job_state,
  DROP COLUMN job_queued_at,
  DROP COLUMN job_started_at,
  DROP COLUMN job_worker,
  DROP COLUMN job_error;
