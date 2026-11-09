// Package usecase wires every use case from the repositories and config.
package usecase

import (
	"log/slog"

	domain_auth "github.com/utkarshk014/procurement-system-be/src/internal/domain/auth"
	domain_codelist "github.com/utkarshk014/procurement-system-be/src/internal/domain/codelist"
	domain_company "github.com/utkarshk014/procurement-system-be/src/internal/domain/company"
	domain_finyear "github.com/utkarshk014/procurement-system-be/src/internal/domain/finyear"
	domain_institution "github.com/utkarshk014/procurement-system-be/src/internal/domain/institution"
	domain_setup "github.com/utkarshk014/procurement-system-be/src/internal/domain/setup"
	domain_taxdetails "github.com/utkarshk014/procurement-system-be/src/internal/domain/taxdetails"
	domain_tenant "github.com/utkarshk014/procurement-system-be/src/internal/domain/tenant"
	domain_trust "github.com/utkarshk014/procurement-system-be/src/internal/domain/trust"
	"github.com/utkarshk014/procurement-system-be/src/internal/repository"
	usecase_auth "github.com/utkarshk014/procurement-system-be/src/internal/usecase/auth"
	usecase_codelist "github.com/utkarshk014/procurement-system-be/src/internal/usecase/codelist"
	usecase_company "github.com/utkarshk014/procurement-system-be/src/internal/usecase/company"
	usecase_finyear "github.com/utkarshk014/procurement-system-be/src/internal/usecase/finyear"
	usecase_institution "github.com/utkarshk014/procurement-system-be/src/internal/usecase/institution"
	usecase_setup "github.com/utkarshk014/procurement-system-be/src/internal/usecase/setup"
	usecase_taxdetails "github.com/utkarshk014/procurement-system-be/src/internal/usecase/taxdetails"
	usecase_tenant "github.com/utkarshk014/procurement-system-be/src/internal/usecase/tenant"
	usecase_trust "github.com/utkarshk014/procurement-system-be/src/internal/usecase/trust"
	"github.com/utkarshk014/procurement-system-be/src/utils"
)

// Usecases groups every use case the delivery layer calls.
type Usecases struct {
	Auth          domain_auth.Usecase
	Tenant        domain_tenant.Usecase
	Setup         domain_setup.Usecase
	FinancialYear domain_finyear.Usecase
	Company       domain_company.Usecase
	Trust         domain_trust.Usecase
	TaxDetails    domain_taxdetails.Usecase
	Places        domain_codelist.Usecase
	Branches      domain_codelist.Usecase
	Regions       domain_codelist.Usecase
	Institution   domain_institution.Usecase
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
		FinancialYear: usecase_finyear.NewFinYearUsecase(repos.FinancialYear, cfg.Location(), cfg.FYFutureYears, logger),
		Company:       usecase_company.NewCompanyUsecase(repos.Company, repos.FinancialYear, cfg.Location(), logger),
		Trust:         usecase_trust.NewTrustUsecase(repos.Trust, repos.FinancialYear, logger),
		TaxDetails:    usecase_taxdetails.NewTaxDetailsUsecase(repos.TaxDetails, cfg.Location(), logger),
		Places:        usecase_codelist.NewCodeListUsecase(domain_codelist.Place, repos.Places, logger),
		Branches:      usecase_codelist.NewCodeListUsecase(domain_codelist.Branch, repos.Branches, logger),
		Regions:       usecase_codelist.NewCodeListUsecase(domain_codelist.Region, repos.Regions, logger),
		Institution: usecase_institution.NewInstitutionUsecase(repos.Institution, usecase_institution.Lookups{
			Trusts: repos.Trust, Places: repos.Places, Branches: repos.Branches, Regions: repos.Regions,
			FinYears: repos.FinancialYear,
		}, cfg.Location(), logger),
	}
}
