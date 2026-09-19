ALTER TABLE coverages
  ADD COLUMN job_state      text NOT NULL DEFAULT 'idle' CHECK (job_state IN ('idle', 'queued', 'running', 'done', 'failed')),
  ADD COLUMN job_queued_at  timestamptz,
  ADD COLUMN job_started_at timestamptz,
  ADD COLUMN job_worker     text,
  ADD COLUMN job_error      text;
UPDATE coverages SET job_state = 'done' WHERE computed_at IS NOT NULL;
CREATE INDEX coverages_job_state_idx ON coverages (job_state, job_queued_at);
