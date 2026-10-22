// Package migrations embeds the *.up.sql files and applies the ones not yet applied.
package migrations

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"sort"
)

//go:embed *.up.sql
var files embed.FS

// Up applies every embedded *.up.sql file (in name order) that has not run yet.
func Up(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS inventory_schema_migrations (
		version    TEXT PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`); err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	entries, err := files.ReadDir(".")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)

	for _, name := range names {
		var done bool
		if err := db.QueryRowContext(ctx,
			`SELECT EXISTS (SELECT 1 FROM inventory_schema_migrations WHERE version = $1)`, name).Scan(&done); err != nil {
			return fmt.Errorf("check %s: %w", name, err)
		}
		if done {
			continue
		}

		body, err := files.ReadFile(name)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, string(body)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO inventory_schema_migrations (version) VALUES ($1)`, name); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record %s: %w", name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit %s: %w", name, err)
		}
	}
	return nil
}