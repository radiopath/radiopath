package web

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/radiopath/radiopath/internal/link"
	"github.com/radiopath/radiopath/internal/store"
	"github.com/radiopath/radiopath/internal/worker"
)

// countingAnalyzer answers every analysis with a fixed result, fails a site named "fail", and counts its calls.
type countingAnalyzer struct{ calls chan struct{} }

// Analyze implements link.Analyzer.
func (a countingAnalyzer) Analyze(_ context.Context, in link.Input) (link.Result, error) {
	a.calls <- struct{}{}
	if in.A.Name == "fail" {
		return link.Result{}, errors.New("no way")
	}
	return link.Result{DistanceM: 42}, nil
}

// The job facade end to end with a real queue: a request queues a link analysis, the notification wakes the worker,
// the worker computes it and wakes the request; the same input is answered from the finished job without another
// computation, a failure comes back as the job's error text. DATABASE_URL=... go test ./internal/web/
func TestRunJob(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("no DATABASE_URL")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	st, err := store.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	log := slog.New(slog.DiscardHandler)
	if err := st.Migrate(ctx, log); err != nil {
		t.Fatal(err)
	}
	st.DeleteUser(ctx, "jobfacade")
	owner, err := st.CreateUser(ctx, "jobfacade", "x")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.DeleteUser(context.Background(), "jobfacade") })

	analyzer := countingAnalyzer{calls: make(chan struct{}, 10)}
	srv := &Server{Store: st, Analyzer: analyzer, Log: log}
	wake := make(chan struct{}, 1)
	go st.Listen(ctx, store.ChanJobQueued, log, func(string) {
		select {
		case wake <- struct{}{}:
		default:
		}
	})
	go st.Listen(ctx, store.ChanJobDone, log, srv.JobDone)
	w := &worker.Worker{Name: "test", Store: st, Log: log, Wake: wake, Compute: srv.RunJob}
	go w.Run(ctx)
	time.Sleep(300 * time.Millisecond) // the listeners subscribe

	input, _ := json.Marshal(link.Input{A: link.Site{Name: "a"}, B: link.Site{Name: "b"}})
	start := time.Now()
	j, err := srv.runJob(ctx, owner, kindLinkAnalysis, input)
	if err != nil || j.State != store.JobDone {
		t.Fatalf("runJob = %+v %v", j, err)
	}
	// Both wake-ups worked when nothing waited for the fallback polls.
	if d := time.Since(start); d > 1500*time.Millisecond {
		t.Errorf("job took %s, the notifications did not wake worker or request", d)
	}
	var res link.Result
	if err := json.Unmarshal(j.Result, &res); err != nil || res.DistanceM != 42 {
		t.Fatalf("result %s: %v", j.Result, err)
	}
	if again, err := srv.runJob(ctx, owner, kindLinkAnalysis, input); err != nil || again.ID != j.ID {
		t.Fatalf("same input again = %+v %v, want job %d", again, err, j.ID)
	}
	if len(analyzer.calls) != 1 {
		t.Errorf("analyzer called %d times, want once", len(analyzer.calls))
	}

	input, _ = json.Marshal(link.Input{A: link.Site{Name: "fail"}})
	if j, err := srv.runJob(ctx, owner, kindLinkAnalysis, input); err != nil || j.State != store.JobFailed || j.Error != "Calculation failed." {
		t.Fatalf("failing job = %+v %v", j, err)
	}
}

// Without the HF data set a worker fails HF jobs with the text the API answers 503 with.
func TestRunJobHFOff(t *testing.T) {
	s := &Server{}
	_, err := s.RunJob(context.Background(), store.Job{Kind: kindHFMap, Input: []byte(`{}`)})
	var pe interface{ Public() string }
	if !errors.As(err, &pe) || pe.Public() != errHFOff.Public() {
		t.Fatalf("err = %v, want the public HF-off text", err)
	}
}
