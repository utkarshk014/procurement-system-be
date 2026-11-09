package domain_codelist

import "context"

// Usecase is one code list (places, branches or regions) of the caller's tenant. There is no
// update or delete: the DBA has given only insert procedures.
type Usecase interface {
	// List returns every row sorted by name (an empty list, never NotFound).
	List(ctx context.Context) ([]Entry, error)
	// Get returns one row, or the kind's NotFound.
	Get(ctx context.Context, code string) (*Entry, error)
	// Validate is a dry run of Create.
	Validate(ctx context.Context, in Input) (*Entry, error)
	Create(ctx context.Context, in Input) (*Entry, error)
}

// Repository talks to one list's table, validate function and insert procedure.
type Repository interface {
	List(ctx context.Context) ([]Entry, error)
	// Get returns one row, or the kind's NotFound.
	Get(ctx context.Context, code string) (*Entry, error)
	// NameTaken returns the code of another row using name, ignoring case and spaces.
	NameTaken(ctx context.Context, name, exceptCode string) (code string, taken bool, err error)
	// CheckWithFunction calls the DBA's validate function without writing.
	CheckWithFunction(ctx context.Context, e *Entry) (*FunctionResult, error)
	// Create checks the code and name are free, re-runs the DBA check and inserts, under one lock.
	Create(ctx context.Context, e *Entry) error
}
