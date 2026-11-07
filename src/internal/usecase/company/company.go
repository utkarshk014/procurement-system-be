package usecase_company

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
	domain_company "github.com/utkarshk014/procurement-system-be/src/internal/domain/company"
	domain_finyear "github.com/utkarshk014/procurement-system-be/src/internal/domain/finyear"
	usecase_common "github.com/utkarshk014/procurement-system-be/src/internal/usecase/common"
	"github.com/utkarshk014/procurement-system-be/src/packages/validation"
)

type companyUsecase struct {
	repo     domain_company.Repository
	finYears domain_finyear.Repository
	location *time.Location
	logger   *slog.Logger
	now      func() time.Time
}

// NewCompanyUsecase builds company setup; location is the zone stamped into time_zone on first save.
func NewCompanyUsecase(repo domain_company.Repository, finYears domain_finyear.Repository, location *time.Location, logger *slog.Logger) domain_company.Usecase {
	return &companyUsecase{repo: repo, finYears: finYears, location: location, logger: logger, now: time.Now}
}

// ErrFinancialYearFirst is returned when the company is saved before a current financial year exists.
var ErrFinancialYearFirst = domain.Conflict("Create the financial year first")

func (uc *companyUsecase) Get(ctx context.Context) (*domain_company.Company, error) {
	if _, err := domain.TenantPrincipal(ctx); err != nil {
		return nil, err
	}
	return uc.repo.Get(ctx)
}

func (uc *companyUsecase) Validate(ctx context.Context, in domain_company.CompanyInput) (*domain_company.Company, error) {
	if _, err := domain.TenantPrincipal(ctx); err != nil {
		return nil, err
	}
	c, err := uc.prepare(ctx, in)
	if err != nil {
		return nil, err
	}
	res, err := uc.repo.CheckWithProcedure(ctx, c)
	if err != nil {
		return nil, err
	}
	if !res.Valid {
		return nil, domain.Validation(domain_company.ProcedureFieldError(res.Message))
	}
	return c, nil
}

func (uc *companyUsecase) Save(ctx context.Context, in domain_company.CompanyInput) (*domain_company.Company, bool, error) {
	p, err := domain.TenantPrincipal(ctx)
	if err != nil {
		return nil, false, err
	}
	c, err := uc.prepare(ctx, in)
	if err != nil {
		return nil, false, err
	}
	// Used on first save only; the repository keeps the stored value on updates.
	stamp := uc.now().In(uc.location)
	c.TimeZone = &stamp

	created, err := uc.repo.Save(ctx, c)
	if err != nil {
		return nil, false, err
	}
	uc.logger.InfoContext(ctx, "company saved", "tenant", p.TenantCode, "user_id", p.UserID, "created", created)
	saved, err := uc.repo.Get(ctx)
	if err != nil {
		return nil, false, err
	}
	return saved, created, nil
}

// prepare requires a current year, normalises, defaults fin_st_dt to the year's start and checks every rule.
func (uc *companyUsecase) prepare(ctx context.Context, in domain_company.CompanyInput) (*domain_company.Company, error) {
	current, err := uc.finYears.Current(ctx)
	if err != nil {
		if domain.IsKind(err, domain.KindNotFound) {
			return nil, ErrFinancialYearFirst
		}
		return nil, err
	}

	in = Normalize(in)
	if in.FinStartDate == "" {
		in.FinStartDate = current.StartDate
	}
	if fields := Check(in); len(fields) > 0 {
		return nil, domain.Validation(fields...)
	}
	return &domain_company.Company{
		SL:           domain_company.SL,
		Name:         in.Name,
		Address:      in.Address,
		City:         in.City,
		State:        in.State,
		Country:      in.Country,
		Pin:          in.Pin,
		Mobile:       in.Mobile,
		Email:        in.Email,
		Website:      in.Website,
		Header1:      in.Header1,
		Header2:      in.Header2,
		ShortName:    in.ShortName,
		FinStartDate: in.FinStartDate,
	}, nil
}

// Normalize trims fields and stores mobile as 10 digits, email lowercase, country India by default, states in official spelling.
func Normalize(in domain_company.CompanyInput) domain_company.CompanyInput {
	trim := strings.TrimSpace
	in.Name = trim(in.Name)
	in.Address = trim(in.Address)
	in.City = trim(in.City)
	in.State = trim(in.State)
	in.Country = trim(in.Country)
	in.Pin = trim(in.Pin)
	in.Email = strings.ToLower(trim(in.Email))
	in.Website = validation.NormalizeWebsite(in.Website)
	in.Header1 = trim(in.Header1)
	in.Header2 = trim(in.Header2)
	in.ShortName = trim(in.ShortName)
	in.FinStartDate = trim(in.FinStartDate)

	if m := trim(in.Mobile); m != "" {
		in.Mobile = validation.NormalizeIndianMobile(m)
	} else {
		in.Mobile = ""
	}
	if in.Country == "" {
		in.Country = domain_company.DefaultCountry
	}
	if isIndia(in.Country) {
		in.Country = domain_company.DefaultCountry
		if st, ok := validation.CanonicalIndianState(in.State); ok {
			in.State = st
		}
	}
	return in
}

// Check returns every rule the normalised input breaks.
func Check(in domain_company.CompanyInput) []domain.FieldError {
	fields := usecase_common.ToFieldErrors(validation.Struct(in))
	hasError := map[string]bool{}
	for _, f := range fields {
		hasError[f.Field] = true
	}

	if isIndia(in.Country) && in.State != "" && !hasError["state"] {
		if _, ok := validation.CanonicalIndianState(in.State); !ok {
			fields = append(fields, domain.FieldError{Field: "state", Message: "must be an Indian state or union territory"})
		}
	}
	if in.FinStartDate != "" && !hasError["fin_st_dt"] {
		d, err := time.Parse(domain_company.DateLayout, in.FinStartDate)
		if err == nil && (d.Month() != domain_company.FinancialYearStartMonth || d.Day() != domain_company.FinancialYearStartDay) {
			fields = append(fields, domain.FieldError{Field: "fin_st_dt", Message: "must be 1 April (start of the Indian financial year)"})
		}
	}
	return fields
}

func isIndia(country string) bool {
	return strings.EqualFold(country, domain_company.DefaultCountry)
}
