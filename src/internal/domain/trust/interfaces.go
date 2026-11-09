package domain_trust

import "context"

// Usecase is the trust master (Form 4) of the caller's tenant.
type Usecase interface {
	// List returns every trust sorted by code (an empty list, never NotFound).
	List(ctx context.Context) ([]Trust, error)
	// Get returns one trust, or ErrNotFound.
	Get(ctx context.Context, code string) (*Trust, error)
	// Validate is a dry run of Create.
	Validate(ctx context.Context, in CreateInput) (*Trust, error)
	Create(ctx context.Context, in CreateInput) (*Trust, error)
	// Update changes the description and working start of an existing trust; the code never changes.
	Update(ctx context.Context, code string, in UpdateInput) (*Trust, error)
}

// Repository talks to pms.mst_trust and the DBA's validate function and insert procedure.
type Repository interface {
	List(ctx context.Context) ([]Trust, error)
	// Get returns one trust, or ErrNotFound.
	Get(ctx context.Context, code string) (*Trust, error)
	// DescTaken returns the code of another trust (not exceptCode) using desc, ignoring case and spaces.
	DescTaken(ctx context.Context, desc, exceptCode string) (code string, taken bool, err error)
	// CheckWithFunction calls pms.mst_trust_validate without writing.
	CheckWithFunction(ctx context.Context, t *Trust) (*FunctionResult, error)
	// Create checks the code and description are free, re-runs the DBA check and inserts, under one lock.
	Create(ctx context.Context, t *Trust) error
	// Update checks the trust exists and the description is free, re-runs the DBA check and saves, under one lock.
	Update(ctx context.Context, t *Trust) error
}
