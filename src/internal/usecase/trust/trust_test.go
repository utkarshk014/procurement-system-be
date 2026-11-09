package usecase_trust

import (
	"context"
	"io"
	"log/slog"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
	domain_finyear "github.com/utkarshk014/procurement-system-be/src/internal/domain/finyear"
	domain_trust "github.com/utkarshk014/procurement-system-be/src/internal/domain/trust"
)

// ---- fakes ------------------------------------------------------------------

type fakeRepo struct {
	trusts   map[string]domain_trust.Trust
	function domain_trust.FunctionResult
}

func newRepo() *fakeRepo {
	return &fakeRepo{trusts: map[string]domain_trust.Trust{}, function: domain_trust.FunctionResult{Valid: true, Message: "Validation successful"}}
}

func (f *fakeRepo) List(context.Context) ([]domain_trust.Trust, error) {
	out := []domain_trust.Trust{}
	for _, t := range f.trusts {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out, nil
}
func (f *fakeRepo) Get(_ context.Context, code string) (*domain_trust.Trust, error) {
	t, ok := f.trusts[code]
	if !ok {
		return nil, domain_trust.ErrNotFound
	}
	return &t, nil
}
func (f *fakeRepo) DescTaken(_ context.Context, desc, except string) (string, bool, error) {
	for code, t := range f.trusts {
		if code != except && strings.EqualFold(strings.TrimSpace(t.Desc), strings.TrimSpace(desc)) {
			return code, true, nil
		}
	}
	return "", false, nil
}
func (f *fakeRepo) CheckWithFunction(context.Context, *domain_trust.Trust) (*domain_trust.FunctionResult, error) {
	r := f.function
	return &r, nil
}
func (f *fakeRepo) Create(ctx context.Context, t *domain_trust.Trust) error {
	if _, ok := f.trusts[t.Code]; ok {
		return domain_trust.ErrCodeTaken
	}
	if other, taken, _ := f.DescTaken(ctx, t.Desc, t.Code); taken {
		return domain_trust.DescTakenError(other)
	}
	f.trusts[t.Code] = *t
	return nil
}
func (f *fakeRepo) Update(ctx context.Context, t *domain_trust.Trust) error {
	if _, ok := f.trusts[t.Code]; !ok {
		return domain_trust.ErrNotFound
	}
	if other, taken, _ := f.DescTaken(ctx, t.Desc, t.Code); taken {
		return domain_trust.DescTakenError(other)
	}
	f.trusts[t.Code] = *t
	return nil
}

type fakeFinYears struct{ domain_finyear.Repository }

func (fakeFinYears) Get(_ context.Context, finYr string) (*domain_finyear.FinancialYear, error) {
	if finYr == "2026-2027" {
		return &domain_finyear.FinancialYear{FinYr: "2026-2027", StartDate: "2026-04-01", EndDate: "2027-03-31"}, nil
	}
	return nil, domain.NotFound("Financial year not found")
}

func tenantCtx() context.Context {
	return domain.WithPrincipal(context.Background(), &domain.Principal{
		UserID: 7, Role: domain.RoleTenantAdmin, TenantID: 1, TenantCode: "ESIS", DBKey: "esis",
	})
}

func newUC(repo *fakeRepo) domain_trust.Usecase {
	return NewTrustUsecase(repo, fakeFinYears{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func sat() domain_trust.CreateInput {
	return domain_trust.CreateInput{Code: " sat ", Desc: " Sri Adichunchanagiri Trust ", StartFY: "2026-2027", StartDate: "2026-04-01"}
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

// ---- tests ------------------------------------------------------------------

func TestCreateNormalisesAndSaves(t *testing.T) {
	repo := newRepo()
	got, err := newUC(repo).Create(tenantCtx(), sat())
	require.NoError(t, err)
	assert.Equal(t, "SAT", got.Code)
	assert.Equal(t, "Sri Adichunchanagiri Trust", got.Desc)
	assert.Equal(t, "2026-2027", got.StartFY)
}

func TestCreateReportsEveryFieldProblemAtOnce(t *testing.T) {
	_, err := newUC(newRepo()).Create(tenantCtx(), domain_trust.CreateInput{Code: "S4T", Desc: strings.Repeat("d", 101), StartFY: "2030-2031", StartDate: "01-04-2026"})
	assert.True(t, domain.IsKind(err, domain.KindValidation))
	f := fieldsOf(err)
	assert.Equal(t, "must be exactly 3 letters, e.g. SAT", f["trust_code"])
	assert.Equal(t, "must be at most 100 characters", f["trust_desc"])
	assert.Equal(t, "must be an existing financial year", f["wrk_st_finyr"])
	assert.Equal(t, "must be a date in YYYY-MM-DD format", f["wrk_st_date"])

	_, err = newUC(newRepo()).Create(tenantCtx(), domain_trust.CreateInput{})
	f = fieldsOf(err)
	for _, field := range []string{"trust_code", "trust_desc", "wrk_st_finyr", "wrk_st_date"} {
		assert.Equal(t, "is required", f[field], field)
	}
}

func TestStartDateMustFallInsideTheStartYear(t *testing.T) {
	in := sat()
	in.StartDate = "2027-04-01"
	_, err := newUC(newRepo()).Create(tenantCtx(), in)
	assert.Equal(t, "must fall inside 2026-2027 (2026-04-01 to 2027-03-31)", fieldsOf(err)["wrk_st_date"])

	in.StartDate = "2027-03-31"
	_, err = newUC(newRepo()).Create(tenantCtx(), in)
	assert.NoError(t, err, "31 March is the last day of the year")
}

func TestDuplicatesAreConflictsOnTheirField(t *testing.T) {
	repo := newRepo()
	uc := newUC(repo)
	_, err := uc.Create(tenantCtx(), sat())
	require.NoError(t, err)

	_, err = uc.Create(tenantCtx(), sat())
	assert.True(t, domain.IsKind(err, domain.KindConflict))
	assert.Equal(t, "is already taken", fieldsOf(err)["trust_code"])

	other := sat()
	other.Code = "ACU"
	other.Desc = "SRI ADICHUNCHANAGIRI TRUST"
	_, err = uc.Create(tenantCtx(), other)
	assert.True(t, domain.IsKind(err, domain.KindConflict))
	assert.Equal(t, "is already used by trust SAT", fieldsOf(err)["trust_desc"])

	_, err = uc.Validate(tenantCtx(), sat())
	assert.Equal(t, "is already taken", fieldsOf(err)["trust_code"], "validate runs the same duplicate checks")
	_, err = uc.Validate(tenantCtx(), other)
	assert.Equal(t, "is already used by trust SAT", fieldsOf(err)["trust_desc"])
}

func TestValidateMapsTheFunctionMessage(t *testing.T) {
	repo := newRepo()
	repo.function = domain_trust.FunctionResult{Valid: false, Message: "Invalid financial year. Please select from mst_fin_year."}
	_, err := newUC(repo).Validate(tenantCtx(), sat())
	assert.True(t, domain.IsKind(err, domain.KindValidation))
	assert.Equal(t, "Invalid financial year. Please select from mst_fin_year", fieldsOf(err)["wrk_st_finyr"])

	repo.function = domain_trust.FunctionResult{Valid: true, Message: "Validation successful"}
	got, err := newUC(repo).Validate(tenantCtx(), sat())
	require.NoError(t, err)
	assert.Equal(t, "SAT", got.Code)
	assert.Empty(t, repo.trusts, "validate writes nothing")
}

func TestUpdate(t *testing.T) {
	repo := newRepo()
	uc := newUC(repo)
	_, err := uc.Create(tenantCtx(), sat())
	require.NoError(t, err)

	got, err := uc.Update(tenantCtx(), "sat", domain_trust.UpdateInput{Desc: "Sri Adichunchanagiri Shikshana Trust", StartFY: "2026-2027", StartDate: "2026-06-01"})
	require.NoError(t, err)
	assert.Equal(t, "SAT", got.Code, "the path code is upper-cased")
	assert.Equal(t, "Sri Adichunchanagiri Shikshana Trust", got.Desc)

	_, err = uc.Update(tenantCtx(), "SAT", domain_trust.UpdateInput{Desc: "sri adichunchanagiri shikshana trust", StartFY: "2026-2027", StartDate: "2026-06-01"})
	assert.NoError(t, err, "a trust may keep its own description in another case")

	_, err = uc.Update(tenantCtx(), "XYZ", domain_trust.UpdateInput{Desc: "X", StartFY: "2026-2027", StartDate: "2026-06-01"})
	assert.Equal(t, domain_trust.ErrNotFound, err)
	_, err = uc.Update(tenantCtx(), "toolong", domain_trust.UpdateInput{Desc: "X", StartFY: "2026-2027", StartDate: "2026-06-01"})
	assert.Equal(t, domain_trust.ErrNotFound, err)

	_, err = uc.Update(tenantCtx(), "SAT", domain_trust.UpdateInput{Desc: "", StartFY: "2030-2031", StartDate: "2026-06-01"})
	f := fieldsOf(err)
	assert.Equal(t, "is required", f["trust_desc"])
	assert.Equal(t, "must be an existing financial year", f["wrk_st_finyr"])
}

func TestGetAndList(t *testing.T) {
	repo := newRepo()
	uc := newUC(repo)
	list, err := uc.List(tenantCtx())
	require.NoError(t, err)
	assert.NotNil(t, list)
	assert.Empty(t, list)

	_, err = uc.Create(tenantCtx(), sat())
	require.NoError(t, err)
	got, err := uc.Get(tenantCtx(), "sat")
	require.NoError(t, err)
	assert.Equal(t, "SAT", got.Code)
	_, err = uc.Get(tenantCtx(), "abc1")
	assert.Equal(t, domain_trust.ErrNotFound, err)
}

func TestTrustsNeedTenantAccount(t *testing.T) {
	_, err := newUC(newRepo()).List(context.Background())
	assert.True(t, domain.IsKind(err, domain.KindUnauthorized))
}
