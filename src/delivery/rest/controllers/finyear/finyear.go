package controllers_finyear

import (
	"log/slog"
	"net/http"

	rest_utils "github.com/utkarshk014/procurement-system-be/src/delivery/rest/utils"
	domain_finyear "github.com/utkarshk014/procurement-system-be/src/internal/domain/finyear"
	"github.com/utkarshk014/procurement-system-be/src/internal/usecase"
)

// FinYearController serves financial-year setup for the caller's tenant.
type FinYearController interface {
	Options(w http.ResponseWriter, r *http.Request) error
	List(w http.ResponseWriter, r *http.Request) error
	Current(w http.ResponseWriter, r *http.Request) error
	Validate(w http.ResponseWriter, r *http.Request) error
	Create(w http.ResponseWriter, r *http.Request) error
	SetCurrent(w http.ResponseWriter, r *http.Request) error
}

type finYearController struct {
	usecases *usecase.Usecases
	logger   *slog.Logger
}

// NewFinYearController builds the financial-year controller.
func NewFinYearController(usecases *usecase.Usecases, logger *slog.Logger) FinYearController {
	return &finYearController{usecases: usecases, logger: logger}
}

// validateResponse is the dry-run result: the year as it would be saved.
type validateResponse struct {
	Valid         bool                          `json:"valid"`
	FinancialYear *domain_finyear.FinancialYear `json:"financial_year"`
}

// Options lists the years the setup form may offer (current and previous).
func (fc *finYearController) Options(w http.ResponseWriter, r *http.Request) error {
	opts, err := fc.usecases.FinancialYear.Options(r.Context())
	if err != nil {
		return err
	}
	rest_utils.Respond(w, http.StatusOK, opts)
	return nil
}

func (fc *finYearController) List(w http.ResponseWriter, r *http.Request) error {
	years, err := fc.usecases.FinancialYear.List(r.Context())
	if err != nil {
		return err
	}
	rest_utils.Respond(w, http.StatusOK, years)
	return nil
}

// Current returns the active year, or 404 before one exists.
func (fc *finYearController) Current(w http.ResponseWriter, r *http.Request) error {
	fy, err := fc.usecases.FinancialYear.Current(r.Context())
	if err != nil {
		return err
	}
	rest_utils.Respond(w, http.StatusOK, fy)
	return nil
}

func (fc *finYearController) Validate(w http.ResponseWriter, r *http.Request) error {
	var in domain_finyear.FinancialYearInput
	if err := rest_utils.DecodeJSON(w, r, &in); err != nil {
		return err
	}
	fy, err := fc.usecases.FinancialYear.Validate(r.Context(), in)
	if err != nil {
		return err
	}
	rest_utils.Respond(w, http.StatusOK, validateResponse{Valid: true, FinancialYear: fy})
	return nil
}

func (fc *finYearController) Create(w http.ResponseWriter, r *http.Request) error {
	var in domain_finyear.FinancialYearInput
	if err := rest_utils.DecodeJSON(w, r, &in); err != nil {
		return err
	}
	fy, err := fc.usecases.FinancialYear.Create(r.Context(), in)
	if err != nil {
		return err
	}
	rest_utils.Respond(w, http.StatusCreated, fy)
	return nil
}

// SetCurrent makes another existing year the current one (200); all others are un-marked.
func (fc *finYearController) SetCurrent(w http.ResponseWriter, r *http.Request) error {
	var in domain_finyear.SetCurrentInput
	if err := rest_utils.DecodeJSON(w, r, &in); err != nil {
		return err
	}
	fy, err := fc.usecases.FinancialYear.SetCurrent(r.Context(), in)
	if err != nil {
		return err
	}
	rest_utils.Respond(w, http.StatusOK, fy)
	return nil
}
