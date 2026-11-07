package domain_company

import (
	"time"

	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
)

const (
	// SL is the only company row; every save updates it (DBA, 07 Nov).
	SL = 1

	DefaultCountry = "India"
	DateLayout     = "2006-01-02"

	// The Indian financial year starts on 1 April (walkthrough, Form #1).
	FinancialYearStartMonth = time.April
	FinancialYearStartDay   = 1
)

// Company is one row of pms.company_master as the API returns it.
type Company struct {
	SL           int        `json:"sl"`
	Name         string     `json:"name"`
	Address      string     `json:"address"`
	City         string     `json:"city"`
	State        string     `json:"state"`
	Country      string     `json:"country"`
	Pin          string     `json:"pin"`
	Mobile       string     `json:"mobile"`
	Email        string     `json:"email"`
	Website      string     `json:"website"`
	Header1      string     `json:"header_1"`
	Header2      string     `json:"header_2"`
	ShortName    string     `json:"shortname"`
	FinStartDate string     `json:"fin_st_dt"`
	TimeZone     *time.Time `json:"time_zone,omitempty"`
	CreatedAt    *time.Time `json:"rec_created_at,omitempty"`
}

// CompanyInput is the save body (no sl); max lengths match the pms.company_master columns.
type CompanyInput struct {
	Name         string `json:"name" validate:"required,max=300"`
	Address      string `json:"address" validate:"max=300"`
	City         string `json:"city" validate:"max=200"`
	State        string `json:"state" validate:"max=200"`
	Country      string `json:"country" validate:"max=200"`
	Pin          string `json:"pin" validate:"omitempty,in_pin"`
	Mobile       string `json:"mobile" validate:"omitempty,in_mobile"`
	Email        string `json:"email" validate:"omitempty,max=200,db_email"`
	Website      string `json:"website" validate:"omitempty,max=150,website"`
	Header1      string `json:"header_1" validate:"max=200"`
	Header2      string `json:"header_2" validate:"max=200"`
	ShortName    string `json:"shortname" validate:"required,max=50"`
	FinStartDate string `json:"fin_st_dt" validate:"omitempty,datetime=2006-01-02"`
}

// ProcedureResult is what pms.company_master_validate returned.
type ProcedureResult struct {
	Valid   bool
	Message string
}

// procedureMessageField maps each pms.company_master_validate message to its field.
var procedureMessageField = map[string]string{
	"Invalid company SL number":        "sl",
	"Company name is required":         "name",
	"Invalid mobile number":            "mobile",
	"Invalid email address":            "email",
	"Invalid PIN code":                 "pin",
	"Company short name is required":   "shortname",
	"Financial start date is required": "fin_st_dt",
}

// ProcedureFieldError turns a failed procedure check into a field error.
func ProcedureFieldError(message string) domain.FieldError {
	return domain.FieldError{Field: procedureMessageField[message], Message: message}
}
