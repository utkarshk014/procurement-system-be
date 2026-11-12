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
	// Import checks every data row of a sheet (the first row is the header) and, unless dryRun,
	// saves all the rows that pass together; rows with problems are reported and skipped.
	Import(ctx context.Context, rows [][]string, dryRun bool) (*ImportReport, error)
}

// Repository talks to pms.mst_inst and the DBA's validate function and insert procedure.
type Repository interface {
	List(ctx context.Context, f Filter) (*Page, error)
	// Get returns one institution, or ErrNotFound.
	Get(ctx context.Context, code string) (*Institution, error)
	// NextNumbers returns the next free number after the highest one used with each prefix.
	NextNumbers(ctx context.Context, trustCode, regionCode string) (code, reg int, err error)
	// Taken reports whether the ACM code or the reg code is in use: the field name and the ACM code using it.
	Taken(ctx context.Context, in *Institution) (field, by string, err error)
	// CheckWithFunction calls pms.fn_mst_inst_validate without writing.
	CheckWithFunction(ctx context.Context, in *Institution) (*FunctionResult, error)
	// Create checks the codes are free, re-runs the DBA check and inserts, under one lock.
	Create(ctx context.Context, in *Institution) error
	// ExistingCodes returns every ACM code and every reg code in use, each mapped to its ACM code.
	ExistingCodes(ctx context.Context) (codes, regCodes map[string]string, err error)
	// CheckManyWithFunction runs pms.fn_mst_inst_validate for every row in one round trip.
	CheckManyWithFunction(ctx context.Context, rows []Institution) ([]FunctionResult, error)
	// CreateMany saves every row in one transaction under the create lock, after checking again
	// that no code was taken and that the DBA's check still passes; nothing is saved otherwise.
	CreateMany(ctx context.Context, rows []Institution) error
}
