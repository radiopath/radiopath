package store

import (
	"context"
	"fmt"
	"time"
)

type Coverage struct {
	ID               int64
	OwnerID          int64
	Name             string
	SiteID           int64
	FreqMHz          float64
	TxPowerDBm       float64
	TxGainDBi        float64
	TxLineLossDB     float64
	RxHeightM        float64
	RxGainDBi        float64
	RxLineLossDB     float64
	RxSensitivityDBm float64
	Polarization     int16
	RangeM           float64
	ResolutionM      float64

	TxAntennaID  *int64
	TxAzimuthDeg float64
	TxTiltDeg    float64

	LegendDB     []float64
	LegendColors []string

	ComputedAt *time.Time
	ComputeMs  int32
	Note       string
	Bounds     *Bounds
	Job        Job

	ShareToken   string
	ShareExpires *time.Time

	ViewOpacity  int16
	ViewOverlays []int64
	ViewCircles  bool
}

const DefaultOpacity = 60

func (c Coverage) ComputeTime() string {
	ms := int64(c.ComputeMs)
	switch {
	case ms < 1000:
		return fmt.Sprintf("%d ms", ms)
	case ms < 60000:
		return fmt.Sprintf("%.1f s", float64(ms)/1000)
	}
	return fmt.Sprintf("%d min %d s", ms/60000, ms%60000/1000)
}

type Bounds struct {
	North, South, East, West float64
}

type CoverageWithSite struct {
	Coverage
	Site Site
}

const coverageQuery = `SELECT c.id, c.owner_id, c.name, c.site_id, c.frequency_mhz, c.tx_power_dbm, c.tx_gain_dbi, c.tx_line_loss_db,
	c.rx_height_m, c.rx_gain_dbi, c.rx_line_loss_db, c.rx_sensitivity_dbm, c.polarization, c.range_m, c.resolution_m,
	c.tx_antenna_id, c.tx_azimuth_deg, c.tx_tilt_deg, c.legend_db, c.legend_colors,
	c.computed_at, c.compute_ms, c.result_note, c.result_north, c.result_south, c.result_east, c.result_west,
	c.job_state, c.job_queued_at, c.job_started_at, c.job_worker, c.job_error,
	c.share_token, c.share_expires_at, c.view_opacity, c.view_overlays, c.view_circles,
	s.id, s.owner_id, s.name, ST_Y(s.position::geometry), ST_X(s.position::geometry), s.antenna_height_m
	FROM coverages c JOIN sites s ON s.id = c.site_id`

func scanCoverage(row interface{ Scan(...any) error }, c *CoverageWithSite) error {
	var ms *int32
	var note, worker, jobErr, token *string
	var opacity *int16
	var circles *bool
	var n, s, e, w *float64
	err := row.Scan(&c.ID, &c.OwnerID, &c.Name, &c.SiteID, &c.FreqMHz, &c.TxPowerDBm, &c.TxGainDBi, &c.TxLineLossDB,
		&c.RxHeightM, &c.RxGainDBi, &c.RxLineLossDB, &c.RxSensitivityDBm, &c.Polarization, &c.RangeM, &c.ResolutionM,
		&c.TxAntennaID, &c.TxAzimuthDeg, &c.TxTiltDeg, &c.LegendDB, &c.LegendColors,
		&c.ComputedAt, &ms, &note, &n, &s, &e, &w,
		&c.Job.State, &c.Job.QueuedAt, &c.Job.StartedAt, &worker, &jobErr,
		&token, &c.ShareExpires, &opacity, &c.ViewOverlays, &circles,
		&c.Site.ID, &c.Site.OwnerID, &c.Site.Name, &c.Site.Lat, &c.Site.Lon, &c.Site.AntennaHeightM)
	if err != nil {
		return err
	}
	if ms != nil {
		c.ComputeMs = *ms
	}
	if note != nil {
		c.Note = *note
	}
	if worker != nil {
		c.Job.Worker = *worker
	}
	if jobErr != nil {
		c.Job.Error = *jobErr
	}
	if token != nil {
		c.ShareToken = *token
	}
	c.ViewOpacity = DefaultOpacity
	if opacity != nil {
		c.ViewOpacity = *opacity
	}
	c.ViewCircles = circles == nil || *circles
	if n != nil {
		c.Bounds = &Bounds{North: *n, South: *s, East: *e, West: *w}
	}
	return nil
}

func (s *Store) ListCoverages(ctx context.Context, owner int64) ([]CoverageWithSite, error) {
	rows, err := s.pool.Query(ctx, coverageQuery+` WHERE c.owner_id = $1 ORDER BY c.name, c.id`, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CoverageWithSite
	for rows.Next() {
		var c CoverageWithSite
		if err := scanCoverage(rows, &c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) GetCoverage(ctx context.Context, owner, id int64) (CoverageWithSite, error) {
	var c CoverageWithSite
	err := scanCoverage(s.pool.QueryRow(ctx, coverageQuery+` WHERE c.owner_id = $1 AND c.id = $2`, owner, id), &c)
	return c, mapErr(err)
}

const coverageRefsOwned = `(SELECT owner_id FROM sites WHERE id = $3) = $1
	AND ($15::bigint IS NULL OR (SELECT owner_id FROM antennas WHERE id = $15) = $1)`

func (s *Store) CreateCoverage(ctx context.Context, c Coverage) (int64, error) {
	var n int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM coverages WHERE owner_id = $1`, c.OwnerID).Scan(&n); err != nil {
		return 0, err
	}
	if n >= MaxCoveragesPerUser {
		return 0, ErrQuota
	}
	var id int64
	err := s.pool.QueryRow(ctx, `INSERT INTO coverages (owner_id, name, site_id, frequency_mhz, tx_power_dbm, tx_gain_dbi, tx_line_loss_db,
		rx_height_m, rx_gain_dbi, rx_line_loss_db, rx_sensitivity_dbm, polarization, range_m, resolution_m,
		tx_antenna_id, tx_azimuth_deg, legend_db, legend_colors, tx_tilt_deg)
		SELECT $1::bigint, $2::text, $3::bigint, $4::float8, $5::float8, $6::float8, $7::float8, $8::float8, $9::float8, $10::float8,
		$11::float8, $12::smallint, $13::float8, $14::float8, $15::bigint, $16::float8, $17::float8[], $18::text[], $19::float8
		WHERE `+coverageRefsOwned+` RETURNING id`,
		c.OwnerID, c.Name, c.SiteID, c.FreqMHz, c.TxPowerDBm, c.TxGainDBi, c.TxLineLossDB,
		c.RxHeightM, c.RxGainDBi, c.RxLineLossDB, c.RxSensitivityDBm, c.Polarization, c.RangeM, c.ResolutionM,
		c.TxAntennaID, c.TxAzimuthDeg, c.LegendDB, c.LegendColors, c.TxTiltDeg).Scan(&id)
	return id, mapErr(err)
}

func (s *Store) UpdateCoverage(ctx context.Context, c Coverage) error {
	tag, err := s.pool.Exec(ctx, `UPDATE coverages SET name = $2, site_id = $3, frequency_mhz = $4, tx_power_dbm = $5,
		tx_gain_dbi = $6, tx_line_loss_db = $7, rx_height_m = $8, rx_gain_dbi = $9, rx_line_loss_db = $10,
		rx_sensitivity_dbm = $11, polarization = $12, range_m = $13, resolution_m = $14,
		tx_antenna_id = $15, tx_azimuth_deg = $16, tx_tilt_deg = $18, updated_at = now(),
		result_png = NULL, result_margin = NULL, result_north = NULL, result_south = NULL, result_east = NULL, result_west = NULL,
		computed_at = NULL, compute_ms = NULL, result_note = NULL,
		job_state = CASE WHEN job_state = 'running' THEN job_state ELSE 'idle' END, job_error = NULL
		WHERE owner_id = $1 AND id = $17 AND `+coverageRefsOwned,
		c.OwnerID, c.Name, c.SiteID, c.FreqMHz, c.TxPowerDBm, c.TxGainDBi, c.TxLineLossDB,
		c.RxHeightM, c.RxGainDBi, c.RxLineLossDB, c.RxSensitivityDBm, c.Polarization, c.RangeM, c.ResolutionM,
		c.TxAntennaID, c.TxAzimuthDeg, c.ID, c.TxTiltDeg)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteCoverage(ctx context.Context, owner, id int64) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM coverages WHERE owner_id = $1 AND id = $2`, owner, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) SetCoverageView(ctx context.Context, owner, id int64, opacity int16, overlays []int64, circles bool) error {
	tag, err := s.pool.Exec(ctx, `UPDATE coverages SET view_opacity = $3, view_circles = $5,
		view_overlays = (SELECT array_agg(x.id) FROM coverages x
			WHERE x.owner_id = $1 AND x.id <> $2 AND x.id = ANY($4::bigint[])
			AND x.computed_at IS NOT NULL AND x.result_north IS NOT NULL)
		WHERE owner_id = $1 AND id = $2`, owner, id, opacity, overlays, circles)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) CoveragesByID(ctx context.Context, owner int64, ids []int64) ([]CoverageWithSite, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := s.pool.Query(ctx, coverageQuery+` WHERE c.owner_id = $1 AND c.id = ANY($2::bigint[])
		AND c.computed_at IS NOT NULL AND c.result_north IS NOT NULL ORDER BY c.name, c.id`, owner, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CoverageWithSite
	for rows.Next() {
		var c CoverageWithSite
		if err := scanCoverage(rows, &c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

type Raster struct {
	ID           int64
	PNG, Margin  []byte
	Bounds       Bounds
	LegendDB     []float64
	LegendColors []string
}

const rasterQuery = `SELECT id, result_png, result_margin, result_north, result_south, result_east, result_west,
	legend_db, legend_colors FROM coverages WHERE owner_id = $1 AND result_png IS NOT NULL AND result_north IS NOT NULL`

func scanRaster(row interface{ Scan(...any) error }, r *Raster) error {
	return row.Scan(&r.ID, &r.PNG, &r.Margin, &r.Bounds.North, &r.Bounds.South, &r.Bounds.East, &r.Bounds.West,
		&r.LegendDB, &r.LegendColors)
}

func (s *Store) CoverageRasters(ctx context.Context, owner int64, ids []int64) ([]Raster, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := s.pool.Query(ctx, rasterQuery+` AND id = ANY($2::bigint[])`, owner, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Raster
	for rows.Next() {
		var r Raster
		if err := scanRaster(rows, &r); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) CoverageRaster(ctx context.Context, owner, id int64) (Raster, error) {
	var r Raster
	err := scanRaster(s.pool.QueryRow(ctx, rasterQuery+` AND id = $2`, owner, id), &r)
	return r, mapErr(err)
}

func (s *Store) SetCoverageLegend(ctx context.Context, owner, id int64, mins []float64, colors []string) error {
	tag, err := s.pool.Exec(ctx, `UPDATE coverages SET legend_db = $3, legend_colors = $4, updated_at = now()
		WHERE owner_id = $1 AND id = $2`, owner, id, mins, colors)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
