package domain_finyear

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
)

const (
	// DateLayout is how dates travel in the API.
	DateLayout = "2006-01-02"
	// FunctionSuccess is what pms.finyr_master_validate returns for a valid year.
	FunctionSuccess = "SUCCESS"
)

// FinancialYear is one row of pms.finyr_master.
type FinancialYear struct {
	FinYr     string     `json:"finyr"`
	StartDate string     `json:"finyr_st_date"`
	EndDate   string     `json:"finyr_end_date"`
	IsCurrent bool       `json:"finyr_is_current"`
	CreatedAt *time.Time `json:"rec_created_at,omitempty"`
}

// FinancialYearInput is the create body; dates default to 1 April–31 March of the chosen year.
type FinancialYearInput struct {
	FinYr     string `json:"finyr" validate:"required"`
	StartDate string `json:"finyr_st_date" validate:"omitempty,datetime=2006-01-02"`
	EndDate   string `json:"finyr_end_date" validate:"omitempty,datetime=2006-01-02"`
	IsCurrent *bool  `json:"finyr_is_current"`
}

// SetCurrentInput is the body of PUT /financial-years/current.
type SetCurrentInput struct {
	FinYr string `json:"finyr" validate:"required"`
}

// Option kinds, relative to today's financial year.
const (
	KindPrevious = "previous"
	KindCurrent  = "current"
	KindFuture   = "future"
)

// Option is a financial year that may be created: the previous one, the current one, or a future one.
type Option struct {
	FinYr     string `json:"finyr"`
	StartDate string `json:"finyr_st_date"`
	EndDate   string `json:"finyr_end_date"`
	Kind      string `json:"kind"` // KindPrevious, KindCurrent or KindFuture
	Exists    bool   `json:"exists"`
}

var finYrRe = regexp.MustCompile(`^([0-9]{4})-([0-9]{4})$`)

// ParseFinYr returns the first year of a label like "2026-2027" (years must be consecutive).
func ParseFinYr(s string) (int, bool) {
	m := finYrRe.FindStringSubmatch(s)
	if m == nil {
		return 0, false
	}
	start, _ := strconv.Atoi(m[1])
	end, _ := strconv.Atoi(m[2])
	if end != start+1 {
		return 0, false
	}
	return start, true
}

// FinYrLabel formats the year starting in startYear, e.g. 2026 -> "2026-2027".
func FinYrLabel(startYear int) string {
	return fmt.Sprintf("%d-%d", startYear, startYear+1)
}

// StandardDates returns 1 April of startYear and 31 March of the next year.
func StandardDates(startYear int) (time.Time, time.Time) {
	return time.Date(startYear, time.April, 1, 0, 0, 0, 0, time.UTC),
		time.Date(startYear+1, time.March, 31, 0, 0, 0, 0, time.UTC)
}

// StartYearFor returns the first year of the financial year that contains day.
func StartYearFor(day time.Time) int {
	if day.Month() >= time.April {
		return day.Year()
	}
	return day.Year() - 1
}

// functionMessageField maps DBA function/procedure messages to a field and whether they are conflicts.
var functionMessageField = map[string]struct {
	field    string
	conflict bool
}{
	"Financial year is required":                                   {"finyr", false},
	"Financial year already exists":                                {"finyr", true},
	"Financial year start date is required":                        {"finyr_st_date", false},
	"Financial year end date is required":                          {"finyr_end_date", false},
	"Financial year end date cannot be before start date":          {"finyr_end_date", false},
	"Financial year dates overlap with an existing financial year": {"finyr", true},
	"Financial year start date and end date are required":          {"finyr_st_date", false},
	"Financial year does not exist":                                {"finyr", false},
}

// FunctionError turns a DBA function or procedure message into a field error.
func FunctionError(message string) error {
	m, ok := functionMessageField[message]
	// The procedure's duplicate message names the year: "Financial year 2026-2027 already exists".
	if !ok && strings.HasPrefix(message, "Financial year ") && strings.HasSuffix(message, " already exists") {
		m = functionMessageField["Financial year already exists"]
	}
	// mst_finyr_set_current names the year too: "Financial year 2030-2031 does not exist".
	if !ok && strings.HasPrefix(message, "Financial year ") && strings.HasSuffix(message, " does not exist") {
		m = functionMessageField["Financial year does not exist"]
	}
	fe := domain.FieldError{Field: m.field, Message: message}
	if m.conflict {
		return domain.Conflict(message, fe)
	}
	return domain.Validation(fe)
}
