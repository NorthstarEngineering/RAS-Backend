package branch

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestQAMapError(t *testing.T) {
	unknown := errors.New("connection reset")
	wrapper := errors.New("wrapped: " + pgx.ErrNoRows.Error())
	tests := []struct {
		name string
		in   error
		want error
	}{
		{name: "nil remains nil"},
		{name: "no rows", in: pgx.ErrNoRows, want: ErrNotFound},
		{name: "wrapped no rows", in: errors.Join(wrapper, pgx.ErrNoRows), want: ErrNotFound},
		{name: "unique violation", in: &pgconn.PgError{Code: pgUniqueViolation}, want: ErrNameTaken},
		{name: "string too long", in: &pgconn.PgError{Code: pgStringTooLong}, want: ErrNameTooLong},
		{name: "numeric out of range", in: &pgconn.PgError{Code: pgNumericOutOfRange}, want: ErrLateRateTooLarge},
		{name: "unknown postgres error unchanged", in: &pgconn.PgError{Code: "08006"}},
		{name: "non-postgres error unchanged", in: unknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapError(tt.in)
			if tt.want == nil {
				if tt.in == nil && got != nil {
					t.Fatalf("mapError(nil) = %v, want nil", got)
				}
				if tt.in != nil && got != tt.in {
					t.Fatalf("mapError() = %v, want original error %v", got, tt.in)
				}
				return
			}
			if !errors.Is(got, tt.want) {
				t.Fatalf("mapError() = %v, want error matching %v", got, tt.want)
			}
		})
	}
}
