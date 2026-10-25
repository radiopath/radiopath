package store

import (
	"context"
	"log/slog"
	"time"
)

// Notification channels: job_queued carries the kind of the job, job_done the id of a job that finished or failed.
const (
	ChanJobQueued = "job_queued"
	ChanJobDone   = "job_done"
)

// Listen calls fn with the payload of every notification on channel until ctx ends. It holds one pool connection
// and reconnects after a second when it breaks; notifications sent in between are lost, so callers keep a fallback.
func (s *Store) Listen(ctx context.Context, channel string, log *slog.Logger, fn func(payload string)) {
	for ctx.Err() == nil {
		if err := s.listen(ctx, channel, fn); err != nil && ctx.Err() == nil {
			log.Warn("listen", "channel", channel, "err", err)
			select {
			case <-ctx.Done():
			case <-time.After(time.Second):
			}
		}
	}
}

// listen takes a connection, subscribes and delivers notifications until the connection fails or ctx ends.
func (s *Store) listen(ctx context.Context, channel string, fn func(payload string)) error {
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	// the connection has LISTEN state, so it must not go back into the pool
	defer conn.Hijack().Close(context.Background())
	if _, err := conn.Exec(ctx, "LISTEN "+channel); err != nil {
		return err
	}
	for {
		n, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			return err
		}
		fn(n.Payload)
	}
}
