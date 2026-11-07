package database

import (
	"context"
	"embed"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// migrationsTable lives in pms because this login cannot create schemas.
const migrationsTable = "pms.sys_schema_migrations"

func newProvider(pool *pgxpool.Pool) (*goose.Provider, func() error, error) {
	migrations, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		return nil, nil, err
	}
	db := stdlib.OpenDBFromPool(pool)
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations,
		goose.WithTableName(migrationsTable),
	)
	if err != nil {
		_ = db.Close()
		return nil, nil, err
	}
	return provider, db.Close, nil
}

// MigrateUp applies every pending platform migration and returns the versions applied.
func MigrateUp(ctx context.Context, pool *pgxpool.Pool) ([]int64, error) {
	provider, closeDB, err := newProvider(pool)
	if err != nil {
		return nil, fmt.Errorf("migrations: %w", err)
	}
	defer closeDB()

	results, err := provider.Up(ctx)
	if err != nil {
		return nil, fmt.Errorf("migrate up: %w", err)
	}
	applied := make([]int64, 0, len(results))
	for _, r := range results {
		applied = append(applied, r.Source.Version)
	}
	return applied, nil
}

// MigrationStatus describes one migration and whether it has run.
type MigrationStatus struct {
	Version int64
	Name    string
	Applied bool
}

// Status lists every known migration and whether it has been applied.
func Status(ctx context.Context, pool *pgxpool.Pool) ([]MigrationStatus, error) {
	provider, closeDB, err := newProvider(pool)
	if err != nil {
		return nil, fmt.Errorf("migrations: %w", err)
	}
	defer closeDB()

	statuses, err := provider.Status(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]MigrationStatus, 0, len(statuses))
	for _, s := range statuses {
		out = append(out, MigrationStatus{
			Version: s.Source.Version,
			Name:    s.Source.Path,
			Applied: s.State == goose.StateApplied,
		})
	}
	return out, nil
}
