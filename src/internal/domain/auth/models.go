package domain_auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
)

// TokenType tells access tokens and refresh tokens apart inside the JWT.
type TokenType string

const (
	TokenTypeAccess  TokenType = "access"
	TokenTypeRefresh TokenType = "refresh"
)

// Claims is the JWT payload; the frontend may decode it, the backend re-checks the user every request.
type Claims struct {
	Type                 TokenType   `json:"typ"`
	Email                string      `json:"email,omitempty"`
	Name                 string      `json:"name,omitempty"`
	Role                 domain.Role `json:"role"`
	TenantID             int64       `json:"tid,omitempty"`
	TenantCode           string      `json:"tcode,omitempty"`
	jwt.RegisteredClaims             // sub = user id, jti, iss, iat, exp
}

// User is a person who can log in.
type User struct {
	ID           int64       `json:"id"`
	Email        string      `json:"email"`
	FullName     string      `json:"full_name"`
	PasswordHash string      `json:"-"`
	Role         domain.Role `json:"role"`
	TenantID     *int64      `json:"tenant_id,omitempty"`
	IsActive     bool        `json:"is_active"`
	LastLoginAt  *time.Time  `json:"last_login_at,omitempty"`
	CreatedAt    time.Time   `json:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at"`
}

// RefreshToken is the stored record of an issued refresh token.
type RefreshToken struct {
	JTI       string
	UserID    int64
	IP        string
	UserAgent string
	ExpiresAt time.Time
}

// ClientMeta describes where a request came from.
type ClientMeta struct {
	IP        string
	UserAgent string
}

// LoginInput is the login request body.
type LoginInput struct {
	Email    string `json:"email" validate:"required,email,max=200"`
	Password string `json:"password" validate:"required,max=200"`
}

// RefreshInput carries a refresh token, for /auth/refresh and /auth/logout.
type RefreshInput struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// TokenPair is returned by login and refresh.
type TokenPair struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	TokenType        string `json:"token_type"`
	ExpiresIn        int64  `json:"expires_in"`
	RefreshExpiresIn int64  `json:"refresh_expires_in"`
	User             *Me    `json:"user"`
}

// CreateUserInput creates a login; Role and TenantID come from the calling use case, never the body.
type CreateUserInput struct {
	Email    string      `json:"email" validate:"required,email,max=200"`
	FullName string      `json:"full_name" validate:"required,max=200"`
	Password string      `json:"password" validate:"required,min=10,max=200"`
	Role     domain.Role `json:"-"`
	TenantID *int64      `json:"-"`
}

// UpdateUserInput finds a user by Email; empty fields stay unchanged.
type UpdateUserInput struct {
	Email    string `json:"email" validate:"required,email,max=200"`
	NewEmail string `json:"new_email" validate:"omitempty,email,max=200"`
	FullName string `json:"full_name" validate:"max=200"`
	Password string `json:"password" validate:"omitempty,min=10,max=200"`
}

// Me is the logged-in user as the frontend sees them.
type Me struct {
	ID       int64       `json:"id"`
	Email    string      `json:"email"`
	FullName string      `json:"full_name"`
	Role     domain.Role `json:"role"`
	Tenant   *MeTenant   `json:"tenant,omitempty"`
}

// MeTenant is the tenant summary inside Me.
type MeTenant struct {
	ID   int64  `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

// MeFromPrincipal builds the frontend view of the caller.
func MeFromPrincipal(p *domain.Principal) *Me {
	me := &Me{ID: p.UserID, Email: p.Email, FullName: p.FullName, Role: p.Role}
	if p.HasTenant() {
		me.Tenant = &MeTenant{ID: p.TenantID, Code: p.TenantCode, Name: p.TenantName}
	}
	return me
}
