package repository_auth

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
	domain_auth "github.com/utkarshk014/procurement-system-be/src/internal/domain/auth"
	"github.com/utkarshk014/procurement-system-be/src/internal/repository/pgutil"
)

type postgresAuthRepository struct {
	db *pgxpool.Pool
}

// NewPostgresAuthRepository stores users and refresh tokens in the control database.
func NewPostgresAuthRepository(control *pgxpool.Pool) domain_auth.Repository {
	return &postgresAuthRepository{db: control}
}

const userColumns = `id, email, full_name, password_hash, role, tenant_id, is_active, last_login_at, created_at, updated_at`

func (r *postgresAuthRepository) CreateUser(ctx context.Context, u *domain_auth.User) error {
	err := r.db.QueryRow(ctx, `
		INSERT INTO pms.sys_user (email, full_name, password_hash, role, tenant_id, is_active)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at`,
		u.Email, u.FullName, u.PasswordHash, u.Role, u.TenantID, u.IsActive,
	).Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if _, ok := pgutil.UniqueViolation(err); ok {
			return domain.Conflict("A user with this email already exists",
				domain.FieldError{Field: "email", Message: "is already registered"})
		}
		return domain.Internal("Could not create user", err)
	}
	return nil
}

func (r *postgresAuthRepository) GetUserByEmail(ctx context.Context, email string) (*domain_auth.User, error) {
	u := &domain_auth.User{}
	err := r.db.QueryRow(ctx,
		`SELECT `+userColumns+` FROM pms.sys_user WHERE lower(email) = lower($1)`, email,
	).Scan(&u.ID, &u.Email, &u.FullName, &u.PasswordHash, &u.Role, &u.TenantID,
		&u.IsActive, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if pgutil.IsNoRows(err) {
			return nil, domain.NotFound("User not found")
		}
		return nil, domain.Internal("Could not load user", err)
	}
	return u, nil
}

func (r *postgresAuthRepository) TouchLastLogin(ctx context.Context, userID int64) error {
	if _, err := r.db.Exec(ctx,
		`UPDATE pms.sys_user SET last_login_at = now() WHERE id = $1`, userID); err != nil {
		return domain.Internal("Could not update last login", err)
	}
	return nil
}

func (r *postgresAuthRepository) GetPrincipal(ctx context.Context, userID int64) (*domain.Principal, error) {
	p := &domain.Principal{}
	err := r.db.QueryRow(ctx, `
		SELECT u.id, u.email, u.full_name, u.role,
		       COALESCE(t.id, 0), COALESCE(t.code, ''), COALESCE(t.name, ''), COALESCE(t.db_key, '')
		FROM pms.sys_user u
		LEFT JOIN pms.sys_tenant t ON t.id = u.tenant_id
		WHERE u.id = $1
		  AND u.is_active
		  AND (u.tenant_id IS NULL OR t.status = 'active')`, userID,
	).Scan(&p.UserID, &p.Email, &p.FullName, &p.Role,
		&p.TenantID, &p.TenantCode, &p.TenantName, &p.DBKey)
	if err != nil {
		if pgutil.IsNoRows(err) {
			return nil, domain.NotFound("Active user not found")
		}
		return nil, domain.Internal("Could not load user", err)
	}
	return p, nil
}

func (r *postgresAuthRepository) StoreRefreshToken(ctx context.Context, t *domain_auth.RefreshToken) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO pms.sys_refresh_token (jti, user_id, ip, user_agent, expires_at)
		VALUES ($1, $2, $3, $4, $5)`,
		t.JTI, t.UserID, pgutil.NullString(t.IP), pgutil.NullString(truncate(t.UserAgent, 300)), t.ExpiresAt)
	if err != nil {
		return domain.Internal("Could not store refresh token", err)
	}
	return nil
}

func (r *postgresAuthRepository) ConsumeRefreshToken(ctx context.Context, jti string) (int64, error) {
	var userID int64
	err := r.db.QueryRow(ctx, `
		UPDATE pms.sys_refresh_token
		SET revoked_at = now()
		WHERE jti = $1 AND revoked_at IS NULL AND expires_at > now()
		RETURNING user_id`, jti,
	).Scan(&userID)
	if err != nil {
		if pgutil.IsNoRows(err) {
			return 0, domain.NotFound("Refresh token is no longer valid")
		}
		return 0, domain.Internal("Could not use refresh token", err)
	}
	return userID, nil
}

func (r *postgresAuthRepository) RevokeRefreshToken(ctx context.Context, jti string) error {
	if _, err := r.db.Exec(ctx, `
		UPDATE pms.sys_refresh_token SET revoked_at = now()
		WHERE jti = $1 AND revoked_at IS NULL`, jti); err != nil {
		return domain.Internal("Could not revoke refresh token", err)
	}
	return nil
}

func (r *postgresAuthRepository) RevokedBefore(ctx context.Context, jti string, cutoff time.Time) (int64, error) {
	var userID int64
	err := r.db.QueryRow(ctx,
		`SELECT user_id FROM pms.sys_refresh_token WHERE jti = $1 AND revoked_at < $2`, jti, cutoff,
	).Scan(&userID)
	if err != nil {
		if pgutil.IsNoRows(err) {
			return 0, domain.NotFound("Refresh token was not revoked earlier")
		}
		return 0, domain.Internal("Could not check refresh token", err)
	}
	return userID, nil
}

func (r *postgresAuthRepository) RevokeAllRefreshTokens(ctx context.Context, userID int64) (int64, error) {
	tag, err := r.db.Exec(ctx, `
		UPDATE pms.sys_refresh_token SET revoked_at = now()
		WHERE user_id = $1 AND revoked_at IS NULL`, userID)
	if err != nil {
		return 0, domain.Internal("Could not revoke refresh tokens", err)
	}
	return tag.RowsAffected(), nil
}

func (r *postgresAuthRepository) DeleteExpiredRefreshTokens(ctx context.Context) (int64, error) {
	tag, err := r.db.Exec(ctx, `DELETE FROM pms.sys_refresh_token WHERE expires_at <= now()`)
	if err != nil {
		return 0, domain.Internal("Could not clean up refresh tokens", err)
	}
	return tag.RowsAffected(), nil
}

// truncate cuts s to n characters without splitting a multi-byte character.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
