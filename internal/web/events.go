package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/radiopath/radiopath/internal/store"
)

const (
	eventPoll    = time.Second
	eventMaxLife = 15 * time.Minute
)

type jobEvent struct {
	State  string `json:"state"`
	Since  string `json:"since,omitempty"`
	Worker string `json:"worker,omitempty"`
	Error  string `json:"error,omitempty"`
}

func newJobEvent(j store.Job) jobEvent {
	e := jobEvent{State: j.State, Worker: j.Worker, Error: j.Error}
	switch {
	case j.State == store.JobRunning && j.StartedAt != nil:
		e.Since = j.StartedAt.Local().Format("15:04:05")
	case j.State == store.JobQueued && j.QueuedAt != nil:
		e.Since = j.QueuedAt.Local().Format("15:04:05")
	}
	return e
}

func (s *Server) coverageEvents(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
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
	tick := time.NewTicker(eventPoll)
	defer tick.Stop()
	var last jobEvent
	first := true
	for {
		j, err := s.Store.CoverageJob(r.Context(), currentUser(r).ID, id)
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
			return
		case <-deadline:
			return
		case <-tick.C:
		}
	}
}
