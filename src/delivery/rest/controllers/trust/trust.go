package controllers_trust

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	rest_utils "github.com/utkarshk014/procurement-system-be/src/delivery/rest/utils"
	domain_trust "github.com/utkarshk014/procurement-system-be/src/internal/domain/trust"
	"github.com/utkarshk014/procurement-system-be/src/internal/usecase"
)

// TrustController serves the trust master (Form 4) of the caller's tenant.
type TrustController interface {
	List(w http.ResponseWriter, r *http.Request) error
	Get(w http.ResponseWriter, r *http.Request) error
	Validate(w http.ResponseWriter, r *http.Request) error
	Create(w http.ResponseWriter, r *http.Request) error
	Update(w http.ResponseWriter, r *http.Request) error
}

type trustController struct {
	usecases *usecase.Usecases
	logger   *slog.Logger
}

// NewTrustController builds the trust controller.
func NewTrustController(usecases *usecase.Usecases, logger *slog.Logger) TrustController {
	return &trustController{usecases: usecases, logger: logger}
}

// validateResponse is the dry-run result: the trust as it would be saved.
type validateResponse struct {
	Valid bool                `json:"valid"`
	Trust *domain_trust.Trust `json:"trust"`
}

// List returns every trust sorted by code (an empty list before the first one).
func (tc *trustController) List(w http.ResponseWriter, r *http.Request) error {
	trusts, err := tc.usecases.Trust.List(r.Context())
	if err != nil {
		return err
	}
	rest_utils.Respond(w, http.StatusOK, trusts)
	return nil
}

// Get returns one trust; the code in the path is upper-cased first.
func (tc *trustController) Get(w http.ResponseWriter, r *http.Request) error {
	t, err := tc.usecases.Trust.Get(r.Context(), chi.URLParam(r, "code"))
	if err != nil {
		return err
	}
	rest_utils.Respond(w, http.StatusOK, t)
	return nil
}

// Validate dry-runs a create without writing.
func (tc *trustController) Validate(w http.ResponseWriter, r *http.Request) error {
	var in domain_trust.CreateInput
	if err := rest_utils.DecodeJSON(w, r, &in); err != nil {
		return err
	}
	t, err := tc.usecases.Trust.Validate(r.Context(), in)
	if err != nil {
		return err
	}
	rest_utils.Respond(w, http.StatusOK, validateResponse{Valid: true, Trust: t})
	return nil
}

// Create adds a trust (201).
func (tc *trustController) Create(w http.ResponseWriter, r *http.Request) error {
	var in domain_trust.CreateInput
	if err := rest_utils.DecodeJSON(w, r, &in); err != nil {
		return err
	}
	t, err := tc.usecases.Trust.Create(r.Context(), in)
	if err != nil {
		return err
	}
	rest_utils.Respond(w, http.StatusCreated, t)
	return nil
}

// Update changes an existing trust (200); the body has no trust_code.
func (tc *trustController) Update(w http.ResponseWriter, r *http.Request) error {
	var in domain_trust.UpdateInput
	if err := rest_utils.DecodeJSON(w, r, &in); err != nil {
		return err
	}
	t, err := tc.usecases.Trust.Update(r.Context(), chi.URLParam(r, "code"), in)
	if err != nil {
		return err
	}
	rest_utils.Respond(w, http.StatusOK, t)
	return nil
}
