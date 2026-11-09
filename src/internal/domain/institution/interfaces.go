package domain_institution

import "context"

// Usecase is the institution master (Form 5) of the caller's tenant. There is no update or
// delete: the DBA has given only an insert procedure.
type Usecase interface {
	// List returns one page, filtered and sorted by full code.
	List(ctx context.Context, f Filter) (*Page, error)
	// Get returns one institution by ACM code, or ErrNotFound.
	Get(ctx context.Context, code string) (*Institution, error)
	// NextCodes suggests the next ACM number for a trust and the next reg number for a region.
	NextCodes(ctx context.Context, trustCode, regionCode string) (*NextCodes, error)
	// Validate is a dry run of Create.
	Validate(ctx context.Context, in Input) (*Institution, error)
	Create(ctx context.Context, in Input) (*Institution, error)
}

// Repository talks to pms.mst_inst and the DBA's validate function and insert procedure.
type Repository interface {
	List(ctx context.Context, f Filter) (*Page, error)
	// Get returns one institution, or ErrNotFound.
	Get(ctx context.Context, code string) (*Institution, error)
	// NextNumbers returns the next free number after the highest one used with each prefix.
	NextNumbers(ctx context.Context, trustCode, regionCode string) (code, reg int, err error)
	// Taken reports which of the codes are in use: the field name and the ACM code using it.
	Taken(ctx context.Context, in *Institution) (field, by string, err error)
	// CheckWithFunction calls pms.fn_mst_inst_validate without writing.
	CheckWithFunction(ctx context.Context, in *Institution) (*FunctionResult, error)
	// Create checks the codes are free, re-runs the DBA check and inserts, under one lock.
	Create(ctx context.Context, in *Institution) error
}
