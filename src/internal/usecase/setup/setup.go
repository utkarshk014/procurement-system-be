package usecase_setup

import (
	"context"

	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
	domain_company "github.com/utkarshk014/procurement-system-be/src/internal/domain/company"
	domain_finyear "github.com/utkarshk014/procurement-system-be/src/internal/domain/finyear"
	domain_setup "github.com/utkarshk014/procurement-system-be/src/internal/domain/setup"
)

type setupUsecase struct {
	finYears  domain_finyear.Repository
	companies domain_company.Repository
}

// NewSetupUsecase builds the setup-progress use case.
func NewSetupUsecase(finYears domain_finyear.Repository, companies domain_company.Repository) domain_setup.Usecase {
	return &setupUsecase{finYears: finYears, companies: companies}
}

func (uc *setupUsecase) Status(ctx context.Context) (*domain_setup.Status, error) {
	if _, err := domain.TenantPrincipal(ctx); err != nil {
		return nil, err
	}
	st := &domain_setup.Status{NextStep: domain_setup.StepFinancialYear}

	fy, err := uc.finYears.Current(ctx)
	switch {
	case err == nil:
		st.FinancialYearDone = true
		st.CurrentFinancialYear = fy.FinYr
		st.NextStep = domain_setup.StepCompany
	case !domain.IsKind(err, domain.KindNotFound):
		return nil, err
	}

	_, err = uc.companies.Get(ctx)
	switch {
	case err == nil:
		st.CompanyDone = true
	case !domain.IsKind(err, domain.KindNotFound):
		return nil, err
	}
	if st.FinancialYearDone && st.CompanyDone {
		st.NextStep = domain_setup.StepDone
	}
	return st, nil
}
