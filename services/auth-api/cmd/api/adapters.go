package main

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
)

// branchChecker satisfies auth.BranchChecker. Replace it with the branch
// module's service once that module exists.
type branchChecker struct{ db *sql.DB }

func (b branchChecker) BranchExists(ctx context.Context, id uuid.UUID) (bool, error) {
	var exists bool
	err := b.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM branches WHERE id = $1)`, id).Scan(&exists)
	return exists, err
}
