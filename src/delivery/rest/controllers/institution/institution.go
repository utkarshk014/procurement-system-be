package controllers_institution

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	rest_utils "github.com/utkarshk014/procurement-system-be/src/delivery/rest/utils"
	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
	domain_institution "github.com/utkarshk014/procurement-system-be/src/internal/domain/institution"
	"github.com/utkarshk014/procurement-system-be/src/internal/usecase"
)

// InstitutionController serves the institution master (Form 5) of the caller's tenant.
type InstitutionController interface {
	List(w http.ResponseWriter, r *http.Request) error
	Get(w http.ResponseWriter, r *http.Request) error
	NextCodes(w http.ResponseWriter, r *http.Request) error
	Validate(w http.ResponseWriter, r *http.Request) error
	Create(w http.ResponseWriter, r *http.Request) error
}

type institutionController struct {
	usecases *usecase.Usecases
	logger   *slog.Logger
}

// NewInstitutionController builds the institution controller.
func NewInstitutionController(usecases *usecase.Usecases, logger *slog.Logger) InstitutionController {
	return &institutionController{usecases: usecases, logger: logger}
}

// validateResponse is the dry-run result: the institution as it would be saved.
type validateResponse struct {
	Valid       bool                            `json:"valid"`
	Institution *domain_institution.Institution `json:"institution"`
}

// List returns one page: ?trust_code=&branch_code=&place_code=&q=&page=&page_size=
func (c *institutionController) List(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	f := domain_institution.Filter{
		TrustCode:  q.Get("trust_code"),
		BranchCode: q.Get("branch_code"),
		PlaceCode:  q.Get("place_code"),
		Q:          q.Get("q"),
	}
	var err error
	if f.Page, err = optionalInt(q.Get("page"), "page"); err != nil {
		return err
	}
	if f.PageSize, err = optionalInt(q.Get("page_size"), "page_size"); err != nil {
		return err
	}
	page, err := c.usecases.Institution.List(r.Context(), f)
	if err != nil {
		return err
	}
	rest_utils.Respond(w, http.StatusOK, page)
	return nil
}

func optionalInt(s, name string) (int, error) {
	if s == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return 0, domain.InvalidParams(name + " must be a positive whole number")
	}
	return n, nil
}

// Get returns one institution by ACM code; the code in the path is upper-cased first.
func (c *institutionController) Get(w http.ResponseWriter, r *http.Request) error {
	i, err := c.usecases.Institution.Get(r.Context(), chi.URLParam(r, "code"))
	if err != nil {
		return err
	}
	rest_utils.Respond(w, http.StatusOK, i)
	return nil
}

// NextCodes suggests codes: ?trust_code=SAT&regn_code=MYS (either may be left out).
func (c *institutionController) NextCodes(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	next, err := c.usecases.Institution.NextCodes(r.Context(), q.Get("trust_code"), q.Get("regn_code"))
	if err != nil {
		return err
	}
	rest_utils.Respond(w, http.StatusOK, next)
	return nil
}

// Validate dry-runs a create without writing.
func (c *institutionController) Validate(w http.ResponseWriter, r *http.Request) error {
	var in domain_institution.Input
	if err := rest_utils.DecodeJSON(w, r, &in); err != nil {
		return err
	}
	i, err := c.usecases.Institution.Validate(r.Context(), in)
	if err != nil {
		return err
	}
	rest_utils.Respond(w, http.StatusOK, validateResponse{Valid: true, Institution: i})
	return nil
}

// Create adds an institution (201).
func (c *institutionController) Create(w http.ResponseWriter, r *http.Request) error {
	var in domain_institution.Input
	if err := rest_utils.DecodeJSON(w, r, &in); err != nil {
		return err
	}
	i, err := c.usecases.Institution.Create(r.Context(), in)
	if err != nil {
		return err
	}
	rest_utils.Respond(w, http.StatusCreated, i)
	return nil
}
