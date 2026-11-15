package domain_stock

import (
	"context"

	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
)

// FunctionResult is what a DBA validate function returned.
type FunctionResult struct {
	Valid   bool
	Message string
}

// ListUsecase is one one-field list (item groups, categories or manufacturers) of the caller's
// tenant. There is no edit or delete yet: the DBA has given insert procedures only.
type ListUsecase interface {
	// List returns every row sorted by name (an empty list, never NotFound).
	List(ctx context.Context) ([]Entry, error)
	Get(ctx context.Context, id int64) (*Entry, error)
	// Validate is a dry run of Create.
	Validate(ctx context.Context, in EntryInput) (*Entry, error)
	Create(ctx context.Context, in EntryInput) (*Entry, error)
}

// ListRepository talks to one list's table, validate function and insert procedure.
type ListRepository interface {
	List(ctx context.Context) ([]Entry, error)
	Get(ctx context.Context, id int64) (*Entry, error)
	// NameTaken returns the id of a row using name, ignoring case and runs of spaces.
	NameTaken(ctx context.Context, name string) (id int64, taken bool, err error)
	CheckWithFunction(ctx context.Context, name string) (*FunctionResult, error)
	// Create checks the name is free, re-runs the DBA check and inserts under one lock,
	// returning the new id.
	Create(ctx context.Context, name string) (int64, error)
}

// UOMUsecase is the units of measurement.
type UOMUsecase interface {
	List(ctx context.Context) ([]UOM, error)
	Get(ctx context.Context, id int64) (*UOM, error)
	Validate(ctx context.Context, in UOMInput) (*UOM, error)
	Create(ctx context.Context, in UOMInput) (*UOM, error)
}

// UOMRepository talks to pms.mst_stk_uom.
type UOMRepository interface {
	List(ctx context.Context) ([]UOM, error)
	Get(ctx context.Context, id int64) (*UOM, error)
	// Taken returns a 409 field error for each of uom and uom_as_standard already in use.
	Taken(ctx context.Context, u *UOM) ([]domain.FieldError, error)
	CheckWithFunction(ctx context.Context, u *UOM) (*FunctionResult, error)
	Create(ctx context.Context, u *UOM) (int64, error)
}

// ItemUsecase is the item master.
type ItemUsecase interface {
	List(ctx context.Context) ([]Item, error)
	Get(ctx context.Context, id int64) (*Item, error)
	Validate(ctx context.Context, in ItemInput) (*Item, error)
	Create(ctx context.Context, in ItemInput) (*Item, error)
	// Delete removes an item through the DBA's procedure, or returns ErrItemNotFound / a 409.
	Delete(ctx context.Context, id int64) error
}

// ItemRepository talks to pms.mst_stk_items.
type ItemRepository interface {
	List(ctx context.Context) ([]Item, error)
	Get(ctx context.Context, id int64) (*Item, error)
	// Problems returns a field error for each group, category, manufacturer or UOM that does
	// not exist, and a 409 field error for a description or code another item already uses.
	Problems(ctx context.Context, it *Item) (missing, taken []domain.FieldError, err error)
	CheckWithFunction(ctx context.Context, it *Item) (*FunctionResult, error)
	Create(ctx context.Context, it *Item) (int64, error)
	Delete(ctx context.Context, id int64) error
}
