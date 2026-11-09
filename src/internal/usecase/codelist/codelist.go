package usecase_codelist

import (
	"context"
	"log/slog"
	"strings"

	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
	domain_codelist "github.com/utkarshk014/procurement-system-be/src/internal/domain/codelist"
	usecase_common "github.com/utkarshk014/procurement-system-be/src/internal/usecase/common"
	"github.com/utkarshk014/procurement-system-be/src/packages/validation"
)

type codeListUsecase struct {
	kind   domain_codelist.Kind
	repo   domain_codelist.Repository
	logger *slog.Logger
}

// NewCodeListUsecase builds one of Form 5's code lists (places, branches or regions).
func NewCodeListUsecase(kind domain_codelist.Kind, repo domain_codelist.Repository, logger *slog.Logger) domain_codelist.Usecase {
	return &codeListUsecase{kind: kind, repo: repo, logger: logger}
}

func (uc *codeListUsecase) List(ctx context.Context) ([]domain_codelist.Entry, error) {
	if _, err := domain.TenantPrincipal(ctx); err != nil {
		return nil, err
	}
	return uc.repo.List(ctx)
}

func (uc *codeListUsecase) Get(ctx context.Context, code string) (*domain_codelist.Entry, error) {
	if _, err := domain.TenantPrincipal(ctx); err != nil {
		return nil, err
	}
	code = domain_codelist.NormalizeCode(code)
	if !domain_codelist.CodeRe.MatchString(code) {
		return nil, uc.kind.NotFound()
	}
	return uc.repo.Get(ctx, code)
}

func (uc *codeListUsecase) Validate(ctx context.Context, in domain_codelist.Input) (*domain_codelist.Entry, error) {
	if _, err := domain.TenantPrincipal(ctx); err != nil {
		return nil, err
	}
	e, err := Prepare(in)
	if err != nil {
		return nil, err
	}
	// The same duplicate checks Create runs inside its transaction.
	_, err = uc.repo.Get(ctx, e.Code)
	switch {
	case err == nil:
		return nil, uc.kind.CodeTaken()
	case !domain.IsKind(err, domain.KindNotFound):
		return nil, err
	}
	if other, taken, err := uc.repo.NameTaken(ctx, e.Name, e.Code); err != nil {
		return nil, err
	} else if taken {
		return nil, uc.kind.NameTaken(other)
	}
	res, err := uc.repo.CheckWithFunction(ctx, e)
	if err != nil {
		return nil, err
	}
	if !res.Valid {
		return nil, uc.kind.FunctionError(res.Message)
	}
	return e, nil
}

func (uc *codeListUsecase) Create(ctx context.Context, in domain_codelist.Input) (*domain_codelist.Entry, error) {
	p, err := domain.TenantPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	e, err := Prepare(in)
	if err != nil {
		return nil, err
	}
	if err := uc.repo.Create(ctx, e); err != nil {
		return nil, err
	}
	uc.logger.InfoContext(ctx, strings.ToLower(uc.kind.Label)+" created", "tenant", p.TenantCode, "user_id", p.UserID, "code", e.Code)
	return uc.repo.Get(ctx, e.Code)
}

// Prepare trims and upper-cases the body and returns every field problem at once.
func Prepare(in domain_codelist.Input) (*domain_codelist.Entry, error) {
	in.Code = domain_codelist.NormalizeCode(in.Code)
	in.Name = strings.Join(strings.Fields(in.Name), " ")
	fields := usecase_common.ToFieldErrors(validation.Struct(in))
	if in.Code != "" && !domain_codelist.CodeRe.MatchString(in.Code) {
		fields = append(fields, domain.FieldError{Field: "code", Message: "must be exactly 3 letters, e.g. MYS"})
	}
	if len(fields) > 0 {
		return nil, domain.Validation(fields...)
	}
	return &domain_codelist.Entry{Code: in.Code, Name: in.Name}, nil
}
