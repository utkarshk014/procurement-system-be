package domain_taxdetails

import (
	"time"

	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
)

const (
	// SL is the single company's key; tax details live in one row per company (DBA v1.01, Form 3).
	SL = 1

	DateLayout = "2006-01-02"
)

// ErrCompanyFirst is returned when tax details are saved before the company.
var ErrCompanyFirst = domain.Conflict("Save the company details first")

// TaxDetails is the row of pms.mst_company_tax_details as the API returns it.
// Empty text is "", an empty date is null. The mail password is never returned:
// CommMailPwdSet says whether one is stored.
type TaxDetails struct {
	SL               int        `json:"sl"`
	PANEnabled       bool       `json:"pan_enabled"`
	PANEnabledDate   *string    `json:"pan_enabled_dt"`
	PAN              string     `json:"pan"`
	GSTINEnabled     bool       `json:"gstin_enabled"`
	GSTINEnabledDate *string    `json:"gstin_enabled_dt"`
	GSTIN            string     `json:"gstin"`
	GSTINState       string     `json:"gstin_state"`
	CommMailID       string     `json:"comm_mail_id"`
	CommMailPwdSet   bool       `json:"comm_mail_pwd_set"`
	CommMobile       string     `json:"comm_mobile"`
	CommWhatsapp     string     `json:"comm_whatsapp"`
	CreatedAt        *time.Time `json:"rec_created_at,omitempty"`
}

// TaxDetailsInput is the save and validate body. Server-owned fields (sl,
// comm_mail_pwd_set, rec_created_at) are not accepted. Max lengths match the columns.
type TaxDetailsInput struct {
	PANEnabled       bool    `json:"pan_enabled"`
	PANEnabledDate   *string `json:"pan_enabled_dt" validate:"omitempty,datetime=2006-01-02"`
	PAN              string  `json:"pan"`
	GSTINEnabled     bool    `json:"gstin_enabled"`
	GSTINEnabledDate *string `json:"gstin_enabled_dt" validate:"omitempty,datetime=2006-01-02"`
	GSTIN            string  `json:"gstin"`
	GSTINState       string  `json:"gstin_state" validate:"max=100"`
	CommMailID       string  `json:"comm_mail_id" validate:"max=150"`
	// CommMailPwd is write-only: missing or null keeps the stored one, "" clears it, text replaces it.
	CommMailPwd  *string `json:"comm_mail_pwd" validate:"omitempty,max=150"`
	CommMobile   string  `json:"comm_mobile"`
	CommWhatsapp string  `json:"comm_whatsapp"`
}

// PasswordChange says what a save does with the stored mail password.
type PasswordChange struct {
	Keep  bool   // leave the stored password as it is
	Value string // the new password; "" clears it (only when Keep is false)
}
