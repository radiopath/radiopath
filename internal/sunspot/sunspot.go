// Package sunspot keeps the 12-month smoothed sunspot number of a month, the R12 the HF model needs, from the solar
// cycle data of NOAA SWPC.
package sunspot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"
)

// DefaultURL is the directory of NOAA SWPC with the solar cycle files.
const DefaultURL = "https://services.swpc.noaa.gov/json/solar-cycle"

const (
	refreshEvery = 12 * time.Hour // the files change about once a day, the smoothed value once a month
	retryAfter   = 15 * time.Minute
	maxBody      = 8 << 20 // the observed file is about 500 KB
)

// Value is the smoothed sunspot number of a month.
type Value struct {
	SSN       int  // rounded and limited to 1..311, what the model accepts
	Predicted bool // a prediction of NOAA, no smoothed observation exists yet
}

// Source holds the values of all months NOAA publishes and refreshes them from the network.
type Source struct {
	base   string
	client *http.Client

	mu     sync.RWMutex
	months map[string]Value // by "2026-10"
}

// New returns a Source that reads the solar cycle files below baseURL, without values until the first Refresh.
func New(baseURL string) *Source {
	return &Source{base: strings.TrimRight(baseURL, "/"), client: &http.Client{Timeout: 30 * time.Second}}
}

// Lookup returns the value of a month, or false when NOAA has none for it or nothing was loaded yet.
func (s *Source) Lookup(year, month int) (Value, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.months[fmt.Sprintf("%04d-%02d", year, month)]
	return v, ok
}

// Refresh loads both files and replaces the values; on an error the old values stay. A smoothed observation wins over
// a prediction of the same month. Values are pointers so a field NOAA renames or drops is skipped instead of read as 0.
func (s *Source) Refresh(ctx context.Context) error {
	var observed []struct {
		Month    string   `json:"time-tag"`
		Smoothed *float64 `json:"smoothed_ssn"` // -1 where it cannot be calculated yet
	}
	if err := s.get(ctx, "observed-solar-cycle-indices.json", &observed); err != nil {
		return err
	}
	var predicted []struct {
		Month     string   `json:"time-tag"`
		Predicted *float64 `json:"predicted_ssn"`
	}
	if err := s.get(ctx, "predicted-solar-cycle.json", &predicted); err != nil {
		return err
	}

	months := make(map[string]Value, len(observed)+len(predicted))
	for _, p := range predicted {
		if p.Predicted != nil && *p.Predicted >= 0 {
			months[p.Month] = Value{SSN: limit(*p.Predicted), Predicted: true}
		}
	}
	for _, o := range observed {
		if o.Smoothed != nil && *o.Smoothed >= 0 {
			months[o.Month] = Value{SSN: limit(*o.Smoothed)}
		}
	}
	if len(months) == 0 {
		return fmt.Errorf("sunspot: no values in the files of %s", s.base)
	}
	s.mu.Lock()
	s.months = months
	s.mu.Unlock()
	return nil
}

// Run refreshes the values until ctx ends: at once, then every twelve hours, after a failure every quarter of an hour.
func (s *Source) Run(ctx context.Context, log *slog.Logger) {
	for {
		wait := refreshEvery
		if err := s.Refresh(ctx); err != nil {
			log.Warn("sunspot data not refreshed", "err", err)
			wait = retryAfter
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
	}
}

// get decodes the JSON file name of the source into v.
func (s *Source) get(ctx context.Context, name string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.base+"/"+name, nil)
	if err != nil {
		return err
	}
	res, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("sunspot: %s answered %s", name, res.Status)
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, maxBody)).Decode(v); err != nil {
		return fmt.Errorf("sunspot: %s: %w", name, err)
	}
	return nil
}

// limit rounds a sunspot number into the range the model accepts.
func limit(v float64) int {
	return int(math.Min(math.Max(math.Round(v), 1), 311))
}
