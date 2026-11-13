// Package repository wires every repository implementation.
package repository

import (
	domain_auth "github.com/utkarshk014/procurement-system-be/src/internal/domain/auth"
	domain_codelist "github.com/utkarshk014/procurement-system-be/src/internal/domain/codelist"
	domain_company "github.com/utkarshk014/procurement-system-be/src/internal/domain/company"
	domain_finyear "github.com/utkarshk014/procurement-system-be/src/internal/domain/finyear"
	domain_institution "github.com/utkarshk014/procurement-system-be/src/internal/domain/institution"
	domain_taxdetails "github.com/utkarshk014/procurement-system-be/src/internal/domain/taxdetails"
	domain_tenant "github.com/utkarshk014/procurement-system-be/src/internal/domain/tenant"
	domain_trust "github.com/utkarshk014/procurement-system-be/src/internal/domain/trust"
	repository_auth "github.com/utkarshk014/procurement-system-be/src/internal/repository/auth"
	repository_codelist "github.com/utkarshk014/procurement-system-be/src/internal/repository/codelist"
	repository_company "github.com/utkarshk014/procurement-system-be/src/internal/repository/company"
	repository_finyear "github.com/utkarshk014/procurement-system-be/src/internal/repository/finyear"
	repository_institution "github.com/utkarshk014/procurement-system-be/src/internal/repository/institution"
	repository_taxdetails "github.com/utkarshk014/procurement-system-be/src/internal/repository/taxdetails"
	repository_tenant "github.com/utkarshk014/procurement-system-be/src/internal/repository/tenant"
	repository_trust "github.com/utkarshk014/procurement-system-be/src/internal/repository/trust"
	"github.com/utkarshk014/procurement-system-be/src/packages/database"
)

// Repositories groups every repository the use cases depend on.
type Repositories struct {
	Auth          domain_auth.Repository
	Tenant        domain_tenant.Repository
	FinancialYear domain_finyear.Repository
	Company       domain_company.Repository
	Trust         domain_trust.Repository
	TaxDetails    domain_taxdetails.Repository
	Places        domain_codelist.Repository
	Branches      domain_codelist.Repository
	Regions       domain_codelist.Repository
	ItemGroups    domain_codelist.Repository
	Categories    domain_codelist.Repository
	Manufacturers domain_codelist.Repository
	Institution   domain_institution.Repository
}

// NewRepositories puts platform data in the control DB and business data in each tenant's DB.
func NewRepositories(dbm *database.Manager) *Repositories {
	return &Repositories{
		Auth:          repository_auth.NewPostgresAuthRepository(dbm.Control()),
		Tenant:        repository_tenant.NewPostgresTenantRepository(dbm.Control()),
		FinancialYear: repository_finyear.NewPostgresFinYearRepository(dbm),
		Company:       repository_company.NewPostgresCompanyRepository(dbm),
		Trust:         repository_trust.NewPostgresTrustRepository(dbm),
		TaxDetails:    repository_taxdetails.NewPostgresTaxDetailsRepository(dbm),
		Places:        repository_codelist.NewPostgresCodeListRepository(dbm, domain_codelist.Place),
		Branches:      repository_codelist.NewPostgresCodeListRepository(dbm, domain_codelist.Branch),
		Regions:       repository_codelist.NewPostgresCodeListRepository(dbm, domain_codelist.Region),
		ItemGroups:    repository_codelist.NewPostgresCodeListRepository(dbm, domain_codelist.ItemGroup),
		Categories:    repository_codelist.NewPostgresCodeListRepository(dbm, domain_codelist.Category),
		Manufacturers: repository_codelist.NewPostgresCodeListRepository(dbm, domain_codelist.Manufacturer),
		Institution:   repository_institution.NewPostgresInstitutionRepository(dbm),
	}
}
