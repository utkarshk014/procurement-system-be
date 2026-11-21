// Package controllers_vendor serves the vendor master under /api/v1/vendors.
package controllers_vendor

import (
	"errors"
	"log/slog"
	"math"
	"net/http"

	"github.com/go-chi/chi/v5"
	rest_utils "github.com/utkarshk014/procurement-system-be/src/delivery/rest/utils"
	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
	domain_vendor "github.com/utkarshk014/procurement-system-be/src/internal/domain/vendor"
)

// VendorController is the vendor routes.
type VendorController interface {
	Options(w http.ResponseWriter, r *http.Request) error
	List(w http.ResponseWriter, r *http.Request) error
	Get(w http.ResponseWriter, r *http.Request) error
	Validate(w http.ResponseWriter, r *http.Request) error
	Create(w http.ResponseWriter, r *http.Request) error
	Update(w http.ResponseWriter, r *http.Request) error
	Delete(w http.ResponseWriter, r *http.Request) error
	CreateFinancial(w http.ResponseWriter, r *http.Request) error
	UpdateFinancial(w http.ResponseWriter, r *http.Request) error
	DeleteFinancial(w http.ResponseWriter, r *http.Request) error
	SaveCertificates(w http.ResponseWriter, r *http.Request) error
	UploadFile(w http.ResponseWriter, r *http.Request) error
	FileURL(w http.ResponseWriter, r *http.Request) error
}

type controller struct {
	uc domain_vendor.Usecase
}

// NewVendorController serves the vendor master.
func NewVendorController(uc domain_vendor.Usecase, _ *slog.Logger) VendorController {
	return &controller{uc: uc}
}

// vendorID reads {id}; ids are integer columns, so one past the int32 range cannot exist.
func vendorID(r *http.Request) (int64, error) {
	id, err := rest_utils.IntParam(r, "id", "Vendor")
	if err != nil {
		return 0, err
	}
	if id > math.MaxInt32 {
		return 0, domain_vendor.ErrVendorNotFound
	}
	return id, nil
}

// Options lists the choices for the vendor type and supply type dropdowns.
func (c *controller) Options(w http.ResponseWriter, r *http.Request) error {
	rest_utils.Respond(w, http.StatusOK, map[string]any{
		"vendor_types":       domain_vendor.VendorTypes,
		"supply_types":       domain_vendor.SupplyTypes,
		"certificate_slots":  domain_vendor.CertificateSlots,
		"certificate_labels": domain_vendor.CertificateLabels,
		"max_file_bytes":     domain_vendor.MaxFileBytes,
	})
	return nil
}

func (c *controller) List(w http.ResponseWriter, r *http.Request) error {
	rows, err := c.uc.List(r.Context())
	if err != nil {
		return err
	}
	rest_utils.Respond(w, http.StatusOK, rows)
	return nil
}

func (c *controller) Get(w http.ResponseWriter, r *http.Request) error {
	id, err := vendorID(r)
	if err != nil {
		return err
	}
	v, err := c.uc.Get(r.Context(), id)
	if err != nil {
		return err
	}
	rest_utils.Respond(w, http.StatusOK, v)
	return nil
}

// Validate dry-runs a create without writing.
func (c *controller) Validate(w http.ResponseWriter, r *http.Request) error {
	var in domain_vendor.VendorInput
	if err := rest_utils.DecodeJSON(w, r, &in); err != nil {
		return err
	}
	v, err := c.uc.Validate(r.Context(), in)
	if err != nil {
		return err
	}
	rest_utils.Respond(w, http.StatusOK, map[string]any{"valid": true, "entry": v})
	return nil
}

func (c *controller) Create(w http.ResponseWriter, r *http.Request) error {
	var in domain_vendor.VendorInput
	if err := rest_utils.DecodeJSON(w, r, &in); err != nil {
		return err
	}
	v, err := c.uc.Create(r.Context(), in)
	if err != nil {
		return err
	}
	rest_utils.Respond(w, http.StatusCreated, v)
	return nil
}

func (c *controller) Update(w http.ResponseWriter, r *http.Request) error {
	id, err := vendorID(r)
	if err != nil {
		return err
	}
	var in domain_vendor.VendorInput
	if err := rest_utils.DecodeJSON(w, r, &in); err != nil {
		return err
	}
	v, err := c.uc.Update(r.Context(), id, in)
	if err != nil {
		return err
	}
	rest_utils.Respond(w, http.StatusOK, v)
	return nil
}

// Delete removes the vendor with its groups, financials and certificates (204).
func (c *controller) Delete(w http.ResponseWriter, r *http.Request) error {
	id, err := vendorID(r)
	if err != nil {
		return err
	}
	if err := c.uc.Delete(r.Context(), id); err != nil {
		return err
	}
	rest_utils.NoContent(w)
	return nil
}

func (c *controller) CreateFinancial(w http.ResponseWriter, r *http.Request) error {
	id, err := vendorID(r)
	if err != nil {
		return err
	}
	var in domain_vendor.FinancialInput
	if err := rest_utils.DecodeJSON(w, r, &in); err != nil {
		return err
	}
	f, err := c.uc.CreateFinancial(r.Context(), id, in)
	if err != nil {
		return err
	}
	rest_utils.Respond(w, http.StatusCreated, f)
	return nil
}

func (c *controller) UpdateFinancial(w http.ResponseWriter, r *http.Request) error {
	id, err := vendorID(r)
	if err != nil {
		return err
	}
	var in domain_vendor.FinancialInput
	if err := rest_utils.DecodeJSON(w, r, &in); err != nil {
		return err
	}
	f, err := c.uc.UpdateFinancial(r.Context(), id, chi.URLParam(r, "finYear"), in)
	if err != nil {
		return err
	}
	rest_utils.Respond(w, http.StatusOK, f)
	return nil
}

func (c *controller) DeleteFinancial(w http.ResponseWriter, r *http.Request) error {
	id, err := vendorID(r)
	if err != nil {
		return err
	}
	if err := c.uc.DeleteFinancial(r.Context(), id, chi.URLParam(r, "finYear")); err != nil {
		return err
	}
	rest_utils.NoContent(w)
	return nil
}

// SaveCertificates replaces the vendor's certificates: { "gst": {"file": key}, "oth_1": {"file": key, "caption": "ISO 9001"}, … }.
func (c *controller) SaveCertificates(w http.ResponseWriter, r *http.Request) error {
	id, err := vendorID(r)
	if err != nil {
		return err
	}
	in := domain_vendor.Certificates{}
	if err := rest_utils.DecodeJSON(w, r, &in); err != nil {
		return err
	}
	out, err := c.uc.SaveCertificates(r.Context(), id, in)
	if err != nil {
		return err
	}
	rest_utils.Respond(w, http.StatusOK, out)
	return nil
}

const uploadField = "file"

// UploadFile stores a PDF, JPG or PNG (10 MB at most) sent as the multipart field "file" and
// returns its key, to be saved in a certificate or financial year.
func (c *controller) UploadFile(w http.ResponseWriter, r *http.Request) error {
	id, err := vendorID(r)
	if err != nil {
		return err
	}
	r.Body = http.MaxBytesReader(w, r.Body, domain_vendor.MaxFileBytes+1<<20)
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return domain.Validation(domain.FieldError{Field: uploadField, Message: "is too large (10 MB at most)"})
		}
		return domain.InvalidParams("Upload the file as multipart/form-data in the field \"file\"")
	}
	defer r.MultipartForm.RemoveAll()
	file, header, err := r.FormFile(uploadField)
	if err != nil {
		return domain.InvalidParams("Upload the file as multipart/form-data in the field \"file\"")
	}
	defer file.Close()
	out, err := c.uc.UploadFile(r.Context(), id, header.Filename, header.Size, file)
	if err != nil {
		return err
	}
	rest_utils.Respond(w, http.StatusCreated, out)
	return nil
}

// FileURL returns {"url": …}, a 10-minute link to open one of the vendor's files (?key=).
func (c *controller) FileURL(w http.ResponseWriter, r *http.Request) error {
	id, err := vendorID(r)
	if err != nil {
		return err
	}
	url, err := c.uc.FileURL(r.Context(), id, r.URL.Query().Get("key"))
	if err != nil {
		return err
	}
	rest_utils.Respond(w, http.StatusOK, map[string]string{"url": url})
	return nil
}
