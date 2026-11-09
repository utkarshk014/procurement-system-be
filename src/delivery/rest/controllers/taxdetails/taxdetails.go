package controllers_taxdetails

import (
	"log/slog"
	"net/http"

	rest_utils "github.com/utkarshk014/procurement-system-be/src/delivery/rest/utils"
	domain_taxdetails "github.com/utkarshk014/procurement-system-be/src/internal/domain/taxdetails"
	"github.com/utkarshk014/procurement-system-be/src/internal/usecase"
)

// TaxDetailsController serves the company's tax and contact details (Form 3).
type TaxDetailsController interface {
	Get(w http.ResponseWriter, r *http.Request) error
	Validate(w http.ResponseWriter, r *http.Request) error
	Save(w http.ResponseWriter, r *http.Request) error
}

type taxDetailsController struct {
	usecases *usecase.Usecases
	logger   *slog.Logger
}

// NewTaxDetailsController builds the tax details controller.
func NewTaxDetailsController(usecases *usecase.Usecases, logger *slog.Logger) TaxDetailsController {
	return &taxDetailsController{usecases: usecases, logger: logger}
}

// validateResponse is the dry-run result: the record as it would be saved.
type validateResponse struct {
	Valid      bool                          `json:"valid"`
	TaxDetails *domain_taxdetails.TaxDetails `json:"tax_details"`
}

// Get returns the details, or 404 before the first save.
func (tc *taxDetailsController) Get(w http.ResponseWriter, r *http.Request) error {
	t, err := tc.usecases.TaxDetails.Get(r.Context())
	if err != nil {
		return err
	}
	rest_utils.Respond(w, http.StatusOK, t)
	return nil
}

// Validate dry-runs a save without writing.
func (tc *taxDetailsController) Validate(w http.ResponseWriter, r *http.Request) error {
	var in domain_taxdetails.TaxDetailsInput
	if err := rest_utils.DecodeJSON(w, r, &in); err != nil {
		return err
	}
	t, err := tc.usecases.TaxDetails.Validate(r.Context(), in)
	if err != nil {
		return err
	}
	rest_utils.Respond(w, http.StatusOK, validateResponse{Valid: true, TaxDetails: t})
	return nil
}

// Save creates (201) or updates (200) the record, replacing every field except a kept password.
func (tc *taxDetailsController) Save(w http.ResponseWriter, r *http.Request) error {
	var in domain_taxdetails.TaxDetailsInput
	if err := rest_utils.DecodeJSON(w, r, &in); err != nil {
		return err
	}
	t, created, err := tc.usecases.TaxDetails.Save(r.Context(), in)
	if err != nil {
		return err
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	rest_utils.Respond(w, status, t)
	return nil
}
