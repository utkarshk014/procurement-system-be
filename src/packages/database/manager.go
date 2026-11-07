// Package database owns the Postgres pools: one control pool and one per tenant db_key (same database, same pool).
package database

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrUnknownKey is returned when a tenant points at a db_key that is not configured.
var ErrUnknownKey = errors.New("database key is not configured")

// Manager hands out connection pools.
type Manager struct {
	control     *pgxpool.Pool
	tenants     map[string]*pgxpool.Pool
	unreachable map[string]error
	closers     []*pgxpool.Pool
	once        sync.Once
}

// NewManager requires the control database to answer; tenant databases may be down and are retried per request.
func NewManager(ctx context.Context, controlURL string, tenantURLs map[string]string) (*Manager, error) {
	if err := CheckDistinctTenantDatabases(tenantURLs); err != nil {
		return nil, err
	}
	m := &Manager{tenants: map[string]*pgxpool.Pool{}, unreachable: map[string]error{}}
	byTarget := map[string]*pgxpool.Pool{}

	open := func(url string) (*pgxpool.Pool, error) {
		cfg, err := pgxpool.ParseConfig(url)
		if err != nil {
			return nil, fmt.Errorf("parse url: %w", err)
		}
		target := targetOf(cfg)
		if p, ok := byTarget[target]; ok {
			return p, nil
		}
		cfg.MaxConns = 10
		cfg.MaxConnIdleTime = 5 * time.Minute
		cfg.HealthCheckPeriod = time.Minute
		p, err := pgxpool.NewWithConfig(ctx, cfg) // connects lazily
		if err != nil {
			return nil, err
		}
		byTarget[target] = p
		m.closers = append(m.closers, p)
		return p, nil
	}

	control, err := open(controlURL)
	if err == nil {
		err = ping(ctx, control)
	}
	if err != nil {
		m.Close()
		return nil, fmt.Errorf("control database: %w", err)
	}
	m.control = control

	for key, url := range tenantURLs {
		p, err := open(url)
		if err != nil {
			m.Close()
			return nil, fmt.Errorf("tenant database %q: %w", key, err)
		}
		m.tenants[key] = p
		if err := ping(ctx, p); err != nil {
			m.unreachable[key] = err
		}
	}
	return m, nil
}

// CheckDistinctTenantDatabases rejects two db_keys on one database: business tables have no tenant column.
func CheckDistinctTenantDatabases(tenantURLs map[string]string) error {
	keyOf := map[string]string{}
	for key, url := range tenantURLs {
		cfg, err := pgxpool.ParseConfig(url)
		if err != nil {
			return fmt.Errorf("tenant database %q: parse url: %w", key, err)
		}
		target := targetOf(cfg)
		if other, dup := keyOf[target]; dup {
			return fmt.Errorf("tenant databases %q and %q point at the same database %s", other, key, target)
		}
		keyOf[target] = key
	}
	return nil
}

func targetOf(cfg *pgxpool.Config) string {
	return fmt.Sprintf("%s:%d/%s", cfg.ConnConfig.Host, cfg.ConnConfig.Port, cfg.ConnConfig.Database)
}

func ping(ctx context.Context, p *pgxpool.Pool) error {
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := p.Ping(pingCtx); err != nil {
		return fmt.Errorf("ping: %w", err)
	}
	return nil
}

// Unreachable lists tenant databases that did not answer at startup, for logging.
func (m *Manager) Unreachable() map[string]error { return m.unreachable }

// Control returns the pool for the platform tables.
func (m *Manager) Control() *pgxpool.Pool { return m.control }

// Tenant returns the pool for a tenant's db_key.
func (m *Manager) Tenant(key string) (*pgxpool.Pool, error) {
	p, ok := m.tenants[key]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownKey, key)
	}
	return p, nil
}

// HasTenantKey reports whether a db_key is configured.
func (m *Manager) HasTenantKey(key string) bool {
	_, ok := m.tenants[key]
	return ok
}

// Close closes every pool once.
func (m *Manager) Close() {
	m.once.Do(func() {
		for _, p := range m.closers {
			p.Close()
		}
	})
}
