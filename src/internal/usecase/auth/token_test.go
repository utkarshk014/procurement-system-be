package usecase_auth

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
	domain_auth "github.com/utkarshk014/procurement-system-be/src/internal/domain/auth"
)

const (
	accessSecret  = "access-secret-access-secret-access-secret"
	refreshSecret = "refresh-secret-refresh-secret-refresh-secret"
)

func principal() *domain.Principal {
	return &domain.Principal{UserID: 42, Email: "a@b.in", FullName: "Admin", Role: domain.RoleTenantAdmin,
		TenantID: 3, TenantCode: "ESIS", DBKey: "esis"}
}

func TestIssueAndVerifyAccessToken(t *testing.T) {
	iss := NewJWTIssuer(accessSecret, refreshSecret, "pms-api", 15*time.Minute, time.Hour)
	tok, claims, err := iss.Issue(principal(), domain_auth.TokenTypeAccess)
	require.NoError(t, err)

	got, err := iss.Verify(tok, domain_auth.TokenTypeAccess)
	require.NoError(t, err)
	assert.Equal(t, "42", got.Subject)
	assert.Equal(t, domain.RoleTenantAdmin, got.Role)
	assert.Equal(t, int64(3), got.TenantID)
	assert.Equal(t, "ESIS", got.TenantCode)
	assert.Equal(t, claims.ID, got.ID)
}

func TestRefreshTokenIsMinimal(t *testing.T) {
	iss := NewJWTIssuer(accessSecret, refreshSecret, "pms-api", time.Minute, time.Hour)
	_, claims, err := iss.Issue(principal(), domain_auth.TokenTypeRefresh)
	require.NoError(t, err)
	assert.Empty(t, claims.Email)
	assert.Zero(t, claims.TenantID)
}

func TestTokensCannotStandInForEachOther(t *testing.T) {
	iss := NewJWTIssuer(accessSecret, refreshSecret, "pms-api", time.Minute, time.Hour)
	access, _, _ := iss.Issue(principal(), domain_auth.TokenTypeAccess)
	refresh, _, _ := iss.Issue(principal(), domain_auth.TokenTypeRefresh)

	_, err := iss.Verify(refresh, domain_auth.TokenTypeAccess)
	assert.Error(t, err, "refresh token used as access token")
	_, err = iss.Verify(access, domain_auth.TokenTypeRefresh)
	assert.Error(t, err, "access token used as refresh token")
}

func TestExpiredTokenIsRejected(t *testing.T) {
	iss := NewJWTIssuer(accessSecret, refreshSecret, "pms-api", time.Minute, time.Hour).(*jwtIssuer)
	past := time.Now().Add(-2 * time.Hour)
	iss.now = func() time.Time { return past }
	tok, _, err := iss.Issue(principal(), domain_auth.TokenTypeAccess)
	require.NoError(t, err)

	iss.now = time.Now
	_, err = iss.Verify(tok, domain_auth.TokenTypeAccess)
	assert.Error(t, err)
}

func TestTamperedAndForeignTokensAreRejected(t *testing.T) {
	iss := NewJWTIssuer(accessSecret, refreshSecret, "pms-api", time.Minute, time.Hour)
	tok, _, _ := iss.Issue(principal(), domain_auth.TokenTypeAccess)

	parts := strings.Split(tok, ".")
	parts[2] = strings.Repeat("A", len(parts[2]))
	_, err := iss.Verify(strings.Join(parts, "."), domain_auth.TokenTypeAccess)
	assert.Error(t, err, "bad signature")

	other := NewJWTIssuer(accessSecret, refreshSecret, "someone-else", time.Minute, time.Hour)
	foreign, _, _ := other.Issue(principal(), domain_auth.TokenTypeAccess)
	_, err = iss.Verify(foreign, domain_auth.TokenTypeAccess)
	assert.Error(t, err, "wrong issuer")

	// "alg: none" must never be accepted.
	none := "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0." + parts[1] + "."
	_, err = iss.Verify(none, domain_auth.TokenTypeAccess)
	assert.Error(t, err)
}
