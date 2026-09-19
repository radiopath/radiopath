package store

import (
	"context"
	"time"
)

type Link struct {
	ID               int64
	OwnerID          int64
	Name             string
	SiteAID, SiteBID int64
	FreqMHz          float64
	TxPowerDBm       float64
	TxGainDBi        float64
	RxGainDBi        float64
	TxLineLossDB     float64
	RxLineLossDB     float64
	RxSensitivityDBm float64
	Polarization     int16

	TxAntennaID  *int64
	TxAzimuthDeg *float64
	TxTiltDeg    float64
	RxAntennaID  *int64
	RxAzimuthDeg *float64
	RxTiltDeg    float64

	ClutterLossDB float64
	MeasuredRxDBm *float64

	ShareToken   string
	ShareExpires *time.Time
}

type LinkWithSites struct {
	Link
	SiteA, SiteB Site
}

const linkCols = `l.id, l.owner_id, l.name, l.site_a_id, l.site_b_id, l.frequency_mhz, l.tx_power_dbm, l.tx_gain_dbi, l.rx_gain_dbi,
	l.tx_line_loss_db, l.rx_line_loss_db, l.rx_sensitivity_dbm, l.polarization,
	l.tx_antenna_id, l.tx_azimuth_deg, l.rx_antenna_id, l.rx_azimuth_deg, l.clutter_loss_db, l.measured_rx_dbm,
	l.share_token, l.share_expires_at, l.tx_tilt_deg, l.rx_tilt_deg`

func scanLink(row interface{ Scan(...any) error }, l *LinkWithSites) error {
	var token *string
	err := row.Scan(&l.ID, &l.OwnerID, &l.Name, &l.SiteAID, &l.SiteBID, &l.FreqMHz, &l.TxPowerDBm, &l.TxGainDBi, &l.RxGainDBi,
		&l.TxLineLossDB, &l.RxLineLossDB, &l.RxSensitivityDBm, &l.Polarization,
		&l.TxAntennaID, &l.TxAzimuthDeg, &l.RxAntennaID, &l.RxAzimuthDeg, &l.ClutterLossDB, &l.MeasuredRxDBm,
		&token, &l.ShareExpires, &l.TxTiltDeg, &l.RxTiltDeg,
		&l.SiteA.ID, &l.SiteA.OwnerID, &l.SiteA.Name, &l.SiteA.Lat, &l.SiteA.Lon, &l.SiteA.AntennaHeightM,
		&l.SiteB.ID, &l.SiteB.OwnerID, &l.SiteB.Name, &l.SiteB.Lat, &l.SiteB.Lon, &l.SiteB.AntennaHeightM)
	if err != nil {
		return err
	}
	if token != nil {
		l.ShareToken = *token
	}
	return nil
}

const linkQuery = `SELECT ` + linkCols + `,
	a.id, a.owner_id, a.name, ST_Y(a.position::geometry), ST_X(a.position::geometry), a.antenna_height_m,
	b.id, b.owner_id, b.name, ST_Y(b.position::geometry), ST_X(b.position::geometry), b.antenna_height_m
	FROM links l JOIN sites a ON a.id = l.site_a_id JOIN sites b ON b.id = l.site_b_id`

const linkRefsOwned = `(SELECT owner_id FROM sites WHERE id = $3) = $1
	AND (SELECT owner_id FROM sites WHERE id = $4) = $1
	AND ($13::bigint IS NULL OR (SELECT owner_id FROM antennas WHERE id = $13) = $1)
	AND ($15::bigint IS NULL OR (SELECT owner_id FROM antennas WHERE id = $15) = $1)`

func (s *Store) ListLinks(ctx context.Context, owner int64) ([]LinkWithSites, error) {
	rows, err := s.pool.Query(ctx, linkQuery+` WHERE l.owner_id = $1 ORDER BY l.name, l.id`, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LinkWithSites
	for rows.Next() {
		var l LinkWithSites
		if err := scanLink(rows, &l); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (s *Store) GetLink(ctx context.Context, owner, id int64) (LinkWithSites, error) {
	var l LinkWithSites
	err := scanLink(s.pool.QueryRow(ctx, linkQuery+` WHERE l.owner_id = $1 AND l.id = $2`, owner, id), &l)
	return l, mapErr(err)
}

func (s *Store) CreateLink(ctx context.Context, l Link) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `INSERT INTO links (owner_id, name, site_a_id, site_b_id, frequency_mhz, tx_power_dbm,
		tx_gain_dbi, rx_gain_dbi, tx_line_loss_db, rx_line_loss_db, rx_sensitivity_dbm, polarization,
		tx_antenna_id, tx_azimuth_deg, rx_antenna_id, rx_azimuth_deg, clutter_loss_db, measured_rx_dbm, tx_tilt_deg, rx_tilt_deg)
		SELECT $1::bigint, $2::text, $3::bigint, $4::bigint, $5::float8, $6::float8, $7::float8, $8::float8, $9::float8, $10::float8,
		$11::float8, $12::smallint, $13::bigint, $14::float8, $15::bigint, $16::float8, $17::float8, $18::float8, $19::float8, $20::float8
		WHERE `+linkRefsOwned+` RETURNING id`,
		l.OwnerID, l.Name, l.SiteAID, l.SiteBID, l.FreqMHz, l.TxPowerDBm, l.TxGainDBi, l.RxGainDBi,
		l.TxLineLossDB, l.RxLineLossDB, l.RxSensitivityDBm, l.Polarization,
		l.TxAntennaID, l.TxAzimuthDeg, l.RxAntennaID, l.RxAzimuthDeg, l.ClutterLossDB, l.MeasuredRxDBm, l.TxTiltDeg, l.RxTiltDeg).Scan(&id)
	return id, mapErr(err)
}

func (s *Store) UpdateLink(ctx context.Context, l Link) error {
	tag, err := s.pool.Exec(ctx, `UPDATE links SET name = $2, site_a_id = $3, site_b_id = $4, frequency_mhz = $5,
		tx_power_dbm = $6, tx_gain_dbi = $7, rx_gain_dbi = $8, tx_line_loss_db = $9, rx_line_loss_db = $10,
		rx_sensitivity_dbm = $11, polarization = $12, tx_antenna_id = $13, tx_azimuth_deg = $14,
		rx_antenna_id = $15, rx_azimuth_deg = $16, clutter_loss_db = $17, measured_rx_dbm = $18,
		tx_tilt_deg = $20, rx_tilt_deg = $21,
		updated_at = now() WHERE owner_id = $1 AND id = $19 AND `+linkRefsOwned,
		l.OwnerID, l.Name, l.SiteAID, l.SiteBID, l.FreqMHz, l.TxPowerDBm, l.TxGainDBi, l.RxGainDBi,
		l.TxLineLossDB, l.RxLineLossDB, l.RxSensitivityDBm, l.Polarization,
		l.TxAntennaID, l.TxAzimuthDeg, l.RxAntennaID, l.RxAzimuthDeg, l.ClutterLossDB, l.MeasuredRxDBm, l.ID,
		l.TxTiltDeg, l.RxTiltDeg)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteLink(ctx context.Context, owner, id int64) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM links WHERE owner_id = $1 AND id = $2`, owner, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
