package store

import "context"

type Antenna struct {
	ID       int64
	OwnerID  int64
	Name     string
	GainDBi  float64
	FreqMHz  float64
	PatternH []float64
	PatternV []float64
	Source   string
}

const antennaCols = `id, owner_id, name, gain_dbi, coalesce(freq_mhz, 0), pattern_h, pattern_v, coalesce(source, '')`

func scanAntenna(row interface{ Scan(...any) error }, a *Antenna) error {
	return row.Scan(&a.ID, &a.OwnerID, &a.Name, &a.GainDBi, &a.FreqMHz, &a.PatternH, &a.PatternV, &a.Source)
}

func (s *Store) ListAntennas(ctx context.Context, owner int64) ([]Antenna, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+antennaCols+` FROM antennas WHERE owner_id = $1 ORDER BY name, id`, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Antenna
	for rows.Next() {
		var a Antenna
		if err := scanAntenna(rows, &a); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) GetAntenna(ctx context.Context, owner, id int64) (Antenna, error) {
	var a Antenna
	err := scanAntenna(s.pool.QueryRow(ctx, `SELECT `+antennaCols+` FROM antennas WHERE owner_id = $1 AND id = $2`, owner, id), &a)
	return a, mapErr(err)
}

func (s *Store) CreateAntenna(ctx context.Context, a Antenna) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `INSERT INTO antennas (owner_id, name, gain_dbi, freq_mhz, pattern_h, pattern_v, source)
		VALUES ($1, $2, $3, NULLIF($4, 0), $5, $6, NULLIF($7, '')) RETURNING id`,
		a.OwnerID, a.Name, a.GainDBi, a.FreqMHz, a.PatternH, a.PatternV, a.Source).Scan(&id)
	return id, err
}

func (s *Store) UpdateAntenna(ctx context.Context, a Antenna) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE antennas SET name = $3, gain_dbi = $4, freq_mhz = NULLIF($5, 0), pattern_h = $6, pattern_v = $7
		WHERE owner_id = $1 AND id = $2`,
		a.OwnerID, a.ID, a.Name, a.GainDBi, a.FreqMHz, a.PatternH, a.PatternV)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	for _, q := range []string{
		`UPDATE links SET tx_gain_dbi = $2 WHERE tx_antenna_id = $1`,
		`UPDATE links SET rx_gain_dbi = $2 WHERE rx_antenna_id = $1`,
		`UPDATE coverages SET tx_gain_dbi = $2 WHERE tx_antenna_id = $1`,
	} {
		if _, err := tx.Exec(ctx, q, a.ID, a.GainDBi); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) DeleteAntenna(ctx context.Context, owner, id int64) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM antennas WHERE owner_id = $1 AND id = $2`, owner, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
