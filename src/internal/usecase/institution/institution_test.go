package usecase_institution

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
	domain_codelist "github.com/utkarshk014/procurement-system-be/src/internal/domain/codelist"
	domain_finyear "github.com/utkarshk014/procurement-system-be/src/internal/domain/finyear"
	domain_institution "github.com/utkarshk014/procurement-system-be/src/internal/domain/institution"
	domain_trust "github.com/utkarshk014/procurement-system-be/src/internal/domain/trust"
)

func fields(in domain_institution.Input) map[string]string {
	out := map[string]string{}
	for _, f := range Check(Normalize(in), "2026-11-09") {
		out[f.Field] = f.Message
	}
	return out
}

func valid() domain_institution.Input {
	return domain_institution.Input{
		TrustCode: "sat", Code: "sat-001", RegCode: "mys-01", Name: "BGS School",
		PlaceCode: "mys", BranchCode: "myb", FromFinYr: "2026-2027",
	}
}

func TestValidInputHasNoProblems(t *testing.T) {
	assert.Empty(t, fields(valid()))
}

func TestNormalizeTidiesEveryField(t *testing.T) {
	in := valid()
	in.Name, in.HeadMobile, in.AcntMobile, in.Email = "  BGS   School ", "+91 98450-12345", "", " Office@BGS.Test "
	n := Normalize(in)
	assert.Equal(t, "SAT", n.TrustCode)
	assert.Equal(t, "SAT-001", n.Code)
	assert.Equal(t, "MYS-01", n.RegCode)
	assert.Equal(t, "BGS School", n.Name)
	assert.Equal(t, "9845012345", n.HeadMobile)
	assert.Equal(t, "", n.AcntMobile, "an empty mobile stays empty")
	assert.Equal(t, "office@bgs.test", n.Email)
}

func TestCheckReportsEveryProblemAtOnce(t *testing.T) {
	in := valid()
	in.Code, in.RegCode, in.Name, in.PlaceCode = "ACU-001", "MYS-1", "", "M1S"
	in.RegDate, in.HeadMobile, in.LiasonEmail = "2026-11-10", "abc", "nope"
	f := fields(in)
	assert.Equal(t, "must start with the trust code SAT-", f["inst_code"])
	assert.Equal(t, "must be a region code, a dash and 2 digits, e.g. MYS-01", f["inst_reg_code"])
	assert.Equal(t, "is required", f["inst_name"])
	assert.Equal(t, "must be exactly 3 letters", f["place_code"])
	assert.Equal(t, "cannot be in the future", f["reg_date"])
	assert.Contains(t, f, "inst_head_mobile", "junk is rejected, not saved empty")
	assert.Contains(t, f, "inst_liason_email")
}

func TestRegDateTodayIsAllowed(t *testing.T) {
	in := valid()
	in.RegDate = "2026-11-09"
	assert.Empty(t, fields(in))
}

// ---- use case with fakes ---------------------------------------------------

type fakeInstRepo struct {
	domain_institution.Repository
	saved     []domain_institution.Institution
	taken     string
	function  domain_institution.FunctionResult
	code, reg int
}

func (f *fakeInstRepo) Taken(context.Context, *domain_institution.Institution) (string, string, error) {
	if f.taken != "" {
		return f.taken, "SAT-001", nil
	}
	return "", "", nil
}
func (f *fakeInstRepo) CheckWithFunction(context.Context, *domain_institution.Institution) (*domain_institution.FunctionResult, error) {
	r := f.function
	return &r, nil
}
func (f *fakeInstRepo) Create(_ context.Context, i *domain_institution.Institution) error {
	f.saved = append(f.saved, *i)
	return nil
}
func (f *fakeInstRepo) Get(_ context.Context, code string) (*domain_institution.Institution, error) {
	for _, i := range f.saved {
		if i.Code == code {
			return &i, nil
		}
	}
	return nil, domain_institution.ErrNotFound
}
func (f *fakeInstRepo) NextNumbers(context.Context, string, string) (int, int, error) {
	return f.code, f.reg, nil
}

type fakeTrusts struct{ domain_trust.Repository }

func (fakeTrusts) Get(_ context.Context, code string) (*domain_trust.Trust, error) {
	if code == "SAT" {
		return &domain_trust.Trust{Code: "SAT"}, nil
	}
	return nil, domain_trust.ErrNotFound
}

type fakeList struct {
	domain_codelist.Repository
	codes string
}

func (f fakeList) Get(_ context.Context, code string) (*domain_codelist.Entry, error) {
	if strings.Contains(f.codes, code) {
		return &domain_codelist.Entry{Code: code}, nil
	}
	return nil, domain_codelist.Place.NotFound()
}

type fakeYears struct{ domain_finyear.Repository }

func (fakeYears) Get(_ context.Context, fy string) (*domain_finyear.FinancialYear, error) {
	if fy == "2026-2027" {
		return &domain_finyear.FinancialYear{FinYr: fy}, nil
	}
	return nil, domain.NotFound("Financial year not found")
}

func newUC(repo *fakeInstRepo) *institutionUsecase {
	ist, _ := time.LoadLocation("Asia/Kolkata")
	uc := NewInstitutionUsecase(repo, Lookups{
		Trusts: fakeTrusts{}, Places: fakeList{codes: "MYS"}, Branches: fakeList{codes: "MYB"},
		Regions: fakeList{codes: "MYS"}, FinYears: fakeYears{},
	}, ist, slog.New(slog.NewTextHandler(io.Discard, nil))).(*institutionUsecase)
	uc.now = func() time.Time { return time.Date(2026, time.November, 9, 12, 0, 0, 0, ist) }
	return uc
}

func tenantCtx() context.Context {
	return domain.WithPrincipal(context.Background(), &domain.Principal{
		UserID: 7, Role: domain.RoleTenantAdmin, TenantID: 1, TenantCode: "ESIS", DBKey: "esis",
	})
}

func fieldsOf(err error) map[string]string {
	out := map[string]string{}
	if ae, ok := domain.AsAppError(err); ok {
		for _, f := range ae.Fields {
			out[f.Field] = f.Message
		}
	}
	return out
}

func TestCreateBuildsTheFullCodeAndDefaultsTheDate(t *testing.T) {
	repo := &fakeInstRepo{function: domain_institution.FunctionResult{Valid: true}}
	got, err := newUC(repo).Create(tenantCtx(), valid())
	require.NoError(t, err)
	assert.Equal(t, "SAT-001:MYS-01", got.FullCode)
	assert.Equal(t, "2026-11-09", got.RegDate, "today in India when not sent")
}

func TestEveryLookupMustExist(t *testing.T) {
	in := valid()
	in.TrustCode, in.Code, in.RegCode, in.PlaceCode, in.BranchCode, in.FromFinYr = "ACU", "ACU-001", "ZZZ-01", "BGN", "BGB", "1999-2000"
	_, err := newUC(&fakeInstRepo{}).Create(tenantCtx(), in)
	f := fieldsOf(err)
	assert.Equal(t, "must be an existing trust", f["trust_code"])
	assert.Equal(t, "must start with an existing region code (ZZZ is not one)", f["inst_reg_code"])
	assert.Equal(t, "must be an existing place", f["place_code"])
	assert.Equal(t, "must be an existing branch", f["branch_code"])
	assert.Equal(t, "must be an existing financial year", f["from_fin_yr"])
}

func TestBadShapeSkipsTheLookup(t *testing.T) {
	in := valid()
	in.RegCode = "MYS1"
	_, err := newUC(&fakeInstRepo{}).Create(tenantCtx(), in)
	assert.Equal(t, "must be a region code, a dash and 2 digits, e.g. MYS-01", fieldsOf(err)["inst_reg_code"])
}

func TestValidateReportsTakenCodesAndTheDBAsMessage(t *testing.T) {
	_, err := newUC(&fakeInstRepo{taken: "inst_reg_code"}).Validate(tenantCtx(), valid())
	assert.True(t, domain.IsKind(err, domain.KindConflict))
	assert.Equal(t, "is already used by SAT-001", fieldsOf(err)["inst_reg_code"])

	repo := &fakeInstRepo{function: domain_institution.FunctionResult{Message: "Institution code must be exactly 9 characters"}}
	_, err = newUC(repo).Validate(tenantCtx(), valid())
	assert.True(t, domain.IsKind(err, domain.KindValidation))
	assert.Equal(t, "Institution code must be exactly 9 characters", fieldsOf(err)["inst_code"])
	assert.Empty(t, repo.saved, "validate writes nothing")
}

func TestNextCodes(t *testing.T) {
	got, err := newUC(&fakeInstRepo{code: 413, reg: 7}).NextCodes(tenantCtx(), "sat", "mys")
	require.NoError(t, err)
	assert.Equal(t, &domain_institution.NextCodes{Code: "SAT-413", RegCode: "MYS-07"}, got)

	got, err = newUC(&fakeInstRepo{code: 1000, reg: 100}).NextCodes(tenantCtx(), "SAT", "MYS")
	require.NoError(t, err)
	assert.Equal(t, &domain_institution.NextCodes{}, got, "numbers used up: no suggestion")

	_, err = newUC(&fakeInstRepo{}).NextCodes(tenantCtx(), "S1T", "TOOLONG")
	f := fieldsOf(err)
	assert.Contains(t, f, "trust_code")
	assert.Contains(t, f, "regn_code")
}

func TestNeedsATenantAdmin(t *testing.T) {
	_, err := newUC(&fakeInstRepo{}).List(context.Background(), domain_institution.Filter{})
	assert.Error(t, err)
}

func TestListClampsPaging(t *testing.T) {
	repo := &pagingRepo{}
	uc := newUC(&fakeInstRepo{})
	uc.repo = repo
	_, _ = uc.List(tenantCtx(), domain_institution.Filter{Page: 0, PageSize: 5000, TrustCode: " sat "})
	assert.Equal(t, domain_institution.Filter{Page: 1, PageSize: domain_institution.MaxPageSize, TrustCode: "SAT"}, repo.got)
}

type pagingRepo struct {
	fakeInstRepo
	got domain_institution.Filter
}

func (p *pagingRepo) List(_ context.Context, f domain_institution.Filter) (*domain_institution.Page, error) {
	p.got = f
	return &domain_institution.Page{}, nil
}
