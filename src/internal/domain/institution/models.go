// Package domain_institution is Form 5's institutions (pms.mst_inst): Company → Trust → Institution.
package domain_institution

import (
	"regexp"
	"strings"
	"time"

	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
)

const (
	DateLayout = "2006-01-02"
	// FunctionSuccess is what pms.fn_mst_inst_validate returns in error_message for a valid row.
	FunctionSuccess = "Validation successful"
	// DefaultPageSize and MaxPageSize bound GET /institutions (the client has 571 today).
	DefaultPageSize = 50
	MaxPageSize     = 200
)

var (
	// CodeRe is the ACM code: the trust code, "-", then a 3-digit number counted per trust
	// (ACU-009), as in all 571 rows of the client's sheet. Uncle's table says 9 characters;
	// that is an open item, because with 9 the 14-character full code cannot be formed.
	CodeRe = regexp.MustCompile(`^([A-Z]{3})-([0-9]{3})$`)
	// RegCodeRe is the reg code: a region code, "-", then a 2-digit number (BGN-09).
	RegCodeRe = regexp.MustCompile(`^([A-Z]{3})-([0-9]{2})$`)
)

// FullCode is how the client writes an institution: ACU-009:BGN-09 (14 characters).
func FullCode(code, regCode string) string { return code + ":" + regCode }

// Institution is one row of pms.mst_inst. The *_name columns are copies the DBA's procedure
// takes from the trust, place and branch lists; the *_sl_idx columns are not used yet.
type Institution struct {
	Code         string     `json:"inst_code"`
	RegCode      string     `json:"inst_reg_code"`
	FullCode     string     `json:"inst_full_code"`
	Name         string     `json:"inst_name"`
	Address      string     `json:"inst_add"`
	Locality     string     `json:"inst_place"`
	PlaceCode    string     `json:"place_code"`
	PlaceName    string     `json:"place_name"`
	BranchCode   string     `json:"branch_code"`
	BranchName   string     `json:"branch_name"`
	TrustCode    string     `json:"trust_code"`
	TrustName    string     `json:"trust_name"`
	RegDate      string     `json:"reg_date"`
	FromFinYr    string     `json:"from_fin_yr"`
	Email        string     `json:"inst_email"`
	Tel          string     `json:"inst_tel"`
	HeadName     string     `json:"inst_head_name"`
	HeadDesig    string     `json:"inst_head_desig"`
	HeadMobile   string     `json:"inst_head_mobile"`
	HeadEmail    string     `json:"inst_head_email"`
	AcntName     string     `json:"inst_acnt_name"`
	AcntMobile   string     `json:"inst_acnt_mobile"`
	AcntEmail    string     `json:"inst_acnt_email"`
	LiasonName   string     `json:"inst_liason_name"`
	LiasonDesig  string     `json:"inst_liason_desig"`
	LiasonMobile string     `json:"inst_liason_mobile"`
	LiasonEmail  string     `json:"inst_liason_email"`
	CreatedAt    *time.Time `json:"rec_created_at,omitempty"`
	UpdatedAt    *time.Time `json:"last_update_at,omitempty"`
}

// Input is the body of POST /institutions and /institutions/validate. The full code and the
// *_name copies are server-owned, so they are not accepted. JSON names keep Uncle's columns
// (including his spelling "liason").
type Input struct {
	TrustCode    string `json:"trust_code" validate:"required"`
	Code         string `json:"inst_code" validate:"required"`
	RegCode      string `json:"inst_reg_code" validate:"required"`
	Name         string `json:"inst_name" validate:"required,max=250"`
	Address      string `json:"inst_add" validate:"max=250"`
	Locality     string `json:"inst_place" validate:"max=250"`
	PlaceCode    string `json:"place_code" validate:"required"`
	BranchCode   string `json:"branch_code" validate:"required"`
	FromFinYr    string `json:"from_fin_yr" validate:"required"`
	RegDate      string `json:"reg_date" validate:"omitempty,datetime=2006-01-02"`
	Email        string `json:"inst_email" validate:"omitempty,max=150,db_email"`
	Tel          string `json:"inst_tel" validate:"max=100"`
	HeadName     string `json:"inst_head_name" validate:"max=100"`
	HeadDesig    string `json:"inst_head_desig" validate:"max=100"`
	HeadMobile   string `json:"inst_head_mobile" validate:"omitempty,in_mobile"`
	HeadEmail    string `json:"inst_head_email" validate:"omitempty,max=150,db_email"`
	AcntName     string `json:"inst_acnt_name" validate:"max=100"`
	AcntMobile   string `json:"inst_acnt_mobile" validate:"omitempty,in_mobile"`
	AcntEmail    string `json:"inst_acnt_email" validate:"omitempty,max=150,db_email"`
	LiasonName   string `json:"inst_liason_name" validate:"max=100"`
	LiasonDesig  string `json:"inst_liason_desig" validate:"max=100"`
	LiasonMobile string `json:"inst_liason_mobile" validate:"omitempty,in_mobile"`
	LiasonEmail  string `json:"inst_liason_email" validate:"omitempty,max=150,db_email"`
}

// Filter narrows GET /institutions. Empty fields do not filter; Q matches name or codes.
type Filter struct {
	TrustCode  string
	BranchCode string
	PlaceCode  string
	Q          string
	Page       int
	PageSize   int
}

// Page is one page of institutions sorted by full code.
type Page struct {
	Items    []Institution `json:"items"`
	Total    int           `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"page_size"`
}

// NextCodes are the suggested next numbers for a trust and a region ("" when not asked or full).
type NextCodes struct {
	Code    string `json:"inst_code"`
	RegCode string `json:"inst_reg_code"`
}

// FunctionResult is what pms.fn_mst_inst_validate returned.
type FunctionResult struct {
	Valid   bool
	Message string
}

// ErrNotFound is returned for an unknown institution code.
var ErrNotFound = domain.NotFound("Institution not found")

// NormalizeCode trims and upper-cases a code from a body, a query or a path.
func NormalizeCode(code string) string { return strings.ToUpper(strings.TrimSpace(code)) }

// CodeTaken is the 409 for an ACM code, reg code or full code already in use.
func CodeTaken(field, other string) error {
	msg := "is already used"
	if other != "" {
		msg += " by " + other
	}
	return domain.Conflict("Institution code already exists", domain.FieldError{Field: field, Message: msg})
}

// FunctionError turns one message from the DBA into a field error. His validate function stops
// at the first problem and his insert procedure RAISEs the same wording. A duplicate is a 409.
func FunctionError(message string) error {
	message = strings.TrimSpace(message)
	fe := domain.FieldError{Field: functionMessageField(message), Message: message}
	if strings.Contains(message, "already exists") {
		return domain.Conflict("Institution code already exists", fe)
	}
	return domain.Validation(fe)
}

func functionMessageField(m string) string {
	switch {
	case strings.HasPrefix(m, "Institution code"), strings.HasPrefix(m, "Full institution code"):
		return "inst_code"
	case strings.HasPrefix(m, "Institution registration code"), strings.HasPrefix(m, "Registration code"):
		return "inst_reg_code"
	case strings.HasPrefix(m, "Invalid place code"):
		return "place_code"
	case strings.HasPrefix(m, "Invalid branch code"):
		return "branch_code"
	case strings.HasPrefix(m, "Invalid trust code"):
		return "trust_code"
	case strings.HasPrefix(m, "Invalid institution head email"):
		return "inst_head_email"
	case strings.HasPrefix(m, "Invalid institution email"):
		return "inst_email"
	case strings.HasPrefix(m, "Invalid accountant email"):
		return "inst_acnt_email"
	case strings.HasPrefix(m, "Invalid liaison email"):
		return "inst_liason_email"
	case strings.HasPrefix(m, "Institution head mobile"):
		return "inst_head_mobile"
	case strings.HasPrefix(m, "Accountant mobile"):
		return "inst_acnt_mobile"
	case strings.HasPrefix(m, "Liaison mobile"):
		return "inst_liason_mobile"
	default:
		return ""
	}
}
