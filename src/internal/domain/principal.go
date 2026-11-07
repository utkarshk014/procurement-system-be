package domain

import "context"

// Role is what a logged-in user is allowed to do.
type Role string

const (
	// RolePlatformAdmin runs the platform: creates tenants and their admins.
	RolePlatformAdmin Role = "platform_admin"
	// RoleTenantAdmin runs one tenant: company setup, masters, users.
	RoleTenantAdmin Role = "tenant_admin"
)

// Valid reports whether r is a known role.
func (r Role) Valid() bool {
	return r == RolePlatformAdmin || r == RoleTenantAdmin
}

// Principal is the authenticated caller, resolved once per request from the access token.
type Principal struct {
	UserID   int64
	Email    string
	FullName string
	Role     Role

	// Tenant fields are empty for platform admins.
	TenantID   int64
	TenantCode string
	TenantName string
	DBKey      string
}

// HasTenant reports whether the caller belongs to a tenant.
func (p *Principal) HasTenant() bool { return p != nil && p.TenantID != 0 }

type principalKey struct{}

// WithPrincipal stores the caller in ctx.
func WithPrincipal(ctx context.Context, p *Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// PrincipalFrom returns the caller stored in ctx, or nil.
func PrincipalFrom(ctx context.Context) *Principal {
	p, _ := ctx.Value(principalKey{}).(*Principal)
	return p
}

// TenantPrincipal returns the caller only if they belong to a tenant.
func TenantPrincipal(ctx context.Context) (*Principal, error) {
	p := PrincipalFrom(ctx)
	if p == nil {
		return nil, Unauthorized("Please log in")
	}
	if !p.HasTenant() {
		return nil, Forbidden("This action needs a tenant account")
	}
	return p, nil
}
