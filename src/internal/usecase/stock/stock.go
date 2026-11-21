// Package usecase_stock is the stock masters: item groups, categories, manufacturers, units of
// measurement and items. Every write runs our checks, then the DBA's validate function, then one
// locked transaction that re-checks and calls his insert procedure.
package usecase_stock

import (
	"context"
	"log/slog"
	"strings"

	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
	domain_stock "github.com/utkarshk014/procurement-system-be/src/internal/domain/stock"
	usecase_common "github.com/utkarshk014/procurement-system-be/src/internal/usecase/common"
	"github.com/utkarshk014/procurement-system-be/src/packages/validation"
)

// ---- One-field lists

type listUsecase struct {
	list   domain_stock.List
	repo   domain_stock.ListRepository
	logger *slog.Logger
}

// NewListUsecase builds item groups, categories or manufacturers.
func NewListUsecase(list domain_stock.List, repo domain_stock.ListRepository, logger *slog.Logger) domain_stock.ListUsecase {
	return &listUsecase{list: list, repo: repo, logger: logger}
}

func (uc *listUsecase) List(ctx context.Context) ([]domain_stock.Entry, error) {
	if _, err := domain.TenantPrincipal(ctx); err != nil {
		return nil, err
	}
	return uc.repo.List(ctx)
}

func (uc *listUsecase) Get(ctx context.Context, id int64) (*domain_stock.Entry, error) {
	if _, err := domain.TenantPrincipal(ctx); err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, uc.list.NotFound()
	}
	return uc.repo.Get(ctx, id)
}

func (uc *listUsecase) Validate(ctx context.Context, in domain_stock.EntryInput) (*domain_stock.Entry, error) {
	if _, err := domain.TenantPrincipal(ctx); err != nil {
		return nil, err
	}
	name, err := PrepareName(in)
	if err != nil {
		return nil, err
	}
	if other, taken, err := uc.repo.NameTaken(ctx, name, 0); err != nil {
		return nil, err
	} else if taken {
		return nil, uc.list.NameTaken(other)
	}
	res, err := uc.repo.CheckWithFunction(ctx, 0, name)
	if err != nil {
		return nil, err
	}
	if !res.Valid {
		return nil, uc.list.FunctionError(res.Message)
	}
	return &domain_stock.Entry{Name: name}, nil
}

func (uc *listUsecase) Create(ctx context.Context, in domain_stock.EntryInput) (*domain_stock.Entry, error) {
	p, err := domain.TenantPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	name, err := PrepareName(in)
	if err != nil {
		return nil, err
	}
	id, err := uc.repo.Create(ctx, name)
	if err != nil {
		return nil, err
	}
	uc.logger.InfoContext(ctx, strings.ToLower(uc.list.Label)+" created", "tenant", p.TenantCode, "user_id", p.UserID, "id", id)
	return uc.repo.Get(ctx, id)
}

func (uc *listUsecase) Update(ctx context.Context, id int64, in domain_stock.EntryInput) (*domain_stock.Entry, error) {
	p, err := domain.TenantPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, uc.list.NotFound()
	}
	name, err := PrepareName(in)
	if err != nil {
		return nil, err
	}
	if err := uc.repo.Update(ctx, id, name); err != nil {
		return nil, err
	}
	uc.logger.InfoContext(ctx, strings.ToLower(uc.list.Label)+" updated", "tenant", p.TenantCode, "user_id", p.UserID, "id", id)
	return uc.repo.Get(ctx, id)
}

func (uc *listUsecase) Delete(ctx context.Context, id int64) error {
	p, err := domain.TenantPrincipal(ctx)
	if err != nil {
		return err
	}
	if id <= 0 {
		return uc.list.NotFound()
	}
	if err := uc.repo.Delete(ctx, id); err != nil {
		return err
	}
	uc.logger.InfoContext(ctx, strings.ToLower(uc.list.Label)+" deleted", "tenant", p.TenantCode, "user_id", p.UserID, "id", id)
	return nil
}

// PrepareName tidies the spaces in a list entry's name and checks it.
func PrepareName(in domain_stock.EntryInput) (string, error) {
	in.Name = domain_stock.NormalizeName(in.Name)
	if fields := usecase_common.ToFieldErrors(validation.Struct(in)); len(fields) > 0 {
		return "", domain.Validation(fields...)
	}
	return in.Name, nil
}

// ---- Units of measurement

type uomUsecase struct {
	repo   domain_stock.UOMRepository
	logger *slog.Logger
}

// NewUOMUsecase builds the units of measurement.
func NewUOMUsecase(repo domain_stock.UOMRepository, logger *slog.Logger) domain_stock.UOMUsecase {
	return &uomUsecase{repo: repo, logger: logger}
}

func (uc *uomUsecase) List(ctx context.Context) ([]domain_stock.UOM, error) {
	if _, err := domain.TenantPrincipal(ctx); err != nil {
		return nil, err
	}
	return uc.repo.List(ctx)
}

func (uc *uomUsecase) Get(ctx context.Context, id int64) (*domain_stock.UOM, error) {
	if _, err := domain.TenantPrincipal(ctx); err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, domain_stock.ErrUOMNotFound
	}
	return uc.repo.Get(ctx, id)
}

func (uc *uomUsecase) Validate(ctx context.Context, in domain_stock.UOMInput) (*domain_stock.UOM, error) {
	if _, err := domain.TenantPrincipal(ctx); err != nil {
		return nil, err
	}
	u, err := PrepareUOM(in)
	if err != nil {
		return nil, err
	}
	if err := uc.check(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

// check runs the same duplicate checks Create repeats inside its transaction, then the DBA's.
func (uc *uomUsecase) check(ctx context.Context, u *domain_stock.UOM) error {
	taken, err := uc.repo.Taken(ctx, u)
	if err != nil {
		return err
	}
	if len(taken) > 0 {
		return domain.Conflict("Unit of measurement already exists", taken...)
	}
	res, err := uc.repo.CheckWithFunction(ctx, u)
	if err != nil {
		return err
	}
	if !res.Valid {
		return domain_stock.UOMFunctionError(res.Message)
	}
	return nil
}

func (uc *uomUsecase) Create(ctx context.Context, in domain_stock.UOMInput) (*domain_stock.UOM, error) {
	p, err := domain.TenantPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	u, err := PrepareUOM(in)
	if err != nil {
		return nil, err
	}
	id, err := uc.repo.Create(ctx, u)
	if err != nil {
		return nil, err
	}
	uc.logger.InfoContext(ctx, "unit of measurement created", "tenant", p.TenantCode, "user_id", p.UserID, "id", id)
	return uc.repo.Get(ctx, id)
}

func (uc *uomUsecase) Update(ctx context.Context, id int64, in domain_stock.UOMInput) (*domain_stock.UOM, error) {
	p, err := domain.TenantPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, domain_stock.ErrUOMNotFound
	}
	u, err := PrepareUOM(in)
	if err != nil {
		return nil, err
	}
	u.ID = id
	if err := uc.repo.Update(ctx, u); err != nil {
		return nil, err
	}
	uc.logger.InfoContext(ctx, "unit of measurement updated", "tenant", p.TenantCode, "user_id", p.UserID, "id", id)
	return uc.repo.Get(ctx, id)
}

func (uc *uomUsecase) Delete(ctx context.Context, id int64) error {
	p, err := domain.TenantPrincipal(ctx)
	if err != nil {
		return err
	}
	if id <= 0 {
		return domain_stock.ErrUOMNotFound
	}
	if err := uc.repo.Delete(ctx, id); err != nil {
		return err
	}
	uc.logger.InfoContext(ctx, "unit of measurement deleted", "tenant", p.TenantCode, "user_id", p.UserID, "id", id)
	return nil
}

// PrepareUOM upper-cases the unit, matches the standard name to the GST unit code list and checks the decimals: off means
// 0 decimals; on needs 1 to 9 (0 decimals with decimals on would mean nothing).
func PrepareUOM(in domain_stock.UOMInput) (*domain_stock.UOM, error) {
	in.UOM = strings.ToUpper(strings.TrimSpace(in.UOM))
	in.Standard = domain_stock.NormalizeName(in.Standard)
	fields := usecase_common.ToFieldErrors(validation.Struct(in))
	if in.Standard != "" {
		if std, ok := domain_stock.StandardUQC(in.Standard); ok {
			in.Standard = std
		} else {
			fields = append(fields, domain.FieldError{Field: "uom_as_standard", Message: "must be a GST unit code from the list, e.g. NOS-NUMBERS"})
		}
	}
	if in.UOM != "" && strings.ContainsAny(in.UOM, " \t") {
		fields = append(fields, domain.FieldError{Field: "uom", Message: "must not contain spaces"})
	}
	decimals := 0
	switch {
	case !in.EnableDecimal:
		if in.NumberDecimal != nil && *in.NumberDecimal != 0 {
			fields = append(fields, domain.FieldError{Field: "number_decimal", Message: "must be 0 when decimals are off"})
		}
	case in.NumberDecimal == nil:
		fields = append(fields, domain.FieldError{Field: "number_decimal", Message: "is required when decimals are on"})
	case *in.NumberDecimal < 1 || *in.NumberDecimal > 9:
		fields = append(fields, domain.FieldError{Field: "number_decimal", Message: "must be from 1 to 9"})
	default:
		decimals = *in.NumberDecimal
	}
	if len(fields) > 0 {
		return nil, domain.Validation(fields...)
	}
	return &domain_stock.UOM{UOM: in.UOM, Standard: in.Standard, EnableDecimal: in.EnableDecimal, NumberDecimal: decimals}, nil
}

// ---- Items

type itemUsecase struct {
	repo    domain_stock.ItemRepository
	lookups domain_stock.ItemLookups
	logger  *slog.Logger
}

// NewItemUsecase builds the item master. lookups are the lists an Excel import names.
func NewItemUsecase(repo domain_stock.ItemRepository, lookups domain_stock.ItemLookups, logger *slog.Logger) domain_stock.ItemUsecase {
	return &itemUsecase{repo: repo, lookups: lookups, logger: logger}
}

func (uc *itemUsecase) List(ctx context.Context) ([]domain_stock.Item, error) {
	if _, err := domain.TenantPrincipal(ctx); err != nil {
		return nil, err
	}
	return uc.repo.List(ctx)
}

func (uc *itemUsecase) Get(ctx context.Context, id int64) (*domain_stock.Item, error) {
	if _, err := domain.TenantPrincipal(ctx); err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, domain_stock.ErrItemNotFound
	}
	return uc.repo.Get(ctx, id)
}

func (uc *itemUsecase) Validate(ctx context.Context, in domain_stock.ItemInput) (*domain_stock.Item, error) {
	if _, err := domain.TenantPrincipal(ctx); err != nil {
		return nil, err
	}
	it, err := PrepareItem(in)
	if err != nil {
		return nil, err
	}
	if err := CheckItem(ctx, uc.repo, it); err != nil {
		return nil, err
	}
	return it, nil
}

// CheckItem runs the database checks Create repeats inside its transaction: links that do not
// exist (422), a description or code already used (409), then the DBA's own check.
func CheckItem(ctx context.Context, repo domain_stock.ItemRepository, it *domain_stock.Item) error {
	missing, taken, err := repo.Problems(ctx, it)
	if err != nil {
		return err
	}
	if len(missing) > 0 {
		return domain.Validation(missing...)
	}
	if len(taken) > 0 {
		return domain.Conflict("Item already exists", taken...)
	}
	res, err := repo.CheckWithFunction(ctx, it)
	if err != nil {
		return err
	}
	if !res.Valid {
		return domain_stock.ItemFunctionError(res.Message)
	}
	return nil
}

func (uc *itemUsecase) Create(ctx context.Context, in domain_stock.ItemInput) (*domain_stock.Item, error) {
	p, err := domain.TenantPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	it, err := PrepareItem(in)
	if err != nil {
		return nil, err
	}
	id, err := uc.repo.Create(ctx, it)
	if err != nil {
		return nil, err
	}
	uc.logger.InfoContext(ctx, "item created", "tenant", p.TenantCode, "user_id", p.UserID, "id", id)
	return uc.repo.Get(ctx, id)
}

func (uc *itemUsecase) Update(ctx context.Context, id int64, in domain_stock.ItemInput) (*domain_stock.Item, error) {
	p, err := domain.TenantPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, domain_stock.ErrItemNotFound
	}
	it, err := PrepareItem(in)
	if err != nil {
		return nil, err
	}
	it.ID = id
	if err := uc.repo.Update(ctx, it); err != nil {
		return nil, err
	}
	uc.logger.InfoContext(ctx, "item updated", "tenant", p.TenantCode, "user_id", p.UserID, "id", id)
	return uc.repo.Get(ctx, id)
}

func (uc *itemUsecase) Delete(ctx context.Context, id int64) error {
	p, err := domain.TenantPrincipal(ctx)
	if err != nil {
		return err
	}
	if id <= 0 {
		return domain_stock.ErrItemNotFound
	}
	if err := uc.repo.Delete(ctx, id); err != nil {
		return err
	}
	uc.logger.InfoContext(ctx, "item deleted", "tenant", p.TenantCode, "user_id", p.UserID, "id", id)
	return nil
}
