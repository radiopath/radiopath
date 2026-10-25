package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/radiopath/radiopath/internal/antenna"
	"github.com/radiopath/radiopath/internal/coverage"
	"github.com/radiopath/radiopath/internal/geo"
	"github.com/radiopath/radiopath/internal/itm"
	"github.com/radiopath/radiopath/internal/link"
	"github.com/radiopath/radiopath/internal/metrics"
	"github.com/radiopath/radiopath/internal/plot"
	"github.com/radiopath/radiopath/internal/store"
)

const (
	pollInterval = 10 * time.Second // fallback for a lost job_queued notification
	jobTimeout   = 10 * time.Minute
	staleAfter   = 15 * time.Minute
	maxAttempts  = 3 // a job claimed this often failed its workers (e.g. out of memory) and is not run again
)

type Worker struct {
	Name     string
	Store    *store.Store
	Coverage coverage.Computer
	Log      *slog.Logger
	Wake     <-chan struct{} // a coverage job was queued; without it the worker only polls
}

// Run claims and computes coverage jobs one at a time until ctx is cancelled; with the queue empty it waits for a
// wake-up or the next poll.
func (w *Worker) Run(ctx context.Context) {
	w.Log.Info("worker started", "name", w.Name)
	for {
		j, err := w.Store.ClaimJob(ctx, w.Name, true, staleAfter)
		switch {
		case errors.Is(err, store.ErrNotFound):
			select {
			case <-ctx.Done():
				return
			case <-w.Wake:
			case <-time.After(pollInterval):
			}
			continue
		case err != nil:
			if ctx.Err() != nil {
				return
			}
			w.Log.Error("claim job", "err", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(pollInterval):
			}
			continue
		}
		if j.Attempts >= maxAttempts {
			if err := w.Store.FailJob(ctx, j.ID, "The calculation was interrupted repeatedly."); err != nil {
				w.Log.Error("mark failed", "err", err)
			}
			w.Log.Warn("job given up", "job", j.ID, "attempts", j.Attempts)
			continue
		}
		c, err := w.Store.GetCoverage(ctx, j.OwnerID, *j.CoverageID)
		if err != nil {
			// the coverage was deleted meanwhile and took its job along
			if ctx.Err() == nil && !errors.Is(err, store.ErrNotFound) {
				w.Log.Error("load coverage", "job", j.ID, "err", err)
			}
			continue
		}
		w.run(ctx, j.ID, c)
		if ctx.Err() != nil {
			return
		}
	}
}

// run computes one claimed job and stores the result or failure; on shutdown it requeues the job instead.
func (w *Worker) run(ctx context.Context, job int64, c store.CoverageWithSite) {
	log := w.Log.With("coverage", c.ID, "name", c.Name)
	log.Info("job started")
	jobCtx, cancel := context.WithTimeout(ctx, jobTimeout)
	defer cancel()

	var pattern antenna.Lobe
	if c.TxAntennaID != nil {
		a, err := w.Store.GetAntenna(jobCtx, c.OwnerID, *c.TxAntennaID)
		if err != nil {
			log.Warn("antenna pattern not loaded, computing omnidirectional", "err", err)
		} else {
			pattern = antenna.Lobe{H: a.PatternH, V: a.PatternV}
		}
	}

	start := time.Now()
	res, err := w.Coverage.Compute(jobCtx, coverage.Input{
		TX: link.Site{Name: c.Site.Name, Pos: geo.Point{Lat: c.Site.Lat, Lon: c.Site.Lon}, AntennaHeightM: c.Site.AntennaHeightM},
		Radio: link.Radio{
			FreqMHz: c.FreqMHz, TxPowerDBm: c.TxPowerDBm, TxGainDBi: c.TxGainDBi, TxLineLossDB: c.TxLineLossDB,
			RxGainDBi: c.RxGainDBi, RxLineLossDB: c.RxLineLossDB, RxSensitivityDBm: c.RxSensitivityDBm,
			Polarization: itm.Polarization(c.Polarization),
			TxPattern:    pattern, TxAzimuthDeg: c.TxAzimuthDeg, TxTiltDeg: c.TxTiltDeg,
		},
		RxHeightM: c.RxHeightM, RangeM: c.RangeM, ResolutionM: c.ResolutionM,
	})

	dbCtx, dbCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer dbCancel()

	if ctx.Err() != nil {
		if err := w.Store.RequeueJob(dbCtx, job); err != nil {
			log.Error("requeue", "err", err)
		}
		metrics.Jobs.WithLabelValues(store.KindCoverage, "requeued").Inc()
		log.Info("job requeued on shutdown")
		return
	}
	if err != nil {
		reason := link.Reason(err)
		if errors.Is(jobCtx.Err(), context.DeadlineExceeded) {
			reason = fmt.Sprintf("The calculation took longer than %s.", jobTimeout)
		}
		if dbErr := w.Store.FailJob(dbCtx, job, reason); dbErr != nil {
			log.Error("mark failed", "err", dbErr)
		}
		metrics.Jobs.WithLabelValues(store.KindCoverage, "failed").Inc()
		log.Warn("job failed", "err", err)
		return
	}

	elapsed := time.Since(start)
	note := ""
	if res.Truncated > 0 {
		note = fmt.Sprintf("%d of %d radials ended early: no DEM data. Add the missing .hgt tiles.", res.Truncated, res.Radials)
		log.Warn("radials truncated: no DEM data", "truncated", res.Truncated, "radials", res.Radials, "err", res.Errors)
	}
	b := store.Bounds{North: res.North, South: res.South, East: res.East, West: res.West}
	if err := w.Store.FinishCoverageJob(dbCtx, job, c.ID, plot.CoveragePNG(res, plot.LegendOr(c.LegendDB, c.LegendColors)), plot.MarginPNG(res), b, elapsed, note); err != nil {
		log.Error("save result", "err", err)
		return
	}
	metrics.Jobs.WithLabelValues(store.KindCoverage, "done").Inc()
	metrics.JobDuration.WithLabelValues(store.KindCoverage).Observe(elapsed.Seconds())
	log.Info("job done", "pixels", res.W, "radials", res.Radials, "ms", elapsed.Milliseconds())
}
