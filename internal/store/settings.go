package store

import (
	"context"
	"errors"
)

const maintenanceKey = "maintenance"

func (s *Store) Maintenance(ctx context.Context) (bool, error) {
	var v string
	err := s.pool.QueryRow(ctx, `SELECT value FROM settings WHERE key = $1`, maintenanceKey).Scan(&v)
	if errors.Is(mapErr(err), ErrNotFound) {
		return false, nil
	}
	return v == "on", mapErr(err)
}

func (s *Store) SetMaintenance(ctx context.Context, on bool) error {
	v := "off"
	if on {
		v = "on"
	}
	_, err := s.pool.Exec(ctx, `INSERT INTO settings (key, value) VALUES ($1, $2)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()`, maintenanceKey, v)
	return err
}
