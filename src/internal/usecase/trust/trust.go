package usecase_trust

import (
	"context"
	"log/slog"
	"strings"

	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
	domain_finyear "github.com/utkarshk014/procurement-system-be/src/internal/domain/finyear"
	domain_trust "github.com/utkarshk014/procurement-system-be/src/internal/domain/trust"
	usecase_common "github.com/utkarshk014/procurement-system-be/src/internal/usecase/common"
	"github.com/utkarshk014/procurement-system-be/src/packages/validation"
)

type trustUsecase struct {
	repo     domain_trust.Repository
	finYears domain_finyear.Repository
	logger   *slog.Logger
}

// NewTrustUsecase builds the trust master (Form 4).
func NewTrustUsecase(repo domain_trust.Repository, finYears domain_finyear.Repository, logger *slog.Logger) domain_trust.Usecase {
	return &trustUsecase{repo: repo, finYears: finYears, logger: logger}
}

func (uc *trustUsecase) List(ctx context.Context) ([]domain_trust.Trust, error) {
	if _, err := domain.TenantPrincipal(ctx); err != nil {
		return nil, err
	}
	return uc.repo.List(ctx)
}

func (uc *trustUsecase) Get(ctx context.Context, code string) (*domain_trust.Trust, error) {
	if _, err := domain.TenantPrincipal(ctx); err != nil {
		return nil, err
	}
	code = domain_trust.NormalizeCode(code)
	if !domain_trust.CodeRe.MatchString(code) {
		return nil, domain_trust.ErrNotFound
	}
	return uc.repo.Get(ctx, code)
}

func (uc *trustUsecase) Validate(ctx context.Context, in domain_trust.CreateInput) (*domain_trust.Trust, error) {
	if _, err := domain.TenantPrincipal(ctx); err != nil {
		return nil, err
	}
	t, err := uc.prepareCreate(ctx, in)
	if err != nil {
		return nil, err
	}
	// The same duplicate checks Create runs inside its transaction.
	_, err = uc.repo.Get(ctx, t.Code)
	switch {
	case err == nil:
		return nil, domain_trust.ErrCodeTaken
	case !domain.IsKind(err, domain.KindNotFound):
		return nil, err
	}
	if other, taken, err := uc.repo.DescTaken(ctx, t.Desc, t.Code); err != nil {
		return nil, err
	} else if taken {
		return nil, domain_trust.DescTakenError(other)
	}
	res, err := uc.repo.CheckWithFunction(ctx, t)
	if err != nil {
		return nil, err
	}
	if !res.Valid {
		return nil, domain_trust.FunctionErrors(res.Message)
	}
	return t, nil
}

func (uc *trustUsecase) Create(ctx context.Context, in domain_trust.CreateInput) (*domain_trust.Trust, error) {
	p, err := domain.TenantPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	t, err := uc.prepareCreate(ctx, in)
	if err != nil {
		return nil, err
	}
	if err := uc.repo.Create(ctx, t); err != nil {
		return nil, err
	}
	uc.logger.InfoContext(ctx, "trust created", "tenant", p.TenantCode, "user_id", p.UserID, "trust_code", t.Code)
	return uc.repo.Get(ctx, t.Code)
}

func (uc *trustUsecase) Update(ctx context.Context, code string, in domain_trust.UpdateInput) (*domain_trust.Trust, error) {
	p, err := domain.TenantPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	code = domain_trust.NormalizeCode(code)
	if !domain_trust.CodeRe.MatchString(code) {
		return nil, domain_trust.ErrNotFound
	}
	in.Desc = strings.TrimSpace(in.Desc)
	in.StartFY = strings.TrimSpace(in.StartFY)
	in.StartDate = strings.TrimSpace(in.StartDate)
	fields := usecase_common.ToFieldErrors(validation.Struct(in))
	fields, err = uc.checkWorkingStart(ctx, fields, in.StartFY, in.StartDate)
	if err != nil {
		return nil, err
	}
	if len(fields) > 0 {
		return nil, domain.Validation(fields...)
	}
	t := &domain_trust.Trust{Code: code, Desc: in.Desc, StartFY: in.StartFY, StartDate: in.StartDate}
	if err := uc.repo.Update(ctx, t); err != nil {
		return nil, err
	}
	uc.logger.InfoContext(ctx, "trust updated", "tenant", p.TenantCode, "user_id", p.UserID, "trust_code", t.Code)
	return uc.repo.Get(ctx, t.Code)
}

// prepareCreate normalises the body and returns every field problem at once.
func (uc *trustUsecase) prepareCreate(ctx context.Context, in domain_trust.CreateInput) (*domain_trust.Trust, error) {
	in.Code = domain_trust.NormalizeCode(in.Code)
	in.Desc = strings.TrimSpace(in.Desc)
	in.StartFY = strings.TrimSpace(in.StartFY)
	in.StartDate = strings.TrimSpace(in.StartDate)

	fields := usecase_common.ToFieldErrors(validation.Struct(in))
	if in.Code != "" && !domain_trust.CodeRe.MatchString(in.Code) {
		fields = append(fields, domain.FieldError{Field: "trust_code", Message: "must be exactly 3 letters, e.g. SAT"})
	}
	fields, err := uc.checkWorkingStart(ctx, fields, in.StartFY, in.StartDate)
	if err != nil {
		return nil, err
	}
	if len(fields) > 0 {
		return nil, domain.Validation(fields...)
	}
	return &domain_trust.Trust{Code: in.Code, Desc: in.Desc, StartFY: in.StartFY, StartDate: in.StartDate}, nil
}

// checkWorkingStart adds errors when the year does not exist in finyr_master or the date falls
// outside it (1 April to 31 March). Fields already in error are left alone.
func (uc *trustUsecase) checkWorkingStart(ctx context.Context, fields []domain.FieldError, finYr, date string) ([]domain.FieldError, error) {
	has := map[string]bool{}
	for _, f := range fields {
		has[f.Field] = true
	}
	if finYr == "" || has["wrk_st_finyr"] {
		return fields, nil
	}
	fy, err := uc.finYears.Get(ctx, finYr)
	if err != nil {
		if domain.IsKind(err, domain.KindNotFound) {
			return append(fields, domain.FieldError{Field: "wrk_st_finyr", Message: "must be an existing financial year"}), nil
		}
		return nil, err
	}
	if date != "" && !has["wrk_st_date"] && (date < fy.StartDate || date > fy.EndDate) {
		fields = append(fields, domain.FieldError{
			Field:   "wrk_st_date",
			Message: "must fall inside " + fy.FinYr + " (" + fy.StartDate + " to " + fy.EndDate + ")",
		})
	}
	return fields, nil
}
