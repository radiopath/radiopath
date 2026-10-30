package store

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// mapErr turns PostgreSQL error codes into the store's sentinel errors, also when wrapped.
func TestMapErr(t *testing.T) {
	pg := func(code string) error { return fmt.Errorf("exec: %w", &pgconn.PgError{Code: code}) }
	other := pg("42P01")
	for _, c := range []struct {
		in, want error
	}{
		{pgx.ErrNoRows, ErrNotFound},
		{pg("23001"), ErrInUse},
		{pg("23503"), ErrInUse},
		{pg("23505"), ErrExists},
		{other, other},
	} {
		if got := mapErr(c.in); !errors.Is(got, c.want) {
			t.Errorf("mapErr(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}
