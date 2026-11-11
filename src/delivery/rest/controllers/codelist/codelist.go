package controllers_codelist

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	rest_utils "github.com/utkarshk014/procurement-system-be/src/delivery/rest/utils"
	domain_codelist "github.com/utkarshk014/procurement-system-be/src/internal/domain/codelist"
)

// CodeListController serves one of Form 5's code lists (places, branches or regions).
type CodeListController interface {
	List(w http.ResponseWriter, r *http.Request) error
	Get(w http.ResponseWriter, r *http.Request) error
	Validate(w http.ResponseWriter, r *http.Request) error
	Create(w http.ResponseWriter, r *http.Request) error
	Delete(w http.ResponseWriter, r *http.Request) error
}

type codeListController struct {
	uc     domain_codelist.Usecase
	logger *slog.Logger
}

// NewCodeListController builds the controller for one list.
func NewCodeListController(uc domain_codelist.Usecase, logger *slog.Logger) CodeListController {
	return &codeListController{uc: uc, logger: logger}
}

// validateResponse is the dry-run result: the row as it would be saved.
type validateResponse struct {
	Valid bool                   `json:"valid"`
	Entry *domain_codelist.Entry `json:"entry"`
}

// List returns every row sorted by name (an empty list before the first one).
func (c *codeListController) List(w http.ResponseWriter, r *http.Request) error {
	entries, err := c.uc.List(r.Context())
	if err != nil {
		return err
	}
	rest_utils.Respond(w, http.StatusOK, entries)
	return nil
}

// Get returns one row; the code in the path is upper-cased first.
func (c *codeListController) Get(w http.ResponseWriter, r *http.Request) error {
	e, err := c.uc.Get(r.Context(), chi.URLParam(r, "code"))
	if err != nil {
		return err
	}
	rest_utils.Respond(w, http.StatusOK, e)
	return nil
}

// Validate dry-runs a create without writing.
func (c *codeListController) Validate(w http.ResponseWriter, r *http.Request) error {
	var in domain_codelist.Input
	if err := rest_utils.DecodeJSON(w, r, &in); err != nil {
		return err
	}
	e, err := c.uc.Validate(r.Context(), in)
	if err != nil {
		return err
	}
	rest_utils.Respond(w, http.StatusOK, validateResponse{Valid: true, Entry: e})
	return nil
}

// Create adds a row (201).
func (c *codeListController) Create(w http.ResponseWriter, r *http.Request) error {
	var in domain_codelist.Input
	if err := rest_utils.DecodeJSON(w, r, &in); err != nil {
		return err
	}
	e, err := c.uc.Create(r.Context(), in)
	if err != nil {
		return err
	}
	rest_utils.Respond(w, http.StatusCreated, e)
	return nil
}

// Delete removes a row that nothing uses yet (204).
func (c *codeListController) Delete(w http.ResponseWriter, r *http.Request) error {
	if err := c.uc.Delete(r.Context(), chi.URLParam(r, "code")); err != nil {
		return err
	}
	rest_utils.NoContent(w)
	return nil
}
