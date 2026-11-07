package usecase_company

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
	domain_company "github.com/utkarshk014/procurement-system-be/src/internal/domain/company"
	domain_finyear "github.com/utkarshk014/procurement-system-be/src/internal/domain/finyear"
)

func validInput() domain_company.CompanyInput {
	return domain_company.CompanyInput{
		Name:         "ESIS Educational Society",
		Address:      "12 MG Road",
		City:         "Bengaluru",
		State:        "karnataka",
		Pin:          "560001",
		Mobile:       "+91 98765-43210",
		Email:        "Office@ESIS.in",
		Website:      "https://esis.in/",
		ShortName:    "ESIS",
		FinStartDate: "2026-04-01",
	}
}

func fieldsOf(err error) map[string]string {
	ae, _ := domain.AsAppError(err)
	out := map[string]string{}
	if ae != nil {
		for _, f := range ae.Fields {
			out[f.Field] = f.Message
		}
	}
	return out
}

func checkFields(in domain_company.CompanyInput) map[string]string {
	out := map[string]string{}
	for _, e := range Check(Normalize(in)) {
		out[e.Field] = e.Message
	}
	return out
}

func TestNormalize(t *testing.T) {
	n := Normalize(validInput())
	assert.Equal(t, "9876543210", n.Mobile)
	assert.Equal(t, "office@esis.in", n.Email)
	assert.Equal(t, "https://esis.in", n.Website)
	assert.Equal(t, "Karnataka", n.State)
	assert.Equal(t, "India", n.Country)
}

func TestCheckAcceptsValidInput(t *testing.T) {
	assert.Empty(t, Check(Normalize(validInput())))
}

func TestCheckReportsEveryProblemAtOnce(t *testing.T) {
	in := validInput()
	in.Name = ""
	in.ShortName = ""
	in.Pin = "5600"
	in.Mobile = "12345"
	in.Email = "abc@"
	in.Website = "not a site"
	in.FinStartDate = "2026-08-15"
	in.State = "Atlantis"

	f := checkFields(in)
	assert.Equal(t, "is required", f["name"])
	assert.Equal(t, "is required", f["shortname"])
	assert.Equal(t, "must be a 6-digit PIN code", f["pin"])
	assert.Equal(t, "must be a 10-digit Indian mobile number", f["mobile"])
	assert.Equal(t, "must be a valid email address", f["email"])
	assert.Equal(t, "must be a valid website, e.g. example.com", f["website"])
	assert.Equal(t, "must be 1 April (start of the Indian financial year)", f["fin_st_dt"])
	assert.Equal(t, "must be an Indian state or union territory", f["state"])
}

func TestCheckEnforcesColumnLimits(t *testing.T) {
	in := validInput()
	in.Name = strings.Repeat("N", 301)
	in.ShortName = strings.Repeat("S", 51)
	f := checkFields(in)
	assert.Equal(t, "must be at most 300 characters", f["name"])
	assert.Equal(t, "must be at most 50 characters", f["shortname"])
}

func TestCheckForeignCountrySkipsStateList(t *testing.T) {
	in := validInput()
	in.Country = "Nepal"
	in.State = "Bagmati"
	assert.Empty(t, Check(Normalize(in)))
}

func TestCheckBadDateFormat(t *testing.T) {
	in := validInput()
	in.FinStartDate = "01-04-2026"
	assert.Equal(t, "must be a date in YYYY-MM-DD format", checkFields(in)["fin_st_dt"])
}

var discard = slog.New(slog.NewTextHandler(io.Discard, nil))

// ---- fakes ------------------------------------------------------------------

type fakeRepo struct {
	stored     *domain_company.Company
	procResult domain_company.ProcedureResult
	saved      *domain_company.Company
}

func (f *fakeRepo) Get(context.Context) (*domain_company.Company, error) {
	if f.stored == nil {
		return nil, domain.NotFound("Company is not set up yet")
	}
	c := *f.stored
	return &c, nil
}

func (f *fakeRepo) CheckWithProcedure(context.Context, *domain_company.Company) (*domain_company.ProcedureResult, error) {
	r := f.procResult
	return &r, nil
}

func (f *fakeRepo) Save(_ context.Context, c *domain_company.Company) (bool, error) {
	created := f.stored == nil
	if !created {
		c.TimeZone = f.stored.TimeZone
	}
	f.saved = c
	cp := *c
	f.stored = &cp
	return created, nil
}

type fakeFinYears struct{ current *domain_finyear.FinancialYear }

func (f *fakeFinYears) List(context.Context) ([]domain_finyear.FinancialYear, error) { return nil, nil }
func (f *fakeFinYears) Get(context.Context, string) (*domain_finyear.FinancialYear, error) {
	return f.current, nil
}
func (f *fakeFinYears) Current(context.Context) (*domain_finyear.FinancialYear, error) {
	if f.current == nil {
		return nil, domain.NotFound("No current financial year")
	}
	return f.current, nil
}
func (f *fakeFinYears) CheckWithFunction(context.Context, *domain_finyear.FinancialYear) (string, error) {
	return "SUCCESS", nil
}
func (f *fakeFinYears) Create(context.Context, *domain_finyear.FinancialYear) error { return nil }

func withYear() *fakeFinYears {
	return &fakeFinYears{current: &domain_finyear.FinancialYear{
		FinYr: "2026-2027", StartDate: "2026-04-01", EndDate: "2027-03-31", IsCurrent: true,
	}}
}

func tenantCtx() context.Context {
	return domain.WithPrincipal(context.Background(), &domain.Principal{
		UserID: 7, Role: domain.RoleTenantAdmin, TenantID: 1, TenantCode: "ESIS", DBKey: "esis",
	})
}

// ---- use case ---------------------------------------------------------------

func TestSaveNeedsFinancialYearFirst(t *testing.T) {
	uc := NewCompanyUsecase(&fakeRepo{procResult: domain_company.ProcedureResult{Valid: true}}, &fakeFinYears{}, time.UTC, discard)
	_, _, err := uc.Save(tenantCtx(), validInput())
	assert.Equal(t, ErrFinancialYearFirst, err)
}

func TestFirstSaveCreatesSL1AndStampsTimeZone(t *testing.T) {
	repo := &fakeRepo{procResult: domain_company.ProcedureResult{Valid: true}}
	uc := NewCompanyUsecase(repo, withYear(), time.FixedZone("IST", 5*3600+1800), discard).(*companyUsecase)
	fixed := time.Date(2026, 11, 7, 4, 30, 0, 0, time.UTC)
	uc.now = func() time.Time { return fixed }

	c, created, err := uc.Save(tenantCtx(), validInput())
	require.NoError(t, err)
	assert.True(t, created)
	assert.Equal(t, 1, c.SL)
	require.NotNil(t, repo.saved.TimeZone)
	assert.True(t, repo.saved.TimeZone.Equal(fixed))
	_, offset := repo.saved.TimeZone.Zone()
	assert.Equal(t, 5*3600+1800, offset)
}

func TestSecondSaveUpdatesSameRecord(t *testing.T) {
	repo := &fakeRepo{procResult: domain_company.ProcedureResult{Valid: true}}
	uc := NewCompanyUsecase(repo, withYear(), time.UTC, discard)
	first, created, err := uc.Save(tenantCtx(), validInput())
	require.NoError(t, err)
	require.True(t, created)

	in := validInput()
	in.City = "Mysuru"
	second, created, err := uc.Save(tenantCtx(), in)
	require.NoError(t, err)
	assert.False(t, created)
	assert.Equal(t, 1, second.SL)
	assert.Equal(t, "Mysuru", second.City)
	assert.Equal(t, first.TimeZone, second.TimeZone)
}

func TestFinStartDateDefaultsToCurrentYear(t *testing.T) {
	repo := &fakeRepo{procResult: domain_company.ProcedureResult{Valid: true}}
	uc := NewCompanyUsecase(repo, withYear(), time.UTC, discard)
	in := validInput()
	in.FinStartDate = ""
	c, _, err := uc.Save(tenantCtx(), in)
	require.NoError(t, err)
	assert.Equal(t, "2026-04-01", c.FinStartDate)
}

func TestValidateMapsProcedureMessageToField(t *testing.T) {
	repo := &fakeRepo{procResult: domain_company.ProcedureResult{Valid: false, Message: "Invalid PIN code"}}
	uc := NewCompanyUsecase(repo, withYear(), time.UTC, discard)
	_, err := uc.Validate(tenantCtx(), validInput())
	assert.True(t, domain.IsKind(err, domain.KindValidation))
	assert.Equal(t, "Invalid PIN code", fieldsOf(err)["pin"])
}

func TestCompanyNeedsTenantAccount(t *testing.T) {
	uc := NewCompanyUsecase(&fakeRepo{}, withYear(), time.UTC, discard)

	_, err := uc.Get(context.Background())
	assert.True(t, domain.IsKind(err, domain.KindUnauthorized))

	platform := domain.WithPrincipal(context.Background(), &domain.Principal{UserID: 1, Role: domain.RolePlatformAdmin})
	_, err = uc.Get(platform)
	assert.True(t, domain.IsKind(err, domain.KindForbidden))
}
