package domain_company

import "context"

// Usecase is company setup (wizard step 2): one record, sl = 1, in the caller's tenant.
type Usecase interface {
	// Get returns the company, or a NotFound error before it is set up.
	Get(ctx context.Context) (*Company, error)
	// Validate is a dry run of Save (our rules, then pms.company_master_validate).
	Validate(ctx context.Context, in CompanyInput) (*Company, error)
	// Save creates or updates the company (needs a current financial year); created says which.
	Save(ctx context.Context, in CompanyInput) (c *Company, created bool, err error)
}

// Repository talks to pms.company_master and its DBA function and procedure.
type Repository interface {
	// Get returns the company row (sl = 1), or a NotFound error.
	Get(ctx context.Context) (*Company, error)
	// CheckWithProcedure calls pms.company_master_validate without writing.
	CheckWithProcedure(ctx context.Context, c *Company) (*ProcedureResult, error)
	// Save re-checks and writes sl = 1 via pms.company_master_insert in a locked transaction, keeping time_zone.
	Save(ctx context.Context, c *Company) (created bool, err error)
}
