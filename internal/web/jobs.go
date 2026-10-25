package web

import (
	"strconv"
	"sync"
)

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
