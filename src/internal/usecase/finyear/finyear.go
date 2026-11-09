package usecase_finyear

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
	domain_finyear "github.com/utkarshk014/procurement-system-be/src/internal/domain/finyear"
	usecase_common "github.com/utkarshk014/procurement-system-be/src/internal/usecase/common"
)

type finYearUsecase struct {
	repo        domain_finyear.Repository
	location    *time.Location
	futureYears int
	logger      *slog.Logger
	now         func() time.Time
}

// NewFinYearUsecase builds financial-year setup; location decides what "today" is and
// futureYears how many years after the current one may be created (FY_FUTURE_YEARS).
func NewFinYearUsecase(repo domain_finyear.Repository, location *time.Location, futureYears int, logger *slog.Logger) domain_finyear.Usecase {
	return &finYearUsecase{repo: repo, location: location, futureYears: futureYears, logger: logger, now: time.Now}
}

// allowedStartYears returns today's start year and the range that may be created:
// the previous year through futureYears after the current one.
func (uc *finYearUsecase) allowedStartYears() (current, first, last int) {
	current = domain_finyear.StartYearFor(uc.now().In(uc.location))
	return current, current - 1, current + uc.futureYears
}

func (uc *finYearUsecase) Options(ctx context.Context) ([]domain_finyear.Option, error) {
	if _, err := domain.TenantPrincipal(ctx); err != nil {
		return nil, err
	}
	existing, err := uc.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	have := map[string]bool{}
	for _, fy := range existing {
		have[fy.FinYr] = true
	}
	current, first, last := uc.allowedStartYears()
	options := make([]domain_finyear.Option, 0, last-first+1)
	for startYear := first; startYear <= last; startYear++ {
		kind := domain_finyear.KindFuture
		switch {
		case startYear < current:
			kind = domain_finyear.KindPrevious
		case startYear == current:
			kind = domain_finyear.KindCurrent
		}
		st, end := domain_finyear.StandardDates(startYear)
		label := domain_finyear.FinYrLabel(startYear)
		options = append(options, domain_finyear.Option{
			FinYr:     label,
			StartDate: st.Format(domain_finyear.DateLayout),
			EndDate:   end.Format(domain_finyear.DateLayout),
			Kind:      kind,
			Exists:    have[label],
		})
	}
	return options, nil
}

func (uc *finYearUsecase) Validate(ctx context.Context, in domain_finyear.FinancialYearInput) (*domain_finyear.FinancialYear, error) {
	if _, err := domain.TenantPrincipal(ctx); err != nil {
		return nil, err
	}
	fy, decideCurrent, err := uc.prepare(in)
	if err != nil {
		return nil, err
	}
	_, err = uc.repo.Current(ctx)
	if err != nil && !domain.IsKind(err, domain.KindNotFound) {
		return nil, err
	}
	fy.IsCurrent = decideCurrent(err == nil)
	msg, err := uc.repo.CheckWithFunction(ctx, fy)
	if err != nil {
		return nil, err
	}
	if msg != domain_finyear.FunctionSuccess {
		return nil, domain_finyear.FunctionError(msg)
	}
	return fy, nil
}

func (uc *finYearUsecase) Create(ctx context.Context, in domain_finyear.FinancialYearInput) (*domain_finyear.FinancialYear, error) {
	p, err := domain.TenantPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	fy, decideCurrent, err := uc.prepare(in)
	if err != nil {
		return nil, err
	}
	if err := uc.repo.Create(ctx, fy, decideCurrent); err != nil {
		return nil, err
	}
	uc.logger.InfoContext(ctx, "financial year created",
		"tenant", p.TenantCode, "user_id", p.UserID, "finyr", fy.FinYr, "is_current", fy.IsCurrent)
	return uc.repo.Get(ctx, fy.FinYr)
}

func (uc *finYearUsecase) List(ctx context.Context) ([]domain_finyear.FinancialYear, error) {
	if _, err := domain.TenantPrincipal(ctx); err != nil {
		return nil, err
	}
	return uc.repo.List(ctx)
}

func (uc *finYearUsecase) Current(ctx context.Context) (*domain_finyear.FinancialYear, error) {
	if _, err := domain.TenantPrincipal(ctx); err != nil {
		return nil, err
	}
	return uc.repo.Current(ctx)
}

func (uc *finYearUsecase) SetCurrent(ctx context.Context, in domain_finyear.SetCurrentInput) (*domain_finyear.FinancialYear, error) {
	p, err := domain.TenantPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	in.FinYr = strings.TrimSpace(in.FinYr)
	if err := usecase_common.ValidateStruct(in); err != nil {
		return nil, err
	}
	if _, ok := domain_finyear.ParseFinYr(in.FinYr); !ok {
		return nil, domain.Validation(domain.FieldError{
			Field: "finyr", Message: "must look like 2026-2027 (two consecutive years)",
		})
	}
	changed, err := uc.repo.SetCurrent(ctx, in.FinYr)
	if err != nil {
		return nil, err
	}
	if changed {
		uc.logger.InfoContext(ctx, "current financial year changed",
			"tenant", p.TenantCode, "user_id", p.UserID, "finyr", in.FinYr)
	}
	return uc.repo.Get(ctx, in.FinYr)
}

// prepare applies the walkthrough's rules, fills the standard dates and returns the is_current rule.
func (uc *finYearUsecase) prepare(in domain_finyear.FinancialYearInput) (*domain_finyear.FinancialYear, domain_finyear.CurrentRule, error) {
	in.FinYr = strings.TrimSpace(in.FinYr)
	in.StartDate = strings.TrimSpace(in.StartDate)
	in.EndDate = strings.TrimSpace(in.EndDate)
	if err := usecase_common.ValidateStruct(in); err != nil {
		return nil, nil, err
	}

	startYear, ok := domain_finyear.ParseFinYr(in.FinYr)
	if !ok {
		return nil, nil, domain.Validation(domain.FieldError{
			Field: "finyr", Message: "must look like 2026-2027 (two consecutive years)",
		})
	}
	current, first, last := uc.allowedStartYears()
	if startYear < first || startYear > last {
		return nil, nil, domain.Validation(domain.FieldError{
			Field: "finyr",
			Message: "must be between " + domain_finyear.FinYrLabel(first) +
				" and " + domain_finyear.FinYrLabel(last),
		})
	}

	st, end := domain_finyear.StandardDates(startYear)
	wantStart, wantEnd := st.Format(domain_finyear.DateLayout), end.Format(domain_finyear.DateLayout)
	var fields []domain.FieldError
	if in.StartDate != "" && in.StartDate != wantStart {
		fields = append(fields, domain.FieldError{Field: "finyr_st_date", Message: "must be " + wantStart + " (1 April)"})
	}
	if in.EndDate != "" && in.EndDate != wantEnd {
		fields = append(fields, domain.FieldError{Field: "finyr_end_date", Message: "must be " + wantEnd + " (31 March)"})
	}
	if len(fields) > 0 {
		return nil, nil, domain.Validation(fields...)
	}

	fy := &domain_finyear.FinancialYear{FinYr: in.FinYr, StartDate: wantStart, EndDate: wantEnd}
	return fy, currentRule(in.IsCurrent, startYear == current), nil
}

// currentRule: with no current year the new one must be; else an explicit choice wins; else today's year is current.
func currentRule(explicit *bool, isTodaysYear bool) domain_finyear.CurrentRule {
	return func(hasCurrent bool) bool {
		switch {
		case !hasCurrent:
			return true
		case explicit != nil:
			return *explicit
		default:
			return isTodaysYear
		}
	}
}
