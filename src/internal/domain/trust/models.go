package domain_trust

import (
	"regexp"
	"strings"
	"time"

	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
)

const (
	DateLayout = "2006-01-02"
	// FunctionSuccess is what pms.mst_trust_validate returns in take_error_msg for a valid trust.
	FunctionSuccess = "Validation successful"
)

// CodeRe is the table's own check (chk_trust_code): exactly 3 capital letters.
var CodeRe = regexp.MustCompile(`^[A-Z]{3}$`)

// Trust is one row of pms.mst_trust (Form 4). trust_code is the key and never changes.
type Trust struct {
	Code      string     `json:"trust_code"`
	Desc      string     `json:"trust_desc"`
	StartFY   string     `json:"wrk_st_finyr"`
	StartDate string     `json:"wrk_st_date"`
	CreatedAt *time.Time `json:"rec_created_at,omitempty"`
	UpdatedAt *time.Time `json:"last_update_at,omitempty"`
}

// CreateInput is the body of POST /trusts and /trusts/validate.
type CreateInput struct {
	Code      string `json:"trust_code" validate:"required"`
	Desc      string `json:"trust_desc" validate:"required,max=100"`
	StartFY   string `json:"wrk_st_finyr" validate:"required"`
	StartDate string `json:"wrk_st_date" validate:"required,datetime=2006-01-02"`
}

// UpdateInput is the body of PUT /trusts/{code}; a code never changes, so it is not accepted.
type UpdateInput struct {
	Desc      string `json:"trust_desc" validate:"required,max=100"`
	StartFY   string `json:"wrk_st_finyr" validate:"required"`
	StartDate string `json:"wrk_st_date" validate:"required,datetime=2006-01-02"`
}

// FunctionResult is what pms.mst_trust_validate returned.
type FunctionResult struct {
	Valid   bool
	Message string
}

// ErrNotFound is returned for an unknown trust code.
var ErrNotFound = domain.NotFound("Trust not found")

// ErrCodeTaken is the 409 for a trust code already in use.
var ErrCodeTaken = domain.Conflict("Trust code already exists", domain.FieldError{Field: "trust_code", Message: "is already taken"})

// DescTakenError is the 409 for a description another trust already uses (ignoring case).
func DescTakenError(otherCode string) error {
	return domain.Conflict("Trust description already exists",
		domain.FieldError{Field: "trust_desc", Message: "is already used by trust " + otherCode})
}

// NormalizeCode trims and upper-cases a code from a body or a path.
func NormalizeCode(code string) string { return strings.ToUpper(strings.TrimSpace(code)) }

// FunctionErrors turns pms.mst_trust_validate's message (problems joined by ". ") into field
// errors. A duplicate description is a conflict (409); anything else is a validation error (422).
func FunctionErrors(message string) error {
	var fields []domain.FieldError
	conflict := false
	for _, sentence := range splitSentences(message) {
		// "Invalid financial year. Please select from mst_fin_year." is one problem in two sentences.
		if strings.HasPrefix(sentence, "Please ") && len(fields) > 0 {
			fields[len(fields)-1].Message += ". " + sentence
			continue
		}
		field := FunctionMessageField(sentence)
		if field == "trust_desc" && strings.Contains(sentence, "already exists") {
			conflict = true
		}
		fields = append(fields, domain.FieldError{Field: field, Message: sentence})
	}
	if len(fields) == 0 {
		fields = append(fields, domain.FieldError{Message: message})
	}
	if conflict {
		return domain.Conflict("Trust description already exists", fields...)
	}
	return domain.Validation(fields...)
}

// FunctionMessageField maps one sentence of Uncle's message to its field ("" when none fits).
func FunctionMessageField(sentence string) string {
	switch {
	case strings.HasPrefix(sentence, "Trust code"):
		return "trust_code"
	case strings.HasPrefix(sentence, "Trust description"):
		return "trust_desc"
	case strings.HasPrefix(sentence, "Working start date"):
		return "wrk_st_date"
	case strings.Contains(strings.ToLower(sentence), "financial year"):
		return "wrk_st_finyr"
	default:
		return ""
	}
}

func splitSentences(message string) []string {
	var out []string
	for _, part := range strings.Split(strings.TrimSpace(message), ". ") {
		part = strings.TrimSuffix(strings.TrimSpace(part), ".")
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
