ALTER TABLE users
  ADD COLUMN email       text,
  ADD COLUMN verified_at timestamptz;
UPDATE users SET verified_at = created_at;
CREATE UNIQUE INDEX users_email_idx ON users (lower(email));
CREATE TABLE email_tokens (
  id         text PRIMARY KEY,
  user_id    bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  email      text NOT NULL,
  purpose    text NOT NULL CHECK (purpose IN ('verify', 'reset')),
  expires_at timestamptz NOT NULL
);
CREATE INDEX email_tokens_expires_idx ON email_tokens (expires_at);
