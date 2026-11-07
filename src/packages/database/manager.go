// Package database owns the Postgres pools: one control pool and one per tenant db_key (same URL, same pool).
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
	control *pgxpool.Pool
	tenants map[string]*pgxpool.Pool
	closers []*pgxpool.Pool
	once    sync.Once
}

// NewManager opens the control pool and every tenant pool and pings each one.
func NewManager(ctx context.Context, controlURL string, tenantURLs map[string]string) (*Manager, error) {
	byURL := map[string]*pgxpool.Pool{}
	m := &Manager{tenants: map[string]*pgxpool.Pool{}}

	open := func(url string) (*pgxpool.Pool, error) {
		if p, ok := byURL[url]; ok {
			return p, nil
		}
		p, err := newPool(ctx, url)
		if err != nil {
			return nil, err
		}
		byURL[url] = p
		m.closers = append(m.closers, p)
		return p, nil
	}

	control, err := open(controlURL)
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
	}
	return m, nil
}

func newPool(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}
	cfg.MaxConns = 10
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.HealthCheckPeriod = time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return pool, nil
}

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
