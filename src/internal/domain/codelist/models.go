// Package domain_codelist is the small code lists: Form 5's places, branches and regions, and
// the stock masters item groups, categories and manufacturers. They share one shape (a 3-letter
// code and a unique name), so one implementation serves them all.
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
	Label     string // "Place": starts the DBA's messages and our own
	Plural    string // "places", for our own messages
	Table     string // pms.mst_place
	CodeCol   string // place_code
	NameCol   string // place_name
	Insert    string // pms.mst_place_insert(code, name)
	Validate  string // pms.mst_place_validate(code, name)
	Delete    string // pms.mst_place_delete(code)
	CanDelete string // pms.mst_place_validate_delete(code)
}

var (
	Place = Kind{Label: "Place", Plural: "places", Table: "pms.mst_place", CodeCol: "place_code", NameCol: "place_name",
		Insert: "pms.mst_place_insert", Validate: "pms.mst_place_validate",
		Delete: "pms.mst_place_delete", CanDelete: "pms.mst_place_validate_delete"}
	Branch = Kind{Label: "Branch", Plural: "branches", Table: "pms.mst_branch", CodeCol: "branch_code", NameCol: "branch_name",
		Insert: "pms.mst_branch_insert", Validate: "pms.mst_branch_validate",
		Delete: "pms.mst_branch_delete", CanDelete: "pms.mst_branch_validate_delete"}
	Region = Kind{Label: "Region", Plural: "regions", Table: "pms.mst_region", CodeCol: "regn_code", NameCol: "regn_name",
		Insert: "pms.mst_region_insert", Validate: "pms.mst_region_validate",
		Delete: "pms.mst_region_delete", CanDelete: "pms.mst_region_validate_delete"}

	// Stock masters (the old desktop screens Group, Category and Manufacturer). Their objects are
	// our PROPOSED text in db/legacy/005_stock_masters_PROPOSED.sql until the DBA installs them.
	ItemGroup = Kind{Label: "Item group", Plural: "item groups", Table: "pms.mst_item_group", CodeCol: "grp_code", NameCol: "grp_name",
		Insert: "pms.mst_item_group_insert", Validate: "pms.mst_item_group_validate",
		Delete: "pms.mst_item_group_delete", CanDelete: "pms.mst_item_group_validate_delete"}
	Category = Kind{Label: "Category", Plural: "categories", Table: "pms.mst_item_category", CodeCol: "cat_code", NameCol: "cat_name",
		Insert: "pms.mst_item_category_insert", Validate: "pms.mst_item_category_validate",
		Delete: "pms.mst_item_category_delete", CanDelete: "pms.mst_item_category_validate_delete"}
	Manufacturer = Kind{Label: "Manufacturer", Plural: "manufacturers", Table: "pms.mst_manufacturer", CodeCol: "mfr_code", NameCol: "mfr_name",
		Insert: "pms.mst_manufacturer_insert", Validate: "pms.mst_manufacturer_validate",
		Delete: "pms.mst_manufacturer_delete", CanDelete: "pms.mst_manufacturer_validate_delete"}
)

// Entry is one row of a list. The JSON is the same for every list.
type Entry struct {
	Code      string     `json:"code"`
	Name      string     `json:"name"`
	CreatedAt *time.Time `json:"rec_created_at,omitempty"`
	UpdatedAt *time.Time `json:"last_update_at,omitempty"`
}

// Input is the body of POST /places (/branches, /regions, /item-groups, /item-categories,
// /manufacturers) and their /validate.
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
