// Package pgutil holds small helpers shared by the Postgres repositories.
package pgutil

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
)

// Querier is satisfied by both *pgxpool.Pool and pgx.Tx.
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// TenantPools resolves a tenant db_key to its pool.
type TenantPools interface {
	Tenant(key string) (*pgxpool.Pool, error)
}

// TenantPool returns the pool of the logged-in caller's tenant.
func TenantPool(ctx context.Context, pools TenantPools) (*pgxpool.Pool, error) {
	p, err := domain.TenantPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	pool, err := pools.Tenant(p.DBKey)
	if err != nil {
		return nil, domain.Internal("Tenant database is not available", err)
	}
	return pool, nil
}

// UniqueViolation returns the constraint name if err is a unique violation.
func UniqueViolation(err error) (string, bool) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return pgErr.ConstraintName, true
	}
	return "", false
}

// IsNoRows reports whether err means the query found nothing.
func IsNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }

// NullString sends an empty string as SQL NULL.
func NullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
