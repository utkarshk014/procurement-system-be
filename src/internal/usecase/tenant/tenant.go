package usecase_tenant

import (
	"context"
	"log/slog"
	"strings"

	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
	domain_auth "github.com/utkarshk014/procurement-system-be/src/internal/domain/auth"
	domain_tenant "github.com/utkarshk014/procurement-system-be/src/internal/domain/tenant"
	usecase_common "github.com/utkarshk014/procurement-system-be/src/internal/usecase/common"
)

type tenantUsecase struct {
	repo      domain_tenant.Repository
	auth      domain_auth.Usecase
	databases domain_tenant.DatabaseDirectory
	logger    *slog.Logger
}

// NewTenantUsecase builds tenant onboarding for platform admins.
func NewTenantUsecase(repo domain_tenant.Repository, auth domain_auth.Usecase, databases domain_tenant.DatabaseDirectory, logger *slog.Logger) domain_tenant.Usecase {
	return &tenantUsecase{repo: repo, auth: auth, databases: databases, logger: logger}
}

func (uc *tenantUsecase) Create(ctx context.Context, in domain_tenant.CreateTenantInput) (*domain_tenant.Tenant, error) {
	in.Code = strings.ToUpper(strings.TrimSpace(in.Code))
	in.Name = strings.TrimSpace(in.Name)
	in.DBKey = strings.TrimSpace(in.DBKey)
	if err := usecase_common.ValidateStruct(in); err != nil {
		return nil, err
	}
	if !uc.databases.HasTenantKey(in.DBKey) {
		return nil, domain.Validation(domain.FieldError{
			Field: "db_key", Message: "is not a configured database (see TENANT_DATABASES)",
		})
	}
	t := &domain_tenant.Tenant{
		Code:   in.Code,
		Name:   in.Name,
		DBKey:  in.DBKey,
		Status: domain_tenant.StatusActive,
	}
	if err := uc.repo.Create(ctx, t); err != nil {
		return nil, err
	}
	uc.logger.InfoContext(ctx, "tenant created", "tenant_id", t.ID, "tenant", t.Code, "db_key", t.DBKey, "by_user_id", actorID(ctx))
	return t, nil
}

func (uc *tenantUsecase) List(ctx context.Context) ([]domain_tenant.Tenant, error) {
	return uc.repo.List(ctx)
}

func (uc *tenantUsecase) CreateAdmin(ctx context.Context, tenantID int64, in domain_tenant.CreateTenantAdminInput) (*domain_auth.User, error) {
	t, err := uc.repo.GetByID(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if t.Status != domain_tenant.StatusActive {
		return nil, domain.Conflict("This tenant is suspended")
	}
	in.Role = domain.RoleTenantAdmin
	in.TenantID = &t.ID
	user, err := uc.auth.CreateUser(ctx, in)
	if err != nil {
		return nil, err
	}
	uc.logger.InfoContext(ctx, "tenant admin created", "tenant", t.Code, "user_id", user.ID, "by_user_id", actorID(ctx))
	return user, nil
}

// actorID is the logged-in user making the change, for audit logs.
func actorID(ctx context.Context) int64 {
	if p := domain.PrincipalFrom(ctx); p != nil {
		return p.UserID
	}
	return 0
}
