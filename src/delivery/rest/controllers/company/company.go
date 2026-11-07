package controllers_company

import (
	"log/slog"
	"net/http"

	rest_utils "github.com/utkarshk014/procurement-system-be/src/delivery/rest/utils"
	domain_company "github.com/utkarshk014/procurement-system-be/src/internal/domain/company"
	"github.com/utkarshk014/procurement-system-be/src/internal/usecase"
)

// CompanyController serves the single company record of the caller's tenant.
type CompanyController interface {
	Get(w http.ResponseWriter, r *http.Request) error
	Validate(w http.ResponseWriter, r *http.Request) error
	Save(w http.ResponseWriter, r *http.Request) error
}

type companyController struct {
	usecases *usecase.Usecases
	logger   *slog.Logger
}

// NewCompanyController builds the company controller.
func NewCompanyController(usecases *usecase.Usecases, logger *slog.Logger) CompanyController {
	return &companyController{usecases: usecases, logger: logger}
}

// validateResponse is the dry-run result: the company as it would be saved.
type validateResponse struct {
	Valid   bool                    `json:"valid"`
	Company *domain_company.Company `json:"company"`
}

// Get returns the company, or 404 before it is set up.
func (cc *companyController) Get(w http.ResponseWriter, r *http.Request) error {
	c, err := cc.usecases.Company.Get(r.Context())
	if err != nil {
		return err
	}
	rest_utils.Respond(w, http.StatusOK, c)
	return nil
}

// Validate dry-runs a save without writing.
func (cc *companyController) Validate(w http.ResponseWriter, r *http.Request) error {
	var in domain_company.CompanyInput
	if err := rest_utils.DecodeJSON(w, r, &in); err != nil {
		return err
	}
	c, err := cc.usecases.Company.Validate(r.Context(), in)
	if err != nil {
		return err
	}
	rest_utils.Respond(w, http.StatusOK, validateResponse{Valid: true, Company: c})
	return nil
}

// Save creates (201) or updates (200) the company, replacing every editable field.
func (cc *companyController) Save(w http.ResponseWriter, r *http.Request) error {
	var in domain_company.CompanyInput
	if err := rest_utils.DecodeJSON(w, r, &in); err != nil {
		return err
	}
	c, created, err := cc.usecases.Company.Save(r.Context(), in)
	if err != nil {
		return err
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	rest_utils.Respond(w, status, c)
	return nil
}
