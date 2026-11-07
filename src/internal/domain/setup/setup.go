package domain_setup

import "context"

// Step is a stage of the first-time setup wizard.
type Step string

const (
	StepFinancialYear Step = "financial_year"
	StepCompany       Step = "company"
	StepDone          Step = "done"
)

// Status tells the frontend where the tenant is in the setup wizard.
type Status struct {
	FinancialYearDone    bool   `json:"financial_year_done"`
	CurrentFinancialYear string `json:"current_financial_year,omitempty"`
	CompanyDone          bool   `json:"company_done"`
	NextStep             Step   `json:"next_step"`
}

// Usecase reports setup progress for the caller's tenant.
type Usecase interface {
	Status(ctx context.Context) (*Status, error)
}
