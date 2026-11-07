// Package usecase wires every use case from the repositories and config.
package usecase

import (
	"log/slog"

	domain_auth "github.com/utkarshk014/procurement-system-be/src/internal/domain/auth"
	domain_company "github.com/utkarshk014/procurement-system-be/src/internal/domain/company"
	domain_finyear "github.com/utkarshk014/procurement-system-be/src/internal/domain/finyear"
	domain_setup "github.com/utkarshk014/procurement-system-be/src/internal/domain/setup"
	domain_tenant "github.com/utkarshk014/procurement-system-be/src/internal/domain/tenant"
	"github.com/utkarshk014/procurement-system-be/src/internal/repository"
	usecase_auth "github.com/utkarshk014/procurement-system-be/src/internal/usecase/auth"
	usecase_company "github.com/utkarshk014/procurement-system-be/src/internal/usecase/company"
	usecase_finyear "github.com/utkarshk014/procurement-system-be/src/internal/usecase/finyear"
	usecase_setup "github.com/utkarshk014/procurement-system-be/src/internal/usecase/setup"
	usecase_tenant "github.com/utkarshk014/procurement-system-be/src/internal/usecase/tenant"
	"github.com/utkarshk014/procurement-system-be/src/utils"
)

// Usecases groups every use case the delivery layer calls.
type Usecases struct {
	Auth          domain_auth.Usecase
	Tenant        domain_tenant.Usecase
	Setup         domain_setup.Usecase
	FinancialYear domain_finyear.Usecase
	Company       domain_company.Usecase
}

// NewUsecases builds the use cases.
func NewUsecases(cfg *utils.Config, repos *repository.Repositories, databases domain_tenant.DatabaseDirectory, logger *slog.Logger) *Usecases {
	tokens := usecase_auth.NewJWTIssuer(cfg.JWTAccessSecret, cfg.JWTRefreshSecret, cfg.JWTIssuer,
		cfg.AccessTokenTTL, cfg.RefreshTokenTTL)
	auth := usecase_auth.NewAuthUsecase(repos.Auth, tokens, logger)

	return &Usecases{
		Auth:          auth,
		Tenant:        usecase_tenant.NewTenantUsecase(repos.Tenant, auth, databases, logger),
		Setup:         usecase_setup.NewSetupUsecase(repos.FinancialYear, repos.Company),
		FinancialYear: usecase_finyear.NewFinYearUsecase(repos.FinancialYear, cfg.Location(), logger),
		Company:       usecase_company.NewCompanyUsecase(repos.Company, repos.FinancialYear, cfg.Location(), logger),
	}
}
