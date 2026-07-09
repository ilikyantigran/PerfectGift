package postgres

import (
	"context"
	"embed"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrate applies the embedded migrations using the repo's own pool.
func (r *Repo) Migrate(ctx context.Context) error { return Migrate(ctx, r.pool) }

// Migrate applies any not-yet-applied SQL migrations, tracked in
// identity.schema_migrations. It is safe to run on every boot.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	// The tracking table lives in the identity schema, which the first migration
	// creates — so ensure the schema exists before we record anything.
	if _, err := pool.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS identity`); err != nil {
		return fmt.Errorf("ensure schema: %w", err)
	}
	if _, err := pool.Exec(ctx,
		`CREATE TABLE IF NOT EXISTS identity.schema_migrations (version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())`,
	); err != nil {
		return fmt.Errorf("ensure migrations table: %w", err)
	}

	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		var exists bool
		if err := pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM identity.schema_migrations WHERE version=$1)`, name,
		).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}
		sqlBytes, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}
		if err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, string(sqlBytes)); err != nil {
				return fmt.Errorf("apply %s: %w", name, err)
			}
			_, err := tx.Exec(ctx, `INSERT INTO identity.schema_migrations (version) VALUES ($1)`, name)
			return err
		}); err != nil {
			return err
		}
	}
	return nil
}
