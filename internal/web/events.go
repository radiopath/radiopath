package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/radiopath/radiopath/internal/store"
)

const (
	eventPoll         = 2 * time.Second // fallback for a missed job_done notification
	eventMaxLife      = 15 * time.Minute
	maxStreamsPerUser = 5
)

// openStream registers an event stream of user; false when the user already holds maxStreamsPerUser in this process.
func (s *Server) openStream(user int64) bool {
	s.streamsMu.Lock()
	defer s.streamsMu.Unlock()
	if s.streams == nil {
		s.streams = map[int64]int{}
	}
	if s.streams[user] >= maxStreamsPerUser {
		return false
	}
	s.streams[user]++
	return true
}

// closeStream unregisters an event stream of user.
func (s *Server) closeStream(user int64) {
	s.streamsMu.Lock()
	defer s.streamsMu.Unlock()
	if s.streams[user]--; s.streams[user] <= 0 {
		delete(s.streams, user)
	}
}

type jobEvent struct {
	State string `json:"state"`
	Since string `json:"since,omitempty"`
	Error string `json:"error,omitempty"`
}

// newJobEvent converts a job into a client event; Since is the start (running) or queue (queued) time in RFC 3339.
func newJobEvent(j store.Job) jobEvent {
	e := jobEvent{State: j.State, Error: j.Error}
	switch {
	case j.State == store.JobRunning && j.StartedAt != nil:
		e.Since = j.StartedAt.Format(time.RFC3339)
	case j.State == store.JobQueued && j.QueuedAt != nil:
		e.Since = j.QueuedAt.Format(time.RFC3339)
	}
	return e
}

// coverageEvents streams a coverage job's state as server-sent events, sending on change and ending when the job is done, the client leaves or eventMaxLife passes; it waits for the job_done notification and polls as a fallback.
func (s *Server) coverageEvents(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	user := currentUser(r).ID
	if !s.openStream(user) {
		http.Error(w, "too many open streams", http.StatusTooManyRequests)
		return
	}
	defer s.closeStream(user)
	rc := http.NewResponseController(w)
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	send := func(e jobEvent) error {
		b, _ := json.Marshal(e)
		if _, err := fmt.Fprintf(w, "data: %s\n\n", b); err != nil {
			return err
		}
		return rc.Flush()
	}

	deadline := time.After(eventMaxLife)
	var last jobEvent
	first := true
	for {
		j, err := s.Store.CoverageJob(r.Context(), user, id)
		if err != nil {
			if r.Context().Err() == nil {
				s.Log.Warn("job events", "coverage", id, "err", err)
			}
			return
		}
		e := newJobEvent(j)
		if first || e != last {
			if err := send(e); err != nil {
				return
			}
			last, first = e, false
		}
		if j.State != store.JobQueued && j.State != store.JobRunning {
			return
		}
		select {
		case <-r.Context().Done():
			s.waits.forget(j.ID)
			return
		case <-deadline:
			s.waits.forget(j.ID)
			return
		case <-s.waits.wait(j.ID):
		case <-time.After(eventPoll):
		}
	}
}
