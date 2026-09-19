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
	pollInterval = 2 * time.Second
	jobTimeout   = 10 * time.Minute
	staleAfter   = 15 * time.Minute
)

type Worker struct {
	Name     string
	Store    *store.Store
	Coverage coverage.Computer
	Log      *slog.Logger
}

func (w *Worker) Run(ctx context.Context) {
	w.Log.Info("worker started", "name", w.Name)
	for {
		c, err := w.Store.ClaimCoverageJob(ctx, w.Name, staleAfter)
		switch {
		case errors.Is(err, store.ErrNotFound):
			select {
			case <-ctx.Done():
				return
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
		w.run(ctx, c)
		if ctx.Err() != nil {
			return
		}
	}
}

func (w *Worker) run(ctx context.Context, c store.CoverageWithSite) {
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
		if err := w.Store.RequeueCoverageJob(dbCtx, c.ID); err != nil {
			log.Error("requeue", "err", err)
		}
		metrics.CoverageJobs.WithLabelValues("requeued").Inc()
		log.Info("job requeued on shutdown")
		return
	}
	if err != nil {
		if errors.Is(jobCtx.Err(), context.DeadlineExceeded) {
			err = fmt.Errorf("computation exceeded %s", jobTimeout)
		}
		if dbErr := w.Store.FailCoverageJob(dbCtx, c.ID, err.Error()); dbErr != nil {
			log.Error("mark failed", "err", dbErr)
		}
		metrics.CoverageJobs.WithLabelValues("failed").Inc()
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
	if err := w.Store.FinishCoverageJob(dbCtx, c.ID, plot.CoveragePNG(res, plot.LegendOr(c.LegendDB, c.LegendColors)), plot.MarginPNG(res), b, elapsed, note); err != nil {
		log.Error("save result", "err", err)
		return
	}
	metrics.CoverageJobs.WithLabelValues("done").Inc()
	metrics.CoverageJobDuration.Observe(elapsed.Seconds())
	log.Info("job done", "pixels", res.W, "radials", res.Radials, "ms", elapsed.Milliseconds())
}
