package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/radiopath/radiopath/internal/link"
	"github.com/radiopath/radiopath/internal/store"
)

// Job kinds the short-job workers compute through RunJob.
const kindLinkAnalysis = "link-analysis"

const (
	jobWait = 25 * time.Second // a request waits this long for its job before it gives up
	jobPoll = 2 * time.Second  // fallback for a missed job_done notification
)

var errJobTimeout = errors.New("the job did not finish in time")

// runJob queues a job of kind for owner, or finds the owner's job with the same input, and waits for it to finish:
// woken by job_done, polling as a fallback, at most jobWait. The job keeps running when the wait ends, so the next
// request finds its result. Dedupe makes a finished job the cache of its input until the janitor removes it.
func (s *Server) runJob(ctx context.Context, owner int64, kind string, input []byte) (store.Job, error) {
	j, err := s.Store.EnqueueJob(ctx, owner, nil, kind, input)
	if err != nil {
		return j, err
	}
	deadline := time.After(jobWait)
	for j.State == store.JobQueued || j.State == store.JobRunning {
		select {
		case <-ctx.Done():
			s.waits.forget(j.ID)
			return j, ctx.Err()
		case <-deadline:
			s.waits.forget(j.ID)
			return j, errJobTimeout
		case <-s.waits.wait(j.ID):
		case <-time.After(jobPoll):
		}
		if j, err = s.Store.GetJob(ctx, j.ID); err != nil {
			return j, err
		}
	}
	return j, nil
}

// jobReason is the text shown for a job that could not be run or waited for; the error itself goes to the log.
func jobReason(err error) string {
	switch {
	case errors.Is(err, store.ErrQuota):
		return "Too many calculations at once, wait for one to finish."
	case errors.Is(err, store.ErrBusy), errors.Is(err, errJobTimeout):
		return "The server is busy, try again in a moment."
	}
	return "Calculation failed."
}

// RunJob computes a short job for the worker: it decodes the input of the job's kind, runs the model and encodes
// the result. It needs the datasets, so it runs in worker processes only.
func (s *Server) RunJob(ctx context.Context, j store.Job) ([]byte, error) {
	switch j.Kind {
	case kindLinkAnalysis:
		var in link.Input
		if err := json.Unmarshal(j.Input, &in); err != nil {
			return nil, err
		}
		res, err := s.Analyzer.Analyze(ctx, in)
		if err != nil {
			return nil, err
		}
		return json.Marshal(res)
	}
	return nil, fmt.Errorf("unknown job kind %q", j.Kind)
}

// jobWaits lets requests of this process wait for a job to finish; the job_done notification closes the channel of
// the job's waiters. A notification that arrives before a waiter registers is missed, so every waiter keeps a poll.
type jobWaits struct {
	mu sync.Mutex
	m  map[int64]chan struct{}
}

// wait returns a channel that closes when job id is reported done; waiters of one job share it.
func (w *jobWaits) wait(id int64) <-chan struct{} {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.m == nil {
		w.m = map[int64]chan struct{}{}
	}
	ch, ok := w.m[id]
	if !ok {
		ch = make(chan struct{})
		w.m[id] = ch
	}
	return ch
}

// done wakes the waiters of job id.
func (w *jobWaits) done(id int64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if ch, ok := w.m[id]; ok {
		close(ch)
		delete(w.m, id)
	}
}

// forget drops the channel of job id when its last waiter gives up, so jobs that never report (e.g. deleted with
// their coverage) leave nothing behind.
func (w *jobWaits) forget(id int64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.m, id)
}

// JobDone takes the payload of a job_done notification, the id of the job, and wakes its waiters.
func (s *Server) JobDone(payload string) {
	if id, err := strconv.ParseInt(payload, 10, 64); err == nil {
		s.waits.done(id)
	}
}
