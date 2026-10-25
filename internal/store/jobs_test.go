package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"
)

// Dedupe, retry after a failure, limits per kind, claims per kind, stale takeover and the coverage view of the job
// queue: DATABASE_URL=... go test ./internal/store/
func TestJobs(t *testing.T) {
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
	if err := st.Migrate(ctx, slog.New(slog.DiscardHandler)); err != nil {
		t.Fatal(err)
	}

	user := func(name string) int64 {
		var id int64
		if err := st.pool.QueryRow(ctx, `INSERT INTO users (name, password_hash) VALUES ($1, 'x')
			ON CONFLICT (name) DO UPDATE SET password_hash = 'x' RETURNING id`, name).Scan(&id); err != nil {
			t.Fatal(err)
		}
		clean := func() {
			st.pool.Exec(ctx, `DELETE FROM coverages WHERE owner_id = $1`, id)
			st.pool.Exec(ctx, `DELETE FROM sites WHERE owner_id = $1`, id)
			st.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
		}
		clean()
		if err := st.pool.QueryRow(ctx, `INSERT INTO users (name, password_hash) VALUES ($1, 'x') RETURNING id`, name).Scan(&id); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(clean)
		return id
	}
	owner, other := user("jobowner"), user("jobother")

	// Short jobs: the same input is one job, a failed one is queued again, a finished one answers from its result.
	a, err := st.EnqueueJob(ctx, owner, nil, "link-analysis", []byte(`{"n":1}`))
	if err != nil || a.State != JobQueued {
		t.Fatalf("EnqueueJob = %+v %v", a, err)
	}
	if again, err := st.EnqueueJob(ctx, owner, nil, "link-analysis", []byte(`{"n":1}`)); err != nil || again.ID != a.ID {
		t.Fatalf("same input: id %d %v, want %d", again.ID, err, a.ID)
	}
	b, err := st.EnqueueJob(ctx, owner, nil, "link-analysis", []byte(`{"n":2}`))
	if err != nil || b.ID == a.ID {
		t.Fatalf("other input: %+v %v", b, err)
	}
	if o, err := st.EnqueueJob(ctx, other, nil, "hf-prediction", []byte(`{"n":1}`)); err != nil || o.ID == a.ID {
		t.Fatalf("other owner, same input: %+v %v", o, err)
	}
	for i := 3; i <= MaxShortJobsPerUser; i++ {
		if _, err := st.EnqueueJob(ctx, owner, nil, "hf-map", []byte(fmt.Sprintf(`{"n":%d}`, i))); err != nil {
			t.Fatalf("job %d: %v", i, err)
		}
	}
	if _, err := st.EnqueueJob(ctx, owner, nil, "hf-map", []byte(`"one too many"`)); !errors.Is(err, ErrQuota) {
		t.Fatalf("over the limit: err = %v, want ErrQuota", err)
	}

	if _, err := st.ClaimJob(ctx, "w", []string{KindCoverage}, time.Minute); !errors.Is(err, ErrNotFound) {
		t.Fatalf("claim coverage with none queued: %v", err)
	}
	c1, err := st.ClaimJob(ctx, "w1", []string{"link-analysis", "hf-map"}, time.Minute)
	if err != nil || c1.ID != a.ID || c1.State != JobRunning || c1.Attempts != 1 || c1.Worker != "w1" {
		t.Fatalf("first claim = %+v %v, want job %d running", c1, err, a.ID)
	}
	if c2, err := st.ClaimJob(ctx, "w2", []string{"link-analysis"}, time.Minute); err != nil || c2.ID != b.ID {
		t.Fatalf("second claim = %+v %v, want job %d", c2, err, b.ID)
	}

	if err := st.FailJob(ctx, a.ID, "boom"); err != nil {
		t.Fatal(err)
	}
	if j, err := st.EnqueueJob(ctx, owner, nil, "link-analysis", []byte(`{"n":1}`)); err != nil || j.ID != a.ID || j.State != JobQueued || j.Attempts != 0 || j.Error != "" {
		t.Fatalf("after failure: %+v %v, want job %d queued again", j, err, a.ID)
	}
	if err := st.FinishJob(ctx, b.ID, []byte(`{"ok":true}`), 1500*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	var result struct{ OK bool }
	if j, err := st.EnqueueJob(ctx, owner, nil, "link-analysis", []byte(`{"n":2}`)); err != nil || j.State != JobDone || j.ComputeMs != 1500 ||
		json.Unmarshal(j.Result, &result) != nil || !result.OK {
		t.Fatalf("finished job: %+v %v", j, err)
	}

	// A running job of a dead worker is taken over after stale and counts another attempt.
	if _, err := st.pool.Exec(ctx, `UPDATE jobs SET started_at = now() - interval '10 minutes' WHERE id = $1`, a.ID); err != nil {
		t.Fatal(err)
	}
	if c, err := st.ClaimJob(ctx, "w3", []string{"link-analysis"}, time.Minute); err != nil || c.ID != a.ID || c.Attempts != 1 {
		t.Fatalf("claim requeued = %+v %v", c, err)
	}
	if _, err := st.pool.Exec(ctx, `UPDATE jobs SET started_at = now() - interval '10 minutes' WHERE id = $1`, a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ClaimJob(ctx, "w4", []string{"link-analysis"}, time.Hour); !errors.Is(err, ErrNotFound) {
		t.Fatalf("not stale yet: %v", err)
	}
	if c, err := st.ClaimJob(ctx, "w4", []string{"link-analysis"}, 5*time.Minute); err != nil || c.ID != a.ID || c.Attempts != 2 || c.Worker != "w4" {
		t.Fatalf("stale takeover = %+v %v", c, err)
	}

	// Coverage jobs: at most one per coverage, the coverage shows the newest, the limit is separate.
	site, err := st.CreateSite(ctx, Site{OwnerID: owner, Name: "S", Lat: 47, Lon: 9, AntennaHeightM: 10})
	if err != nil {
		t.Fatal(err)
	}
	cov := func(name string) int64 {
		id, err := st.CreateCoverage(ctx, Coverage{OwnerID: owner, Name: name, SiteID: site, FreqMHz: 145, TxPowerDBm: 30, RxHeightM: 2,
			RxSensitivityDBm: -100, Polarization: 1, RangeM: 10000, ResolutionM: 100})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	cv1, cv2, cv3 := cov("one"), cov("two"), cov("three")
	if err := st.EnqueueCoverage(ctx, other, cv1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("foreign coverage: %v", err)
	}
	if err := st.EnqueueCoverage(ctx, owner, cv1); err != nil {
		t.Fatal(err)
	}
	if err := st.EnqueueCoverage(ctx, owner, cv1); err != nil {
		t.Fatalf("queued twice: %v", err)
	}
	if err := st.EnqueueCoverage(ctx, owner, cv2); err != nil {
		t.Fatal(err)
	}
	if err := st.EnqueueCoverage(ctx, owner, cv3); !errors.Is(err, ErrQuota) {
		t.Fatalf("third coverage: err = %v, want ErrQuota", err)
	}
	if j, err := st.CoverageJob(ctx, owner, cv1); err != nil || j.State != JobQueued || j.QueuedAt == nil {
		t.Fatalf("CoverageJob = %+v %v", j, err)
	}
	cj, err := st.ClaimJob(ctx, "cw", []string{KindCoverage}, time.Minute)
	if err != nil || cj.CoverageID == nil || *cj.CoverageID != cv1 || cj.OwnerID != owner {
		t.Fatalf("coverage claim = %+v %v", cj, err)
	}
	if err := st.FinishCoverageJob(ctx, cj.ID, cv1, []byte("png"), []byte("margin"), Bounds{North: 1, South: 0, East: 1, West: 0}, 2*time.Second, ""); err != nil {
		t.Fatal(err)
	}
	c, err := st.GetCoverage(ctx, owner, cv1)
	if err != nil || c.Job.State != JobDone || c.ComputedAt == nil || c.ComputeMs != 2000 || c.Bounds == nil {
		t.Fatalf("finished coverage = %+v %v", c.Coverage, err)
	}
	if err := st.EnqueueCoverage(ctx, owner, cv1); err != nil {
		t.Fatalf("recompute: %v", err)
	}
	var n int
	if err := st.pool.QueryRow(ctx, `SELECT count(*) FROM jobs WHERE coverage_id = $1`, cv1).Scan(&n); err != nil || n != 1 {
		t.Fatalf("jobs of the coverage = %d %v, want 1", n, err)
	}
	c.Name = "renamed"
	if err := st.UpdateCoverage(ctx, c.Coverage); err != nil {
		t.Fatal(err)
	}
	if j, err := st.CoverageJob(ctx, owner, cv1); err != nil || j.State != JobIdle {
		t.Fatalf("after update = %+v %v, want idle", j, err)
	}
	if err := st.DeleteCoverage(ctx, owner, cv2); err != nil {
		t.Fatal(err)
	}
	if err := st.pool.QueryRow(ctx, `SELECT count(*) FROM jobs WHERE coverage_id = $1`, cv2).Scan(&n); err != nil || n != 0 {
		t.Fatalf("jobs of a deleted coverage = %d %v", n, err)
	}
}
