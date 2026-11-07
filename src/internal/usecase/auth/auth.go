package usecase_auth

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
	domain_auth "github.com/utkarshk014/procurement-system-be/src/internal/domain/auth"
	usecase_common "github.com/utkarshk014/procurement-system-be/src/internal/usecase/common"
	"github.com/utkarshk014/procurement-system-be/src/packages/security"
)

type authUsecase struct {
	repo   domain_auth.Repository
	tokens domain_auth.TokenIssuer
	logger *slog.Logger
}

// NewAuthUsecase builds the auth use case.
func NewAuthUsecase(repo domain_auth.Repository, tokens domain_auth.TokenIssuer, logger *slog.Logger) domain_auth.Usecase {
	return &authUsecase{repo: repo, tokens: tokens, logger: logger}
}

// errBadCredentials is the same for unknown email and wrong password.
var errBadCredentials = domain.Unauthorized("Email or password is incorrect")

func (uc *authUsecase) Login(ctx context.Context, in domain_auth.LoginInput, meta domain_auth.ClientMeta) (*domain_auth.TokenPair, error) {
	in.Email = strings.TrimSpace(in.Email)
	if err := usecase_common.ValidateStruct(in); err != nil {
		return nil, err
	}

	user, err := uc.repo.GetUserByEmail(ctx, in.Email)
	if err != nil {
		if domain.IsKind(err, domain.KindNotFound) {
			security.BurnPasswordCheck(in.Password)
			uc.logger.WarnContext(ctx, "login failed", "reason", "unknown_email", "ip", meta.IP)
			return nil, errBadCredentials
		}
		return nil, err
	}
	ok, err := security.CheckPassword(in.Password, user.PasswordHash)
	if err != nil {
		return nil, domain.Internal("Could not check password", err)
	}
	if !ok {
		uc.logger.WarnContext(ctx, "login failed", "reason", "wrong_password", "user_id", user.ID, "ip", meta.IP)
		return nil, errBadCredentials
	}

	// GetPrincipal also enforces "user active" and "tenant active".
	principal, err := uc.repo.GetPrincipal(ctx, user.ID)
	if err != nil {
		if domain.IsKind(err, domain.KindNotFound) {
			uc.logger.WarnContext(ctx, "login failed", "reason", "account_disabled", "user_id", user.ID, "ip", meta.IP)
			return nil, domain.Forbidden("This account is disabled")
		}
		return nil, err
	}

	pair, err := uc.issuePair(ctx, principal, meta)
	if err != nil {
		return nil, err
	}
	if err := uc.repo.TouchLastLogin(ctx, user.ID); err != nil {
		uc.logger.WarnContext(ctx, "could not record last login", "user_id", user.ID, "error", err)
	}
	uc.logger.InfoContext(ctx, "login succeeded", "user_id", user.ID, "role", principal.Role, "tenant", principal.TenantCode, "ip", meta.IP)
	return pair, nil
}

func (uc *authUsecase) Refresh(ctx context.Context, in domain_auth.RefreshInput, meta domain_auth.ClientMeta) (*domain_auth.TokenPair, error) {
	if err := usecase_common.ValidateStruct(in); err != nil {
		return nil, err
	}
	claims, err := uc.tokens.Verify(in.RefreshToken, domain_auth.TokenTypeRefresh)
	if err != nil {
		return nil, domain.Unauthorized("Session expired, please log in again")
	}
	// Consuming revokes the token: a refresh token works exactly once.
	userID, err := uc.repo.ConsumeRefreshToken(ctx, claims.ID)
	if err != nil {
		if domain.IsKind(err, domain.KindNotFound) {
			uc.revokeAllOnReuse(ctx, claims.ID, meta)
			return nil, domain.Unauthorized("Session expired, please log in again")
		}
		return nil, err
	}
	if strconv.FormatInt(userID, 10) != claims.Subject {
		return nil, domain.Unauthorized("Session expired, please log in again")
	}
	principal, err := uc.repo.GetPrincipal(ctx, userID)
	if err != nil {
		if domain.IsKind(err, domain.KindNotFound) {
			return nil, domain.Unauthorized("This account is disabled")
		}
		return nil, err
	}
	return uc.issuePair(ctx, principal, meta)
}

// refreshReuseGrace lets two tabs refresh at the same moment without being treated as theft.
const refreshReuseGrace = 30 * time.Second

// revokeAllOnReuse logs the user out everywhere when an already-used refresh token is replayed.
func (uc *authUsecase) revokeAllOnReuse(ctx context.Context, jti string, meta domain_auth.ClientMeta) {
	userID, err := uc.repo.RevokedBefore(ctx, jti, time.Now().Add(-refreshReuseGrace))
	if err != nil {
		if !domain.IsKind(err, domain.KindNotFound) {
			uc.logger.ErrorContext(ctx, "refresh reuse check failed", "error", err)
		}
		return
	}
	revoked, err := uc.repo.RevokeAllRefreshTokens(ctx, userID)
	if err != nil {
		uc.logger.ErrorContext(ctx, "could not revoke tokens after refresh reuse", "user_id", userID, "error", err)
		return
	}
	uc.logger.WarnContext(ctx, "refresh token reuse detected, all sessions revoked", "user_id", userID, "revoked", revoked, "ip", meta.IP)
}

func (uc *authUsecase) Logout(ctx context.Context, in domain_auth.RefreshInput) error {
	if err := usecase_common.ValidateStruct(in); err != nil {
		return err
	}
	claims, err := uc.tokens.Verify(in.RefreshToken, domain_auth.TokenTypeRefresh)
	if err != nil {
		return nil // an invalid or expired token is already logged out
	}
	return uc.repo.RevokeRefreshToken(ctx, claims.ID)
}

func (uc *authUsecase) Authenticate(ctx context.Context, accessToken string) (*domain.Principal, error) {
	claims, err := uc.tokens.Verify(accessToken, domain_auth.TokenTypeAccess)
	if err != nil {
		return nil, domain.Unauthorized("Invalid or expired token")
	}
	userID, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil {
		return nil, domain.Unauthorized("Invalid or expired token")
	}
	principal, err := uc.repo.GetPrincipal(ctx, userID)
	if err != nil {
		if domain.IsKind(err, domain.KindNotFound) {
			return nil, domain.Unauthorized("This account is disabled")
		}
		return nil, err
	}
	return principal, nil
}

func (uc *authUsecase) CreateUser(ctx context.Context, in domain_auth.CreateUserInput) (*domain_auth.User, error) {
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	in.FullName = strings.TrimSpace(in.FullName)
	if err := usecase_common.ValidateStruct(in); err != nil {
		return nil, err
	}
	if !in.Role.Valid() {
		return nil, domain.Internal("Invalid role", nil)
	}
	if (in.Role == domain.RolePlatformAdmin) != (in.TenantID == nil) {
		return nil, domain.Internal("Role and tenant do not match", nil)
	}

	hash, err := security.HashPassword(in.Password)
	if err != nil {
		return nil, domain.Internal("Could not hash password", err)
	}
	user := &domain_auth.User{
		Email:        in.Email,
		FullName:     in.FullName,
		PasswordHash: hash,
		Role:         in.Role,
		TenantID:     in.TenantID,
		IsActive:     true,
	}
	if err := uc.repo.CreateUser(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (uc *authUsecase) UpdateUser(ctx context.Context, in domain_auth.UpdateUserInput) (*domain_auth.User, error) {
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	in.NewEmail = strings.ToLower(strings.TrimSpace(in.NewEmail))
	in.FullName = strings.TrimSpace(in.FullName)
	if err := usecase_common.ValidateStruct(in); err != nil {
		return nil, err
	}
	if in.NewEmail == "" && in.FullName == "" && in.Password == "" {
		return nil, domain.InvalidParams("Nothing to change: give a new email, name or password")
	}

	user, err := uc.repo.GetUserByEmail(ctx, in.Email)
	if err != nil {
		return nil, err
	}
	emailChanged := in.NewEmail != "" && in.NewEmail != user.Email
	if emailChanged {
		user.Email = in.NewEmail
	}
	endSessions := emailChanged
	if in.FullName != "" {
		user.FullName = in.FullName
	}
	if in.Password != "" {
		if user.PasswordHash, err = security.HashPassword(in.Password); err != nil {
			return nil, domain.Internal("Could not hash password", err)
		}
		endSessions = true
	}
	if err := uc.repo.UpdateUser(ctx, user); err != nil {
		return nil, err
	}

	// New credentials must not leave the old ones' sessions alive.
	revoked := int64(0)
	if endSessions {
		if revoked, err = uc.repo.RevokeAllRefreshTokens(ctx, user.ID); err != nil {
			return nil, err
		}
	}
	uc.logger.InfoContext(ctx, "user updated", "user_id", user.ID,
		"email_changed", emailChanged, "name_changed", in.FullName != "", "password_changed", in.Password != "",
		"sessions_revoked", revoked)
	return user, nil
}

func (uc *authUsecase) issuePair(ctx context.Context, p *domain.Principal, meta domain_auth.ClientMeta) (*domain_auth.TokenPair, error) {
	access, accessClaims, err := uc.tokens.Issue(p, domain_auth.TokenTypeAccess)
	if err != nil {
		return nil, domain.Internal("Could not issue access token", err)
	}
	refresh, refreshClaims, err := uc.tokens.Issue(p, domain_auth.TokenTypeRefresh)
	if err != nil {
		return nil, domain.Internal("Could not issue refresh token", err)
	}
	if err := uc.repo.StoreRefreshToken(ctx, &domain_auth.RefreshToken{
		JTI:       refreshClaims.ID,
		UserID:    p.UserID,
		IP:        meta.IP,
		UserAgent: meta.UserAgent,
		ExpiresAt: refreshClaims.ExpiresAt.Time,
	}); err != nil {
		return nil, err
	}
	return &domain_auth.TokenPair{
		AccessToken:      access,
		RefreshToken:     refresh,
		TokenType:        "Bearer",
		ExpiresIn:        secondsUntil(accessClaims.ExpiresAt.Time),
		RefreshExpiresIn: secondsUntil(refreshClaims.ExpiresAt.Time),
		User:             domain_auth.MeFromPrincipal(p),
	}, nil
}

func secondsUntil(t time.Time) int64 {
	return int64(time.Until(t).Round(time.Second) / time.Second)
}
