package usecase_auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
	domain_auth "github.com/utkarshk014/procurement-system-be/src/internal/domain/auth"
)

type jwtIssuer struct {
	accessSecret  []byte
	refreshSecret []byte
	issuer        string
	accessTTL     time.Duration
	refreshTTL    time.Duration
	now           func() time.Time
}

// NewJWTIssuer signs HS256 tokens; access and refresh use different secrets and a typ claim.
func NewJWTIssuer(accessSecret, refreshSecret, issuer string, accessTTL, refreshTTL time.Duration) domain_auth.TokenIssuer {
	return &jwtIssuer{
		accessSecret:  []byte(accessSecret),
		refreshSecret: []byte(refreshSecret),
		issuer:        issuer,
		accessTTL:     accessTTL,
		refreshTTL:    refreshTTL,
		now:           time.Now,
	}
}

func (j *jwtIssuer) secretAndTTL(typ domain_auth.TokenType) ([]byte, time.Duration, error) {
	switch typ {
	case domain_auth.TokenTypeAccess:
		return j.accessSecret, j.accessTTL, nil
	case domain_auth.TokenTypeRefresh:
		return j.refreshSecret, j.refreshTTL, nil
	default:
		return nil, 0, errors.New("unknown token type")
	}
}

func (j *jwtIssuer) Issue(p *domain.Principal, typ domain_auth.TokenType) (string, *domain_auth.Claims, error) {
	secret, ttl, err := j.secretAndTTL(typ)
	if err != nil {
		return "", nil, err
	}
	jti, err := newJTI()
	if err != nil {
		return "", nil, err
	}
	now := j.now()
	claims := &domain_auth.Claims{
		Type: typ,
		Role: p.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			Subject:   strconv.FormatInt(p.UserID, 10),
			Issuer:    j.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	// Only the access token carries profile details for the frontend; the
	// refresh token stays minimal.
	if typ == domain_auth.TokenTypeAccess {
		claims.Email = p.Email
		claims.Name = p.FullName
		claims.TenantID = p.TenantID
		claims.TenantCode = p.TenantCode
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
	if err != nil {
		return "", nil, err
	}
	return signed, claims, nil
}

// clockLeeway tolerates small clock differences between API instances.
const clockLeeway = 30 * time.Second

func (j *jwtIssuer) Verify(token string, typ domain_auth.TokenType) (*domain_auth.Claims, error) {
	secret, _, err := j.secretAndTTL(typ)
	if err != nil {
		return nil, err
	}
	claims := &domain_auth.Claims{}
	_, err = jwt.ParseWithClaims(token, claims,
		func(*jwt.Token) (any, error) { return secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(j.issuer),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(clockLeeway),
		jwt.WithTimeFunc(j.now),
	)
	if err != nil {
		return nil, err
	}
	if claims.Type != typ {
		return nil, errors.New("wrong token type")
	}
	return claims, nil
}

func newJTI() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
