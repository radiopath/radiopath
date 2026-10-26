package store

import (
	"context"
	"os"
	"testing"
	"time"
)

// The sliding expiry lives in SQL, so it needs a database: DATABASE_URL=... go test ./internal/store/
// make dev brings one up; without it the test skips.
func TestSessionSliding(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("no DATABASE_URL")
	}
	ctx := context.Background()
	st, err := New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	const idle, maxAge = 7 * 24 * time.Hour, 30 * 24 * time.Hour
	var uid int64
	if err := st.pool.QueryRow(ctx, `INSERT INTO users (name, password_hash) VALUES ('sessionsliding', 'x')
		ON CONFLICT (name) DO UPDATE SET password_hash = 'x' RETURNING id`).Scan(&uid); err != nil {
		t.Fatal(err)
	}
	defer st.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, uid)

	// a session logged in `created` ago that expires in `expires`, both as intervals from now
	set := func(created, expires string) {
		st.pool.Exec(ctx, `DELETE FROM sessions WHERE id = 'sessionsliding'`)
		if _, err := st.pool.Exec(ctx, `INSERT INTO sessions (id, user_id, created_at, expires_at)
			VALUES ('sessionsliding', $1, now() + $2::interval, now() + $3::interval)`, uid, created, expires); err != nil {
			t.Fatal(err)
		}
	}
	expiry := func() time.Time {
		var ts time.Time
		if err := st.pool.QueryRow(ctx, `SELECT expires_at FROM sessions WHERE id = 'sessionsliding'`).Scan(&ts); err != nil {
			t.Fatal(err)
		}
		return ts
	}
	daysLeft := func() float64 { return time.Until(expiry()).Hours() / 24 }
	use := func() error {
		_, err := st.SessionUser(ctx, "sessionsliding", idle, maxAge)
		return err
	}

	for _, c := range []struct {
		name             string
		created, expires string
		want             float64
	}{
		{"idled towards expiry, pushed back out", "-3 days", "10 minutes", 7},
		{"window longer than the cap, pulled in", "0", "30 days", 7},
		{"old login, capped at created_at + maxAge", "-29 days", "7 days", 1},
	} {
		set(c.created, c.expires)
		if err := use(); err != nil {
			t.Fatal(err)
		}
		if d := daysLeft(); d < c.want-0.1 || d > c.want+0.1 {
			t.Errorf("%s: %.2f days left, want ~%.0f", c.name, d, c.want)
		}
	}

	// within the hour of its target: left alone, so a busy session is not a write per request
	set("-3 days", "7 days")
	before := expiry()
	if err := use(); err != nil {
		t.Fatal(err)
	}
	if after := expiry(); !after.Equal(before) {
		t.Errorf("throttle: expiry moved, %v -> %v", before, after)
	}

	// expired sessions are gone, not revived
	set("-8 days", "-1 minute")
	if err := use(); err != ErrNotFound {
		t.Errorf("expired session: err = %v, want ErrNotFound", err)
	}
	if d := daysLeft(); d > 0 {
		t.Errorf("expired session revived: %.4f days left", d)
	}
}

// last_seen_at follows the session at most once a minute and feeds the active-users counts of Stats.
func TestLastSeen(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("no DATABASE_URL")
	}
	ctx := context.Background()
	st, err := New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	var uid int64
	if err := st.pool.QueryRow(ctx, `INSERT INTO users (name, password_hash) VALUES ('lastseen', 'x')
		ON CONFLICT (name) DO UPDATE SET password_hash = 'x' RETURNING id`).Scan(&uid); err != nil {
		t.Fatal(err)
	}
	defer st.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, uid)
	st.pool.Exec(ctx, `DELETE FROM sessions WHERE id = 'lastseen'`)
	if err := st.CreateSession(ctx, "lastseen", uid, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}

	// last_seen_at put `ago` into the past, then one request; reports how far back it is afterwards
	seenAfterUse := func(ago string) time.Duration {
		if _, err := st.pool.Exec(ctx, `UPDATE users SET last_seen_at = now() - $2::interval WHERE id = $1`, uid, ago); err != nil {
			t.Fatal(err)
		}
		if _, err := st.SessionUser(ctx, "lastseen", time.Hour, 24*time.Hour); err != nil {
			t.Fatal(err)
		}
		var d float64
		if err := st.pool.QueryRow(ctx, `SELECT extract(epoch FROM now() - last_seen_at) FROM users WHERE id = $1`, uid).Scan(&d); err != nil {
			t.Fatal(err)
		}
		return time.Duration(d * float64(time.Second))
	}
	if d := seenAfterUse("2 minutes"); d > 5*time.Second {
		t.Errorf("older than a minute: not refreshed, %v ago", d)
	}
	if d := seenAfterUse("30 seconds"); d < 25*time.Second {
		t.Errorf("within the minute: refreshed anyway, %v ago", d)
	}

	s, err := st.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if s.ActiveDay < 1 || s.ActiveWeek < s.ActiveDay || s.ActiveMonth < s.ActiveWeek {
		t.Errorf("active users: day %d, week %d, month %d", s.ActiveDay, s.ActiveWeek, s.ActiveMonth)
	}
}
