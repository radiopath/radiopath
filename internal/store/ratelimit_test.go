package store

import (
	"context"
	"os"
	"testing"
	"time"
)

// The window reset lives in SQL, so it needs a database: DATABASE_URL=... go test ./internal/store/
// make db-up brings one up; without it the test skips.
func TestRateHitWindow(t *testing.T) {
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

	const key = "test:ratehitwindow"
	st.pool.Exec(ctx, `DELETE FROM rate_limits WHERE key = $1`, key)
	defer st.pool.Exec(ctx, `DELETE FROM rate_limits WHERE key = $1`, key)

	for want := 1; want <= 3; want++ {
		if n, err := st.RateHit(ctx, key, time.Hour); err != nil || n != want {
			t.Fatalf("RateHit = %d, %v; want %d", n, err, want)
		}
	}
	if n, err := st.RateCount(ctx, key, time.Hour); err != nil || n != 3 {
		t.Fatalf("RateCount = %d, %v; want 3", n, err)
	}

	// age the window past its end: the next hit starts over, and a stale count reads as zero
	if _, err := st.pool.Exec(ctx, `UPDATE rate_limits SET window_start = now() - interval '2 hours' WHERE key = $1`, key); err != nil {
		t.Fatal(err)
	}
	if n, err := st.RateCount(ctx, key, time.Hour); err != nil || n != 0 {
		t.Fatalf("RateCount after window = %d, %v; want 0", n, err)
	}
	if n, err := st.RateHit(ctx, key, time.Hour); err != nil || n != 1 {
		t.Fatalf("RateHit after window = %d, %v; want 1", n, err)
	}
}
