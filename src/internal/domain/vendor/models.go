// Package domain_vendor is the vendor master: a vendor with its contacts, the stock groups it
// supplies, its declared financials per year and its certificates (files kept in S3). The DBA's
// pms.mst_vendor numbers vendors itself (INT GENERATED ALWAYS AS IDENTITY).
package domain_vendor

import (
	"strings"

	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
)

// Choices for the vendor type and the supply type (Uncle's sheet: "Properitor / Partnership /
// Ltd / Public", "sales/service provider").
var (
	VendorTypes = []string{"Proprietor", "Partnership", "Ltd", "Public"}
	SupplyTypes = []string{"Sales", "Service", "Sales and service"}
)

// Canonical returns the list's spelling of s, matched ignoring case, or "" if it is not listed.
func Canonical(list []string, s string) string {
	for _, v := range list {
		if strings.EqualFold(v, strings.TrimSpace(s)) {
			return v
		}
	}
	return ""
}

// Contact is one of a vendor's three contact people.
type Contact struct {
	Person      string `json:"person"`
	Designation string `json:"designation"`
	Mobile      string `json:"mobile"`
	Email       string `json:"email"`
}

// Group is a stock group the vendor supplies.
type Group struct {
	ID   int64  `json:"grp_cd"`
	Name string `json:"item_group"`
}

// Vendor is one row of pms.mst_vendor with the groups it supplies. Column names are the DBA's
// (including his spellings refered_by and is_contruction_comp).
type Vendor struct {
	ID               int64      `json:"id"`
	Name             string     `json:"name"`
	Address          string     `json:"address"`
	City             string     `json:"city"`
	State            string     `json:"state"`
	Country          string     `json:"country"`
	Pin              string     `json:"pin"`
	VendorType       string     `json:"vendor_type"`
	Mobile           string     `json:"mobile"`
	Email            string     `json:"email"`
	Website          string     `json:"website"`
	RegDate          *string    `json:"reg_date"` // YYYY-MM-DD
	PAN              string     `json:"pan"`
	GSTIN            string     `json:"gstin"`
	GSTINState       string     `json:"gstin_state"`
	TAN              string     `json:"tan"`
	ReferredBy       string     `json:"refered_by"`
	CorrespondEmail  string     `json:"correspond_email"`
	CorrespondMobile string     `json:"correspond_mobile"`
	SupplyType       string     `json:"supply_type"`
	IsConstruction   bool       `json:"is_contruction_comp"`
	IsManufacturer   bool       `json:"is_manufacturer"`
	CompanySince     string     `json:"company_since"`
	Contacts         [3]Contact `json:"contacts"`
	Groups           []Group    `json:"item_groups"`
}

// GroupIDs returns the ids of the vendor's groups.
func (v *Vendor) GroupIDs() []int64 {
	ids := make([]int64, len(v.Groups))
	for i, g := range v.Groups {
		ids[i] = g.ID
	}
	return ids
}

// VendorDetail is a vendor with its financials and certificates (GET /vendors/{id}).
type VendorDetail struct {
	Vendor
	Financials   []Financial  `json:"financials"`
	Certificates Certificates `json:"certificates"`
}

// VendorInput is the body of POST /vendors, PUT /vendors/{id} and POST /vendors/validate.
// GroupIDs replaces the vendor's groups on every save.
type VendorInput struct {
	Name             string    `json:"name" validate:"required,max=250"`
	Address          string    `json:"address" validate:"max=250"`
	City             string    `json:"city" validate:"max=250"`
	State            string    `json:"state" validate:"max=250"`
	Country          string    `json:"country" validate:"max=250"`
	Pin              string    `json:"pin" validate:"max=10"`
	VendorType       string    `json:"vendor_type"`
	Mobile           string    `json:"mobile" validate:"max=20"`
	Email            string    `json:"email" validate:"max=150"`
	Website          string    `json:"website" validate:"max=100"`
	RegDate          string    `json:"reg_date"`
	PAN              string    `json:"pan"`
	GSTIN            string    `json:"gstin"`
	TAN              string    `json:"tan"`
	ReferredBy       string    `json:"refered_by" validate:"max=250"`
	CorrespondEmail  string    `json:"correspond_email" validate:"max=150"`
	CorrespondMobile string    `json:"correspond_mobile" validate:"max=20"`
	SupplyType       string    `json:"supply_type"`
	IsConstruction   bool      `json:"is_contruction_comp"`
	IsManufacturer   bool      `json:"is_manufacturer"`
	CompanySince     string    `json:"company_since"`
	Contacts         []Contact `json:"contacts"`
	GroupIDs         []int64   `json:"grp_cds"`
}

// ErrVendorNotFound is the 404 for an unknown vendor id.
var ErrVendorNotFound = domain.NotFound("Vendor not found")

// ---- Financials

// Financial is one year of pms.mst_vendor_financials. FinYear is "2025-2026".
type Financial struct {
	FinYear  string   `json:"fin_year"`
	Turnover *float64 `json:"fin_turnover_declared"`
	Profit   *float64 `json:"fin_profit_declared"`
	ITRNo    string   `json:"fin_itr_no"`
	ITRDate  *string  `json:"fin_itr_date"` // YYYY-MM-DD
	ITRFile  string   `json:"fin_itr_s3_bucket_reference"`
}

// FinancialInput is the body of POST /vendors/{id}/financials and PUT …/financials/{fin_year}
// (where the path names the year).
type FinancialInput struct {
	FinYear  string   `json:"fin_year"`
	Turnover *float64 `json:"fin_turnover_declared"`
	Profit   *float64 `json:"fin_profit_declared"`
	ITRNo    string   `json:"fin_itr_no" validate:"max=25"`
	ITRDate  string   `json:"fin_itr_date"`
	ITRFile  string   `json:"fin_itr_s3_bucket_reference"`
}

// ErrFinancialNotFound is the 404 for a year the vendor has no financials for.
var ErrFinancialNotFound = domain.NotFound("No financials for that year")

// ---- Certificates

// Certificate slots, in the DBA's column order. Each has <slot>_uploaded and
// <slot>_s3_bucket_reference; the oth_* slots also have a caption.
var CertificateSlots = []string{"gst", "company_reg", "pan", "msme", "tan", "aadhar", "oth_1", "oth_2", "oth_3"}

// CertificateLabels name the slots in messages.
var CertificateLabels = map[string]string{
	"gst": "GST certificate", "company_reg": "Company registration", "pan": "PAN card", "msme": "MSME certificate",
	"tan": "TAN", "aadhar": "Aadhaar", "oth_1": "Other certificate 1", "oth_2": "Other certificate 2", "oth_3": "Other certificate 3",
}

// IsOther reports whether a slot is one of the three captioned "other" slots.
func IsOther(slot string) bool { return strings.HasPrefix(slot, "oth_") }

// Certificate is one slot: the S3 key of its file ("" for none) and, for oth_*, its caption.
type Certificate struct {
	File    string `json:"file"`
	Caption string `json:"caption,omitempty"`
}

// Certificates is the vendor's one row of pms.mst_vendor_certificates, by slot. A slot is
// uploaded exactly when it has a file.
type Certificates map[string]Certificate

// Files returns every S3 key the certificates point at.
func (c Certificates) Files() []string {
	var out []string
	for _, s := range CertificateSlots {
		if f := c[s].File; f != "" {
			out = append(out, f)
		}
	}
	return out
}

// ---- Files

// UploadedFile is what POST /vendors/{id}/files returns: the key to put in a certificate or
// financial year.
type UploadedFile struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	ContentType string `json:"content_type"`
}

// MaxFileBytes bounds one uploaded document.
const MaxFileBytes = 10 << 20

// AllowedFileTypes are the document types a vendor file may be, by sniffed content type.
var AllowedFileTypes = map[string]string{
	"application/pdf": ".pdf",
	"image/jpeg":      ".jpg",
	"image/png":       ".png",
}
