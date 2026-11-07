package domain_auth

import (
	"context"

	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
)

// Usecase is the auth business logic the delivery layer calls.
type Usecase interface {
	Login(ctx context.Context, in LoginInput, meta ClientMeta) (*TokenPair, error)
	// Refresh swaps a refresh token for a new pair. The old refresh token is
	// revoked, so each one works exactly once.
	Refresh(ctx context.Context, in RefreshInput, meta ClientMeta) (*TokenPair, error)
	Logout(ctx context.Context, in RefreshInput) error
	// Authenticate verifies an access token and re-loads the caller, so a
	// deactivated user or suspended tenant is locked out immediately.
	Authenticate(ctx context.Context, accessToken string) (*domain.Principal, error)
	// CreateUser validates the input, hashes the password and stores the user.
	CreateUser(ctx context.Context, in CreateUserInput) (*User, error)
}

// Repository stores users and refresh tokens in the control database.
type Repository interface {
	CreateUser(ctx context.Context, u *User) error
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	TouchLastLogin(ctx context.Context, userID int64) error
	// GetPrincipal returns an active user (in an active tenant, if any) as a
	// principal, or a NotFound error.
	GetPrincipal(ctx context.Context, userID int64) (*domain.Principal, error)

	StoreRefreshToken(ctx context.Context, t *RefreshToken) error
	// ConsumeRefreshToken revokes an unexpired, unrevoked refresh token and
	// returns its user id. It returns NotFound if the token cannot be used.
	ConsumeRefreshToken(ctx context.Context, jti string) (int64, error)
	RevokeRefreshToken(ctx context.Context, jti string) error
	DeleteExpiredRefreshTokens(ctx context.Context) (int64, error)
}

// TokenIssuer signs and verifies JWTs.
type TokenIssuer interface {
	// Issue signs a token of the given type for the principal and returns it
	// with its claims (jti, expiry).
	Issue(p *domain.Principal, typ TokenType) (string, *Claims, error)
	// Verify checks signature, issuer, expiry and type, and returns the claims.
	Verify(token string, typ TokenType) (*Claims, error)
}
