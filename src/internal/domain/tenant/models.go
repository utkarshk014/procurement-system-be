package domain_tenant

import (
	"time"

	domain_auth "github.com/utkarshk014/procurement-system-be/src/internal/domain/auth"
)

// Status says whether a tenant's users may log in.
type Status string

const (
	StatusActive    Status = "active"
	StatusSuspended Status = "suspended"
)

// Tenant is one client organisation (e.g. ESIS); its business data lives in the DBKey database.
type Tenant struct {
	ID        int64     `json:"id"`
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	DBKey     string    `json:"db_key"`
	Status    Status    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateTenantInput is the body for creating a tenant.
type CreateTenantInput struct {
	Code  string `json:"code" validate:"required,min=2,max=20,alphanum"`
	Name  string `json:"name" validate:"required,max=200"`
	DBKey string `json:"db_key" validate:"required,max=50"`
}

// CreateTenantAdminInput is the body for adding an admin to a tenant.
type CreateTenantAdminInput = domain_auth.CreateUserInput
