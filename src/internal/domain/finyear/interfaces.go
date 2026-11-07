package domain_finyear

import "context"

// Usecase is financial-year setup for the caller's tenant (wizard step 1).
type Usecase interface {
	// Options lists the years that may be created today: current and previous.
	Options(ctx context.Context) ([]Option, error)
	// Validate is a dry run of Create.
	Validate(ctx context.Context, in FinancialYearInput) (*FinancialYear, error)
	Create(ctx context.Context, in FinancialYearInput) (*FinancialYear, error)
	List(ctx context.Context) ([]FinancialYear, error)
	Current(ctx context.Context) (*FinancialYear, error)
}

// Repository talks to pms.finyr_master and its DBA function and procedure.
type Repository interface {
	List(ctx context.Context) ([]FinancialYear, error)
	// Get returns one year, or a NotFound error.
	Get(ctx context.Context, finYr string) (*FinancialYear, error)
	// Current returns the year marked current, or a NotFound error.
	Current(ctx context.Context) (*FinancialYear, error)
	// CheckWithFunction calls pms.finyr_master_validate and returns its message.
	CheckWithFunction(ctx context.Context, fy *FinancialYear) (string, error)
	// Create sets fy.IsCurrent via decideCurrent, re-checks and inserts, all under one lock.
	Create(ctx context.Context, fy *FinancialYear, decideCurrent CurrentRule) error
}

// CurrentRule decides whether a new year is current, given whether any year already is.
type CurrentRule func(hasCurrent bool) bool
