package domain_taxdetails

import "context"

// Usecase is the company's tax and contact details (Form 3) in the caller's tenant.
type Usecase interface {
	// Get returns the details, or a NotFound error before the first save.
	Get(ctx context.Context) (*TaxDetails, error)
	// Validate is a dry run of Save.
	Validate(ctx context.Context, in TaxDetailsInput) (*TaxDetails, error)
	// Save creates or updates the single record (the company must exist); created says which.
	Save(ctx context.Context, in TaxDetailsInput) (t *TaxDetails, created bool, err error)
}

// Repository talks to pms.mst_company_tax_details and the DBA's insert procedure.
type Repository interface {
	// Get returns the row (sl = 1), or a NotFound error.
	Get(ctx context.Context) (*TaxDetails, error)
	// CompanyExists reports whether pms.company_master has sl = 1.
	CompanyExists(ctx context.Context) (bool, error)
	// Save writes sl = 1 via pms.mst_company_tax_details_insert in one locked transaction,
	// after checking the company exists; pwd decides what happens to the stored password.
	Save(ctx context.Context, t *TaxDetails, pwd PasswordChange) (created bool, err error)
}
