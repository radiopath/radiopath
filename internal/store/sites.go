package store

import "context"

type Site struct {
	ID             int64
	OwnerID        int64
	Name           string
	Lat, Lon       float64
	AntennaHeightM float64
}

const siteCols = `id, owner_id, name, ST_Y(position::geometry), ST_X(position::geometry), antenna_height_m`

func (s *Store) ListSites(ctx context.Context, owner int64) ([]Site, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+siteCols+` FROM sites WHERE owner_id = $1 ORDER BY name, id`, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Site
	for rows.Next() {
		var st Site
		if err := rows.Scan(&st.ID, &st.OwnerID, &st.Name, &st.Lat, &st.Lon, &st.AntennaHeightM); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

func (s *Store) GetSite(ctx context.Context, owner, id int64) (Site, error) {
	var st Site
	err := s.pool.QueryRow(ctx, `SELECT `+siteCols+` FROM sites WHERE owner_id = $1 AND id = $2`, owner, id).
		Scan(&st.ID, &st.OwnerID, &st.Name, &st.Lat, &st.Lon, &st.AntennaHeightM)
	return st, mapErr(err)
}

func (s *Store) CountSites(ctx context.Context, owner int64) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM sites WHERE owner_id = $1`, owner).Scan(&n)
	return n, err
}

func (s *Store) CreateSite(ctx context.Context, st Site) (int64, error) {
	n, err := s.CountSites(ctx, st.OwnerID)
	if err != nil {
		return 0, err
	}
	if n >= MaxSitesPerUser {
		return 0, ErrQuota
	}
	var id int64
	err = s.pool.QueryRow(ctx, `INSERT INTO sites (owner_id, name, position, antenna_height_m)
		VALUES ($1, $2, ST_SetSRID(ST_MakePoint($3, $4), 4326)::geography, $5) RETURNING id`,
		st.OwnerID, st.Name, st.Lon, st.Lat, st.AntennaHeightM).Scan(&id)
	return id, err
}

func (s *Store) UpdateSite(ctx context.Context, st Site) error {
	tag, err := s.pool.Exec(ctx, `UPDATE sites SET name = $3,
		position = ST_SetSRID(ST_MakePoint($4, $5), 4326)::geography,
		antenna_height_m = $6, updated_at = now() WHERE owner_id = $1 AND id = $2`,
		st.OwnerID, st.ID, st.Name, st.Lon, st.Lat, st.AntennaHeightM)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteSite(ctx context.Context, owner, id int64) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM sites WHERE owner_id = $1 AND id = $2`, owner, id)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
