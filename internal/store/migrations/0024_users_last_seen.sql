-- Last request of a user over a session or token, written at most once a minute; feeds the active-users metric.
ALTER TABLE users ADD COLUMN last_seen_at timestamptz;
