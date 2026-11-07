package domain_tenant

import (
	"context"

	domain_auth "github.com/utkarshk014/procurement-system-be/src/internal/domain/auth"
)

// Usecase is the tenant onboarding logic, used by platform admins.
type Usecase interface {
	Create(ctx context.Context, in CreateTenantInput) (*Tenant, error)
	List(ctx context.Context) ([]Tenant, error)
	CreateAdmin(ctx context.Context, tenantID int64, in CreateTenantAdminInput) (*domain_auth.User, error)
}

// Repository stores tenants in the control database.
type Repository interface {
	Create(ctx context.Context, t *Tenant) error
	List(ctx context.Context) ([]Tenant, error)
	GetByID(ctx context.Context, id int64) (*Tenant, error)
}

// DatabaseDirectory answers whether a db_key is wired to a real database.
type DatabaseDirectory interface {
	HasTenantKey(key string) bool
}
