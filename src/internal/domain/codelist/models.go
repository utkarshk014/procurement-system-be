// Package domain_codelist is Form 5's three small code lists: places, branches and regions.
// They share one shape (a 3-letter code and a unique name), so one implementation serves all three.
package domain_codelist

import (
	"regexp"
	"strings"
	"time"

	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
)

// FunctionSuccess is what the DBA's validate functions return in error_message for a valid row.
const FunctionSuccess = "Validation successful"

// CodeRe is our rule for every list: exactly 3 capital letters, like trust codes. The DBA's
// own checks only count 3 characters, so "1 a" would pass them.
var CodeRe = regexp.MustCompile(`^[A-Z]{3}$`)

// Kind names one list and the DBA objects behind it. Every field is a constant, never input.
type Kind struct {
	Label    string // "Place": starts the DBA's messages and our own
	Table    string // pms.mst_place
	CodeCol  string // place_code
	NameCol  string // place_name
	Insert   string // pms.mst_place_insert(code, name)
	Validate string // pms.mst_place_validate(code, name)
}

var (
	Place = Kind{Label: "Place", Table: "pms.mst_place", CodeCol: "place_code", NameCol: "place_name",
		Insert: "pms.mst_place_insert", Validate: "pms.mst_place_validate"}
	Branch = Kind{Label: "Branch", Table: "pms.mst_branch", CodeCol: "branch_code", NameCol: "branch_name",
		Insert: "pms.mst_branch_insert", Validate: "pms.mst_branch_validate"}
	Region = Kind{Label: "Region", Table: "pms.mst_region", CodeCol: "regn_code", NameCol: "regn_name",
		Insert: "pms.mst_region_insert", Validate: "pms.mst_region_validate"}
)

// Entry is one row of a list. The JSON is the same for all three lists.
type Entry struct {
	Code      string     `json:"code"`
	Name      string     `json:"name"`
	CreatedAt *time.Time `json:"rec_created_at,omitempty"`
	UpdatedAt *time.Time `json:"last_update_at,omitempty"`
}

// Input is the body of POST /places (/branches, /regions) and their /validate.
type Input struct {
	Code string `json:"code" validate:"required"`
	Name string `json:"name" validate:"required,max=250"`
}

// FunctionResult is what a DBA validate function returned.
type FunctionResult struct {
	Valid   bool
	Message string
}

// NotFound is the 404 for an unknown code.
func (k Kind) NotFound() error { return domain.NotFound(k.Label + " not found") }

// CodeTaken is the 409 for a code already in use.
func (k Kind) CodeTaken() error {
	return domain.Conflict(k.Label+" code already exists", domain.FieldError{Field: "code", Message: "is already taken"})
}

// NameTaken is the 409 for a name another row already uses (ignoring case and spaces).
func (k Kind) NameTaken(otherCode string) error {
	return domain.Conflict(k.Label+" name already exists",
		domain.FieldError{Field: "name", Message: "is already used by " + otherCode})
}

// NormalizeCode trims and upper-cases a code from a body or a path.
func NormalizeCode(code string) string { return strings.ToUpper(strings.TrimSpace(code)) }

// FunctionError turns one message from the DBA (his validate functions stop at the first
// problem; his insert procedures RAISE the same wording) into a field error. A duplicate is a
// 409, anything else a 422.
func (k Kind) FunctionError(message string) error {
	message = strings.TrimSpace(message)
	field := ""
	switch {
	case strings.HasPrefix(message, k.Label+" code"):
		field = "code"
	case strings.HasPrefix(message, k.Label+" name"):
		field = "name"
	}
	fe := domain.FieldError{Field: field, Message: message}
	if strings.Contains(message, "already exists") {
		return domain.Conflict(k.Label+" already exists", fe)
	}
	return domain.Validation(fe)
}
