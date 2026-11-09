package usecase_finyear

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
	domain_finyear "github.com/utkarshk014/procurement-system-be/src/internal/domain/finyear"
)

type fakeRepo struct {
	years   []domain_finyear.FinancialYear
	created *domain_finyear.FinancialYear
	check   string
}

func (f *fakeRepo) List(context.Context) ([]domain_finyear.FinancialYear, error) { return f.years, nil }
func (f *fakeRepo) Get(_ context.Context, finYr string) (*domain_finyear.FinancialYear, error) {
	for i := range f.years {
		if f.years[i].FinYr == finYr {
			return &f.years[i], nil
		}
	}
	return nil, domain.NotFound("Financial year not found")
}
func (f *fakeRepo) Current(context.Context) (*domain_finyear.FinancialYear, error) {
	for i := range f.years {
		if f.years[i].IsCurrent {
			return &f.years[i], nil
		}
	}
	return nil, domain.NotFound("No current financial year")
}
func (f *fakeRepo) CheckWithFunction(context.Context, *domain_finyear.FinancialYear) (string, error) {
	if f.check == "" {
		return "SUCCESS", nil
	}
	return f.check, nil
}
func (f *fakeRepo) Create(ctx context.Context, fy *domain_finyear.FinancialYear, decideCurrent domain_finyear.CurrentRule) error {
	_, err := f.Current(ctx)
	fy.IsCurrent = decideCurrent(err == nil)
	f.created = fy
	if fy.IsCurrent {
		for i := range f.years {
			f.years[i].IsCurrent = false
		}
	}
	f.years = append(f.years, *fy)
	return nil
}

func (f *fakeRepo) SetCurrent(_ context.Context, finYr string) (bool, error) {
	idx := -1
	for i := range f.years {
		if f.years[i].FinYr == finYr {
			idx = i
		}
	}
	if idx < 0 {
		return false, domain.Validation(domain.FieldError{Field: "finyr", Message: "must be an existing financial year"})
	}
	if f.years[idx].IsCurrent {
		return false, nil
	}
	for i := range f.years {
		f.years[i].IsCurrent = i == idx
	}
	return true, nil
}

func ctx() context.Context {
	return domain.WithPrincipal(context.Background(), &domain.Principal{
		UserID: 7, Role: domain.RoleTenantAdmin, TenantID: 1, DBKey: "esis",
	})
}

// newAt builds the use case with "today" fixed, in IST, and the default of one future year.
func newAt(repo *fakeRepo, y int, m time.Month, d int) *finYearUsecase {
	return newWithFuture(repo, 1, y, m, d)
}

func newWithFuture(repo *fakeRepo, futureYears, y int, m time.Month, d int) *finYearUsecase {
	ist := time.FixedZone("IST", 5*3600+1800)
	uc := NewFinYearUsecase(repo, ist, futureYears, slog.New(slog.NewTextHandler(io.Discard, nil))).(*finYearUsecase)
	uc.now = func() time.Time { return time.Date(y, m, d, 12, 0, 0, 0, ist) }
	return uc
}

func fields(err error) map[string]string {
	out := map[string]string{}
	if ae, ok := domain.AsAppError(err); ok {
		for _, f := range ae.Fields {
			out[f.Field] = f.Message
		}
	}
	return out
}

func TestParseFinYr(t *testing.T) {
	y, ok := domain_finyear.ParseFinYr("2026-2027")
	assert.True(t, ok)
	assert.Equal(t, 2026, y)
	for _, bad := range []string{"abc", "2026-2028", "2026/2027", "2026-20270", "26-27", ""} {
		_, ok := domain_finyear.ParseFinYr(bad)
		assert.False(t, ok, bad)
	}
}

func TestStartYearFor(t *testing.T) {
	assert.Equal(t, 2026, domain_finyear.StartYearFor(time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)))
	assert.Equal(t, 2026, domain_finyear.StartYearFor(time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)))
	assert.Equal(t, 2025, domain_finyear.StartYearFor(time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)))
}

// Uncle's v1.01 example: on 02.11.2026 the choices are 2025-2026, 2026-2027 and the future 2027-2028.
func TestOptionsArePreviousCurrentThenFutureOldestFirst(t *testing.T) {
	uc := newAt(&fakeRepo{}, 2026, time.November, 2)
	opts, err := uc.Options(ctx())
	require.NoError(t, err)
	require.Len(t, opts, 3)
	assert.Equal(t, domain_finyear.Option{FinYr: "2025-2026", StartDate: "2025-04-01", EndDate: "2026-03-31", Kind: "previous"}, opts[0])
	assert.Equal(t, domain_finyear.Option{FinYr: "2026-2027", StartDate: "2026-04-01", EndDate: "2027-03-31", Kind: "current"}, opts[1])
	assert.Equal(t, domain_finyear.Option{FinYr: "2027-2028", StartDate: "2027-04-01", EndDate: "2028-03-31", Kind: "future"}, opts[2])
}

func TestOptionsFollowFutureYearsSetting(t *testing.T) {
	opts, err := newWithFuture(&fakeRepo{}, 0, 2026, time.November, 2).Options(ctx())
	require.NoError(t, err)
	require.Len(t, opts, 2, "0 future years: previous and current only")
	assert.Equal(t, "current", opts[1].Kind)

	opts, err = newWithFuture(&fakeRepo{}, 3, 2026, time.November, 2).Options(ctx())
	require.NoError(t, err)
	require.Len(t, opts, 5)
	assert.Equal(t, "2029-2030", opts[4].FinYr)
	assert.Equal(t, "future", opts[4].Kind)
}

func TestOptionsInFebruaryUseThePreviousCalendarYear(t *testing.T) {
	uc := newAt(&fakeRepo{}, 2027, time.February, 10)
	opts, err := uc.Options(ctx())
	require.NoError(t, err)
	assert.Equal(t, "2025-2026", opts[0].FinYr)
	assert.Equal(t, "2026-2027", opts[1].FinYr)
	assert.Equal(t, "current", opts[1].Kind)
	assert.Equal(t, "2027-2028", opts[2].FinYr)
}

func TestOptionsFlagExistingYears(t *testing.T) {
	repo := &fakeRepo{years: []domain_finyear.FinancialYear{{FinYr: "2026-2027", IsCurrent: true}}}
	opts, err := newAt(repo, 2026, time.November, 2).Options(ctx())
	require.NoError(t, err)
	assert.False(t, opts[0].Exists)
	assert.True(t, opts[1].Exists)
	assert.False(t, opts[2].Exists)
}

func TestFutureYearCanBeCreatedButIsNotCurrentByDefault(t *testing.T) {
	repo := &fakeRepo{years: []domain_finyear.FinancialYear{{FinYr: "2026-2027", IsCurrent: true}}}
	fy, err := newAt(repo, 2026, time.November, 2).Create(ctx(), domain_finyear.FinancialYearInput{FinYr: "2027-2028"})
	require.NoError(t, err)
	assert.Equal(t, "2027-04-01", fy.StartDate)
	assert.Equal(t, "2028-03-31", fy.EndDate)
	assert.False(t, fy.IsCurrent, "a future year is current only when asked")
	cur, err := repo.Current(ctx())
	require.NoError(t, err)
	assert.Equal(t, "2026-2027", cur.FinYr)

	yes := true
	fy, err = newAt(&fakeRepo{years: []domain_finyear.FinancialYear{{FinYr: "2026-2027", IsCurrent: true}}}, 2026, time.November, 2).
		Create(ctx(), domain_finyear.FinancialYearInput{FinYr: "2027-2028", IsCurrent: &yes})
	require.NoError(t, err)
	assert.True(t, fy.IsCurrent, "an explicit choice wins")
}

func TestFutureYearIsCurrentWhenNoYearIsYet(t *testing.T) {
	fy, err := newAt(&fakeRepo{}, 2026, time.November, 2).Create(ctx(), domain_finyear.FinancialYearInput{FinYr: "2027-2028"})
	require.NoError(t, err)
	assert.True(t, fy.IsCurrent, "the first year is always current")
}

func TestCreateFillsStandardDatesAndMakesFirstYearCurrent(t *testing.T) {
	repo := &fakeRepo{}
	fy, err := newAt(repo, 2026, time.November, 2).Create(ctx(), domain_finyear.FinancialYearInput{FinYr: "2026-2027"})
	require.NoError(t, err)
	assert.Equal(t, "2026-04-01", fy.StartDate)
	assert.Equal(t, "2027-03-31", fy.EndDate)
	assert.True(t, fy.IsCurrent, "first year becomes current")
}

func TestFirstYearIsCurrentEvenIfToldOtherwise(t *testing.T) {
	no := false
	fy, err := newAt(&fakeRepo{}, 2026, time.November, 2).Create(ctx(), domain_finyear.FinancialYearInput{FinYr: "2026-2027", IsCurrent: &no})
	require.NoError(t, err)
	assert.True(t, fy.IsCurrent, "with no current year the company step could never start")
}

// The review's click order: previous year first, then the current one, which must take over.
func TestCurrentYearBecomesCurrentEvenWhenPreviousWasCreatedFirst(t *testing.T) {
	repo := &fakeRepo{}
	uc := newAt(repo, 2026, time.November, 2)
	prev, err := uc.Create(ctx(), domain_finyear.FinancialYearInput{FinYr: "2025-2026"})
	require.NoError(t, err)
	assert.True(t, prev.IsCurrent, "first year is current while none is")

	cur, err := uc.Create(ctx(), domain_finyear.FinancialYearInput{FinYr: "2026-2027"})
	require.NoError(t, err)
	assert.True(t, cur.IsCurrent, "today's year becomes current by default")
	now, err := repo.Current(ctx())
	require.NoError(t, err)
	assert.Equal(t, "2026-2027", now.FinYr)
}

func TestPreviousYearAddedLaterStaysNotCurrent(t *testing.T) {
	repo := &fakeRepo{years: []domain_finyear.FinancialYear{{FinYr: "2026-2027", IsCurrent: true}}}
	fy, err := newAt(repo, 2026, time.November, 2).Create(ctx(), domain_finyear.FinancialYearInput{FinYr: "2025-2026"})
	require.NoError(t, err)
	assert.False(t, fy.IsCurrent)
}

func TestExplicitChoiceWinsOnceAYearIsCurrent(t *testing.T) {
	no, yes := false, true
	repo := &fakeRepo{years: []domain_finyear.FinancialYear{{FinYr: "2025-2026", IsCurrent: true}}}
	fy, err := newAt(repo, 2026, time.November, 2).Create(ctx(), domain_finyear.FinancialYearInput{FinYr: "2026-2027", IsCurrent: &no})
	require.NoError(t, err)
	assert.False(t, fy.IsCurrent)

	repo2 := &fakeRepo{years: []domain_finyear.FinancialYear{{FinYr: "2026-2027", IsCurrent: true}}}
	fy, err = newAt(repo2, 2026, time.November, 2).Create(ctx(), domain_finyear.FinancialYearInput{FinYr: "2025-2026", IsCurrent: &yes})
	require.NoError(t, err)
	assert.True(t, fy.IsCurrent)
}

func TestRejectsBadOrDisallowedYears(t *testing.T) {
	uc := newAt(&fakeRepo{}, 2026, time.November, 2)
	cases := map[string]string{
		"abc":        "must look like 2026-2027 (two consecutive years)",
		"2026-2028":  "must look like 2026-2027 (two consecutive years)",
		"2026-20270": "must look like 2026-2027 (two consecutive years)",
		"2028-2029":  "must be between 2025-2026 and 2027-2028",
		"2024-2025":  "must be between 2025-2026 and 2027-2028",
	}
	for in, want := range cases {
		_, err := uc.Create(ctx(), domain_finyear.FinancialYearInput{FinYr: in})
		assert.True(t, domain.IsKind(err, domain.KindValidation), in)
		assert.Equal(t, want, fields(err)["finyr"], in)
	}
	_, err := uc.Create(ctx(), domain_finyear.FinancialYearInput{})
	assert.Equal(t, "is required", fields(err)["finyr"])

	_, err = newWithFuture(&fakeRepo{}, 0, 2026, time.November, 2).Create(ctx(), domain_finyear.FinancialYearInput{FinYr: "2027-2028"})
	assert.Equal(t, "must be between 2025-2026 and 2026-2027", fields(err)["finyr"], "0 future years")
}

func TestRejectsNonStandardDates(t *testing.T) {
	uc := newAt(&fakeRepo{}, 2026, time.November, 2)
	_, err := uc.Create(ctx(), domain_finyear.FinancialYearInput{
		FinYr: "2026-2027", StartDate: "2026-05-01", EndDate: "2027-09-30",
	})
	f := fields(err)
	assert.Equal(t, "must be 2026-04-01 (1 April)", f["finyr_st_date"])
	assert.Equal(t, "must be 2027-03-31 (31 March)", f["finyr_end_date"])

	ok, err := uc.Validate(ctx(), domain_finyear.FinancialYearInput{
		FinYr: "2026-2027", StartDate: "2026-04-01", EndDate: "2027-03-31",
	})
	require.NoError(t, err)
	assert.Equal(t, "2026-2027", ok.FinYr)
}

func TestFunctionMessagesBecomeFieldErrors(t *testing.T) {
	uc := newAt(&fakeRepo{check: "Financial year already exists"}, 2026, time.November, 2)
	_, err := uc.Validate(ctx(), domain_finyear.FinancialYearInput{FinYr: "2026-2027"})
	assert.True(t, domain.IsKind(err, domain.KindConflict))
	assert.Equal(t, "Financial year already exists", fields(err)["finyr"])

	err = domain_finyear.FunctionError("Financial year 2026-2027 already exists")
	assert.True(t, domain.IsKind(err, domain.KindConflict))
	assert.Equal(t, "Financial year 2026-2027 already exists", fields(err)["finyr"])

	err = domain_finyear.FunctionError("Financial year end date cannot be before start date")
	assert.True(t, domain.IsKind(err, domain.KindValidation))
	assert.Contains(t, fields(err), "finyr_end_date")
}

func TestSetCurrentSwitchesTheOnlyCurrentYear(t *testing.T) {
	repo := &fakeRepo{years: []domain_finyear.FinancialYear{
		{FinYr: "2026-2027", IsCurrent: true}, {FinYr: "2027-2028"},
	}}
	uc := newAt(repo, 2026, time.November, 2)
	fy, err := uc.SetCurrent(ctx(), domain_finyear.SetCurrentInput{FinYr: " 2027-2028 "})
	require.NoError(t, err)
	assert.Equal(t, "2027-2028", fy.FinYr)
	assert.True(t, fy.IsCurrent)
	cur, err := repo.Current(ctx())
	require.NoError(t, err)
	assert.Equal(t, "2027-2028", cur.FinYr)
	assert.False(t, repo.years[0].IsCurrent, "the old one is un-marked")

	again, err := uc.SetCurrent(ctx(), domain_finyear.SetCurrentInput{FinYr: "2027-2028"})
	require.NoError(t, err, "already current is a no-op, not an error")
	assert.True(t, again.IsCurrent)
}

func TestSetCurrentRejectsBadOrUnknownYears(t *testing.T) {
	uc := newAt(&fakeRepo{years: []domain_finyear.FinancialYear{{FinYr: "2026-2027", IsCurrent: true}}}, 2026, time.November, 2)
	_, err := uc.SetCurrent(ctx(), domain_finyear.SetCurrentInput{})
	assert.Equal(t, "is required", fields(err)["finyr"])
	_, err = uc.SetCurrent(ctx(), domain_finyear.SetCurrentInput{FinYr: "2026-2028"})
	assert.Equal(t, "must look like 2026-2027 (two consecutive years)", fields(err)["finyr"])
	_, err = uc.SetCurrent(ctx(), domain_finyear.SetCurrentInput{FinYr: "2030-2031"})
	assert.True(t, domain.IsKind(err, domain.KindValidation))
	assert.Equal(t, "must be an existing financial year", fields(err)["finyr"])
}

func TestSetCurrentProcedureMessagesMapToFinyr(t *testing.T) {
	err := domain_finyear.FunctionError("Financial year 2030-2031 does not exist")
	assert.True(t, domain.IsKind(err, domain.KindValidation))
	assert.Equal(t, "Financial year 2030-2031 does not exist", fields(err)["finyr"])
}
