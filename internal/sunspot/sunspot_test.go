package sunspot

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

const observedJSON = `[
 {"time-tag": "2026-02", "ssn": 77.4, "smoothed_ssn": 99.7},
 {"time-tag": "2026-03", "ssn": 86.6, "smoothed_ssn": 94.4},
 {"time-tag": "2026-04", "ssn": 79.0, "smoothed_ssn": -1.0},
 {"time-tag": "2009-08", "ssn": 0.0, "smoothed_ssn": 0.2},
 {"time-tag": "1957-10", "ssn": 359.4, "smoothed_ssn": 400.0}
]`

const predictedJSON = `[
 {"time-tag": "2026-03", "predicted_ssn": 50.0},
 {"time-tag": "2026-04", "predicted_ssn": 92.3},
 {"time-tag": "2026-10", "predicted_ssn": 87.9}
]`

// serve answers the two NOAA files from memory.
func serve(t *testing.T, observed, predicted string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/observed-solar-cycle-indices.json", func(w http.ResponseWriter, _ *http.Request) { io.WriteString(w, observed) })
	mux.HandleFunc("/predicted-solar-cycle.json", func(w http.ResponseWriter, _ *http.Request) { io.WriteString(w, predicted) })
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// TestLookup prefers a smoothed observation, falls back to the prediction, rounds and limits.
func TestLookup(t *testing.T) {
	srv := serve(t, observedJSON, predictedJSON)
	s := New(srv.URL)
	if _, ok := s.Lookup(2026, 10); ok {
		t.Fatal("a value before the first refresh")
	}
	if err := s.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		year, month int
		want        Value
		ok          bool
	}{
		{2026, 3, Value{SSN: 94}, true},                  // observed beats the prediction of 50
		{2026, 4, Value{SSN: 92, Predicted: true}, true}, // smoothed is -1, so the prediction
		{2026, 10, Value{SSN: 88, Predicted: true}, true},
		{2009, 8, Value{SSN: 1}, true},    // 0.2 is limited up
		{1957, 10, Value{SSN: 311}, true}, // limited down
		{2030, 1, Value{}, false},
	} {
		got, ok := s.Lookup(c.year, c.month)
		if ok != c.ok || got != c.want {
			t.Errorf("%d-%02d: got %+v %v, want %+v %v", c.year, c.month, got, ok, c.want, c.ok)
		}
	}
}

// TestRefreshKeepsValues leaves the old values in place when the source fails or sends nonsense.
func TestRefreshKeepsValues(t *testing.T) {
	var fail atomic.Bool
	good := serve(t, observedJSON, predictedJSON)
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail.Load() {
			http.Error(w, "down", http.StatusServiceUnavailable)
			return
		}
		http.Redirect(w, r, good.URL+r.URL.Path, http.StatusFound)
	}))
	t.Cleanup(bad.Close)

	s := New(bad.URL)
	if err := s.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	fail.Store(true)
	if err := s.Refresh(context.Background()); err == nil {
		t.Error("a 503 was accepted")
	}
	if v, ok := s.Lookup(2026, 10); !ok || v.SSN != 88 {
		t.Errorf("old value lost: %+v %v", v, ok)
	}

	empty := serve(t, `[]`, `[]`)
	if err := New(empty.URL).Refresh(context.Background()); err == nil {
		t.Error("files without values were accepted")
	}
	garbage := serve(t, `not json`, predictedJSON)
	if err := New(garbage.URL).Refresh(context.Background()); err == nil {
		t.Error("garbage was accepted")
	}
}

// TestRunStops returns when the context ends.
func TestRunStops(t *testing.T) {
	srv := serve(t, observedJSON, predictedJSON)
	s := New(srv.URL)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.Run(ctx, slog.New(slog.DiscardHandler)); close(done) }()
	for i := 0; i < 200; i++ {
		if _, ok := s.Lookup(2026, 10); ok {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, ok := s.Lookup(2026, 10); !ok {
		t.Error("Run did not load the values")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("Run did not stop")
	}
}

// TestMissingField skips months whose value field is missing, as after a change of the NOAA format, instead of
// serving them as a sunspot number of 1.
func TestMissingField(t *testing.T) {
	srv := serve(t, `[{"time-tag": "2026-03", "ssn": 86.6, "smoothed_ssn": 94.4}]`, `[{"time-tag": "2026-10", "predicted": 87.9}]`)
	s := New(srv.URL)
	if err := s.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Lookup(2026, 10); ok {
		t.Error("a prediction without its value field was served")
	}
	if v, ok := s.Lookup(2026, 3); !ok || v.SSN != 94 {
		t.Errorf("observed value: %+v %v", v, ok)
	}
}

// mapCache is a Cache in memory that counts its sets.
type mapCache struct {
	m    map[string][]byte
	sets int
}

// Get implements Cache.
func (c *mapCache) Get(_ context.Context, key string) ([]byte, error) { return c.m[key], nil }

// Set implements Cache.
func (c *mapCache) Set(_ context.Context, key string, val []byte, _ time.Duration) error {
	c.m[key] = val
	c.sets++
	return nil
}

// TestCache fills the cache on the first refresh; a second source with the same cache needs no server at all.
func TestCache(t *testing.T) {
	srv := serve(t, observedJSON, predictedJSON)
	cache := &mapCache{m: map[string][]byte{}}
	s := New(srv.URL)
	s.Cache = cache
	if err := s.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if cache.sets != 2 {
		t.Fatalf("%d files cached, want 2", cache.sets)
	}
	srv.Close()
	other := New(srv.URL)
	other.Cache = cache
	if err := other.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if v, ok := other.Lookup(2026, 3); !ok || v.SSN != 94 {
		t.Errorf("from the cache: %+v %v", v, ok)
	}
	if cache.sets != 2 {
		t.Errorf("%d sets after a cache hit, want still 2", cache.sets)
	}
}
