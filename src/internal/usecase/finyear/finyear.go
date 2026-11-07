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
	repo     domain_finyear.Repository
	location *time.Location
	logger   *slog.Logger
	now      func() time.Time
}

// NewFinYearUsecase builds financial-year setup; location decides what "today" is.
func NewFinYearUsecase(repo domain_finyear.Repository, location *time.Location, logger *slog.Logger) domain_finyear.Usecase {
	return &finYearUsecase{repo: repo, location: location, logger: logger, now: time.Now}
}

// allowedStartYears returns the start years of the current and previous financial year.
func (uc *finYearUsecase) allowedStartYears() (current, previous int) {
	current = domain_finyear.StartYearFor(uc.now().In(uc.location))
	return current, current - 1
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
	current, previous := uc.allowedStartYears()
	option := func(startYear int, kind string) domain_finyear.Option {
		st, end := domain_finyear.StandardDates(startYear)
		label := domain_finyear.FinYrLabel(startYear)
		return domain_finyear.Option{
			FinYr:     label,
			StartDate: st.Format(domain_finyear.DateLayout),
			EndDate:   end.Format(domain_finyear.DateLayout),
			Kind:      kind,
			Exists:    have[label],
		}
	}
	return []domain_finyear.Option{option(current, "current"), option(previous, "previous")}, nil
}

func (uc *finYearUsecase) Validate(ctx context.Context, in domain_finyear.FinancialYearInput) (*domain_finyear.FinancialYear, error) {
	if _, err := domain.TenantPrincipal(ctx); err != nil {
		return nil, err
	}
	fy, err := uc.prepare(ctx, in)
	if err != nil {
		return nil, err
	}
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
	fy, err := uc.prepare(ctx, in)
	if err != nil {
		return nil, err
	}
	if err := uc.repo.Create(ctx, fy); err != nil {
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

// prepare applies the walkthrough's rules, fills the standard dates and decides is_current.
func (uc *finYearUsecase) prepare(ctx context.Context, in domain_finyear.FinancialYearInput) (*domain_finyear.FinancialYear, error) {
	in.FinYr = strings.TrimSpace(in.FinYr)
	in.StartDate = strings.TrimSpace(in.StartDate)
	in.EndDate = strings.TrimSpace(in.EndDate)
	if err := usecase_common.ValidateStruct(in); err != nil {
		return nil, err
	}

	startYear, ok := domain_finyear.ParseFinYr(in.FinYr)
	if !ok {
		return nil, domain.Validation(domain.FieldError{
			Field: "finyr", Message: "must look like 2026-2027 (two consecutive years)",
		})
	}
	current, previous := uc.allowedStartYears()
	if startYear != current && startYear != previous {
		return nil, domain.Validation(domain.FieldError{
			Field: "finyr",
			Message: "must be the current (" + domain_finyear.FinYrLabel(current) +
				") or previous (" + domain_finyear.FinYrLabel(previous) + ") financial year",
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
		return nil, domain.Validation(fields...)
	}

	// While no year is current the new one must be, or the company step could never start.
	_, err := uc.repo.Current(ctx)
	hasCurrent := err == nil
	if err != nil && !domain.IsKind(err, domain.KindNotFound) {
		return nil, err
	}
	isCurrent := !hasCurrent || (in.IsCurrent != nil && *in.IsCurrent)

	return &domain_finyear.FinancialYear{
		FinYr:     in.FinYr,
		StartDate: wantStart,
		EndDate:   wantEnd,
		IsCurrent: isCurrent,
	}, nil
}
