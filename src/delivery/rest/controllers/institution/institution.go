package controllers_institution

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	rest_utils "github.com/utkarshk014/procurement-system-be/src/delivery/rest/utils"
	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
	domain_institution "github.com/utkarshk014/procurement-system-be/src/internal/domain/institution"
	"github.com/utkarshk014/procurement-system-be/src/internal/usecase"
	"github.com/utkarshk014/procurement-system-be/src/packages/sheet"
)

// InstitutionController serves the institution master (Form 5) of the caller's tenant.
type InstitutionController interface {
	List(w http.ResponseWriter, r *http.Request) error
	Get(w http.ResponseWriter, r *http.Request) error
	NextCodes(w http.ResponseWriter, r *http.Request) error
	Validate(w http.ResponseWriter, r *http.Request) error
	Create(w http.ResponseWriter, r *http.Request) error
	Import(w http.ResponseWriter, r *http.Request) error
	ImportTemplate(w http.ResponseWriter, r *http.Request) error
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

// maxImportBytes bounds an uploaded import file; 571 institutions are about 50 KB.
const maxImportBytes = 5 << 20

// Import checks (?dry_run=true) or imports an .xlsx uploaded as the multipart field "file".
func (c *institutionController) Import(w http.ResponseWriter, r *http.Request) error {
	dryRun := r.URL.Query().Get("dry_run") == "true"
	r.Body = http.MaxBytesReader(w, r.Body, maxImportBytes+1<<20)
	if err := r.ParseMultipartForm(maxImportBytes); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return domain.InvalidParams("The file is too large (5 MB at most)")
		}
		return domain.InvalidParams("Upload the file as multipart/form-data in the field \"file\"")
	}
	defer r.MultipartForm.RemoveAll()
	file, header, err := r.FormFile("file")
	if err != nil {
		return domain.InvalidParams("Upload the file as multipart/form-data in the field \"file\"")
	}
	defer file.Close()
	if header.Size > maxImportBytes {
		return domain.InvalidParams("The file is too large (5 MB at most)")
	}
	rows, err := sheet.ReadFirstSheet(file)
	if err != nil {
		return domain.Validation(domain.FieldError{Field: "file", Message: err.Error()})
	}
	report, err := c.usecases.Institution.Import(r.Context(), rows, dryRun)
	if err != nil {
		return err
	}
	rest_utils.Respond(w, http.StatusOK, report)
	return nil
}

// ImportTemplate returns an .xlsx with every import column, an example row and a Help sheet.
func (c *institutionController) ImportTemplate(w http.ResponseWriter, r *http.Request) error {
	examples := map[string]string{
		"trust_code": "SAT", "inst_code": "SAT-001", "inst_reg_code": "MYS-01", "inst_name": "BGS Public School",
		"place": "MYS", "branch": "MYB", "from_fin_yr": "", "reg_date": "", "inst_add": "Kuvempunagar",
		"inst_place": "Kuvempunagar", "inst_tel": "0821 2541234", "inst_email": "office@example.in",
		"inst_head_name": "Dr. Head", "inst_head_desig": "Principal", "inst_head_mobile": "9845012345",
		"inst_head_email": "head@example.in",
	}
	var cols []sheet.Column
	for _, col := range domain_institution.ImportColumns {
		if col.Header == "inst_full_code" || col.Header == "regn_code" {
			continue // optional checks for the old system's sheet; not part of the template
		}
		cols = append(cols, sheet.Column{Header: col.Header, Example: examples[col.Header], Help: col.Help, Required: col.Required})
	}
	data, err := sheet.Template("Institutions", cols)
	if err != nil {
		return domain.Internal("Could not build the template", err)
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="institutions-import-template.xlsx"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
	return nil
}
