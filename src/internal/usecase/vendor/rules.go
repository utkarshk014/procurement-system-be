package usecase_vendor

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
	domain_vendor "github.com/utkarshk014/procurement-system-be/src/internal/domain/vendor"
	usecase_common "github.com/utkarshk014/procurement-system-be/src/internal/usecase/common"
	"github.com/utkarshk014/procurement-system-be/src/packages/validation"
)

var (
	// vendorMobileRe is the DBA's rule for the vendor's own number: an optional + and 7 to 14
	// digits, so a landline or a foreign number fits.
	vendorMobileRe = regexp.MustCompile(`^\+?[0-9]{7,14}$`)
	tanRe          = regexp.MustCompile(`^[A-Z]{4}[0-9]{5}[A-Z]$`)
	yearRe         = regexp.MustCompile(`^[0-9]{4}$`)
	finYearRe      = regexp.MustCompile(`^([0-9]{4})-([0-9]{4})$`)
	foreignPinRe   = regexp.MustCompile(`^[A-Za-z0-9 -]{3,10}$`)
)

// fieldErrors collects one message per field.
type fieldErrors struct {
	list []domain.FieldError
	has  map[string]bool
}

func newFieldErrors(initial []domain.FieldError) *fieldErrors {
	f := &fieldErrors{has: map[string]bool{}}
	for _, e := range initial {
		f.add(e.Field, e.Message)
	}
	return f
}

func (f *fieldErrors) add(field, msg string) {
	if !f.has[field] {
		f.list = append(f.list, domain.FieldError{Field: field, Message: msg})
		f.has[field] = true
	}
}

func (f *fieldErrors) err() error {
	if len(f.list) == 0 {
		return nil
	}
	return domain.Validation(f.list...)
}

// mobile10 tidies an Indian mobile ("+91 98765-43210" -> "9876543210"); empty stays empty.
func mobile10(s string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	return validation.NormalizeIndianMobile(s)
}

// digitsPlus keeps digits and a leading +, dropping spaces, dashes and brackets.
func digitsPlus(s string) string {
	s = strings.TrimSpace(s)
	var b strings.Builder
	for i, r := range s {
		if (r >= '0' && r <= '9') || (r == '+' && i == 0) {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return s
	}
	return b.String()
}

// isIndia reports whether a country is India (or not given, which we take as India).
func isIndia(country string) bool {
	c := strings.ToLower(strings.TrimSpace(country))
	return c == "" || c == "india" || c == "bharat"
}

// PrepareVendor tidies the body and returns every field problem at once. today is YYYY-MM-DD
// in the tenant's zone.
//
// The DBA's rules (fn_validate_mst_vendor and fn_validate_update_mst_vendor), applied first so
// all problems come back together: a name; PAN, GSTIN and TAN formats; a GSTIN carries the PAN;
// the vendor's mobile is + and 7-14 digits; other mobiles 10 digits; email format; company since
// a 4-digit year up to this year. Ours on top: the table's lengths; PAN, GSTIN and TAN upper-cased
// (his update refuses lower case his insert accepts); the GSTIN check digit and state code, with
// gstin_state taken from it; a 6-digit PIN in India; vendor and supply type from the lists; a
// registration date not in the future; at most 3 contacts; group ids positive and unique.
func PrepareVendor(in domain_vendor.VendorInput, today string) (*domain_vendor.Vendor, error) {
	trim := strings.TrimSpace
	in.Name = normalizeName(in.Name)
	in.Address, in.City, in.State, in.Country = trim(in.Address), trim(in.City), trim(in.State), trim(in.Country)
	in.Pin = trim(in.Pin)
	in.Mobile = digitsPlus(in.Mobile)
	in.Email = strings.ToLower(trim(in.Email))
	in.Website = validation.NormalizeWebsite(in.Website)
	in.RegDate = trim(in.RegDate)
	in.PAN = strings.ToUpper(trim(in.PAN))
	in.GSTIN = strings.ToUpper(trim(in.GSTIN))
	in.TAN = strings.ToUpper(trim(in.TAN))
	in.ReferredBy = trim(in.ReferredBy)
	in.CorrespondEmail = strings.ToLower(trim(in.CorrespondEmail))
	in.CorrespondMobile = mobile10(in.CorrespondMobile)
	in.CompanySince = trim(in.CompanySince)

	f := newFieldErrors(usecase_common.ToFieldErrors(validation.Struct(in)))

	v := &domain_vendor.Vendor{
		Name: in.Name, Address: in.Address, City: in.City, State: in.State, Country: in.Country, Pin: in.Pin,
		Mobile: in.Mobile, Email: in.Email, Website: in.Website, PAN: in.PAN, GSTIN: in.GSTIN, TAN: in.TAN,
		ReferredBy: in.ReferredBy, CorrespondEmail: in.CorrespondEmail, CorrespondMobile: in.CorrespondMobile,
		IsConstruction: in.IsConstruction, IsManufacturer: in.IsManufacturer, CompanySince: in.CompanySince,
	}

	if in.Pin != "" {
		if isIndia(in.Country) {
			if !validation.IsIndianPIN(in.Pin) {
				f.add("pin", "must be a 6-digit PIN code")
			}
		} else if !foreignPinRe.MatchString(in.Pin) {
			f.add("pin", "may contain only letters, digits, spaces and -")
		}
	}
	if in.VendorType != "" {
		if v.VendorType = domain_vendor.Canonical(domain_vendor.VendorTypes, in.VendorType); v.VendorType == "" {
			f.add("vendor_type", "must be one of "+strings.Join(domain_vendor.VendorTypes, ", "))
		}
	}
	if in.SupplyType != "" {
		if v.SupplyType = domain_vendor.Canonical(domain_vendor.SupplyTypes, in.SupplyType); v.SupplyType == "" {
			f.add("supply_type", "must be one of "+strings.Join(domain_vendor.SupplyTypes, ", "))
		}
	}
	if in.Mobile != "" && !vendorMobileRe.MatchString(in.Mobile) {
		f.add("mobile", "must be 7 to 14 digits, optionally starting with +")
	}
	if in.Email != "" && !validation.IsEmail(in.Email) {
		f.add("email", "is not a valid email address")
	}
	if in.Website != "" && !validation.IsWebsite(in.Website) {
		f.add("website", "is not a valid website")
	}
	if in.RegDate != "" {
		d, err := time.Parse(time.DateOnly, in.RegDate)
		switch {
		case err != nil:
			f.add("reg_date", "must be a date (YYYY-MM-DD)")
		case in.RegDate > today:
			f.add("reg_date", "cannot be in the future")
		case d.Year() < 1800:
			f.add("reg_date", "is too far in the past")
		default:
			v.RegDate = &in.RegDate
		}
	}

	panOK := in.PAN == "" || validation.IsPAN(in.PAN)
	if !panOK {
		f.add("pan", "must be a PAN like ABCDE1234F")
	}
	if in.GSTIN != "" {
		switch {
		case !validation.IsGSTIN(in.GSTIN):
			f.add("gstin", "is not a valid GSTIN")
		default:
			st, ok := validation.GSTStateByCode(in.GSTIN[:2])
			if !ok {
				f.add("gstin", "starts with an unknown GST state code "+in.GSTIN[:2])
			} else {
				v.GSTINState = st
			}
			if in.PAN != "" && panOK && !validation.GSTINMatchesPAN(in.GSTIN, in.PAN) {
				f.add("gstin", "PAN inside GSTIN must match the PAN")
			}
		}
	}
	if in.TAN != "" && !tanRe.MatchString(in.TAN) {
		f.add("tan", "must be a TAN like ABCD12345E")
	}
	if in.CorrespondEmail != "" && !validation.IsEmail(in.CorrespondEmail) {
		f.add("correspond_email", "is not a valid email address")
	}
	if in.CorrespondMobile != "" && !validation.IsIndianMobile(in.CorrespondMobile) {
		f.add("correspond_mobile", "must be a 10-digit mobile number")
	}
	if in.CompanySince != "" {
		year, _ := strconv.Atoi(in.CompanySince)
		thisYear, _ := strconv.Atoi(today[:4])
		switch {
		case !yearRe.MatchString(in.CompanySince):
			f.add("company_since", "must be a 4-digit year")
		case year < 1800 || year > thisYear:
			f.add("company_since", fmt.Sprintf("must be a year from 1800 to %d", thisYear))
		}
	}

	if len(in.Contacts) > 3 {
		f.add("contacts", "at most 3 contacts")
	}
	for i := 0; i < len(in.Contacts) && i < 3; i++ {
		c := in.Contacts[i]
		c.Person, c.Designation = normalizeName(c.Person), normalizeName(c.Designation)
		c.Mobile, c.Email = mobile10(c.Mobile), strings.ToLower(trim(c.Email))
		key := fmt.Sprintf("contacts.%d.", i)
		if len(c.Person) > 150 {
			f.add(key+"person", "must be at most 150 characters")
		}
		if len(c.Designation) > 150 {
			f.add(key+"designation", "must be at most 150 characters")
		}
		if c.Mobile != "" && !validation.IsIndianMobile(c.Mobile) {
			f.add(key+"mobile", "must be a 10-digit mobile number")
		}
		if c.Email != "" && (len(c.Email) > 150 || !validation.IsEmail(c.Email)) {
			f.add(key+"email", "is not a valid email address")
		}
		if c.Person == "" && (c.Designation != "" || c.Mobile != "" || c.Email != "") {
			f.add(key+"person", "is required for a contact")
		}
		v.Contacts[i] = c
	}

	seen := map[int64]bool{}
	for _, id := range in.GroupIDs {
		switch {
		case id <= 0 || id > math.MaxInt32:
			f.add("grp_cds", "has an id that does not exist")
		case seen[id]:
			continue
		default:
			seen[id] = true
			v.Groups = append(v.Groups, domain_vendor.Group{ID: id})
		}
	}

	if err := f.err(); err != nil {
		return nil, err
	}
	return v, nil
}

// normalizeName trims and collapses runs of spaces.
func normalizeName(s string) string { return strings.Join(strings.Fields(s), " ") }

// maxAmount is the largest NUMERIC(15,2).
const maxAmount = 9999999999999.99

// PrepareFinancial tidies one financial year and returns every problem. currentFY is the
// tenant's current financial year ("2026-2027"); a later year cannot be declared.
//
// The DBA's rules: a year written YYYY-YYYY of two consecutive years; turnover not negative;
// both amounts within NUMERIC(15,2); an ITR number of at most 25 characters. Ours on top: at most
// 2 decimals, an ITR date not in the future, and only years up to the current one.
func PrepareFinancial(in domain_vendor.FinancialInput, today, currentFY string) (*domain_vendor.Financial, error) {
	in.FinYear = strings.TrimSpace(in.FinYear)
	in.ITRNo = strings.ToUpper(strings.TrimSpace(in.ITRNo))
	in.ITRDate = strings.TrimSpace(in.ITRDate)
	in.ITRFile = strings.TrimSpace(in.ITRFile)
	f := newFieldErrors(usecase_common.ToFieldErrors(validation.Struct(in)))

	m := finYearRe.FindStringSubmatch(in.FinYear)
	switch {
	case in.FinYear == "":
		f.add("fin_year", "is required")
	case m == nil:
		f.add("fin_year", "must be like 2025-2026")
	default:
		from, _ := strconv.Atoi(m[1])
		to, _ := strconv.Atoi(m[2])
		if to != from+1 {
			f.add("fin_year", "must be two consecutive years, like 2025-2026")
		} else if from < 1950 {
			f.add("fin_year", "is too far in the past")
		} else if currentFY != "" && in.FinYear > currentFY {
			f.add("fin_year", "cannot be after the current financial year "+currentFY)
		}
	}
	amount := func(field string, v *float64, allowNegative bool) {
		if v == nil {
			return
		}
		switch {
		case math.IsNaN(*v) || math.IsInf(*v, 0):
			f.add(field, "must be a number")
		case !allowNegative && *v < 0:
			f.add(field, "cannot be negative")
		case math.Abs(*v) > maxAmount:
			f.add(field, "is too large")
		case math.Abs(*v*100-math.Round(*v*100)) > 1e-6:
			f.add(field, "may have at most 2 decimals")
		}
	}
	amount("fin_turnover_declared", in.Turnover, false)
	amount("fin_profit_declared", in.Profit, true)

	out := &domain_vendor.Financial{FinYear: in.FinYear, Turnover: in.Turnover, Profit: in.Profit, ITRNo: in.ITRNo, ITRFile: in.ITRFile}
	if in.ITRDate != "" {
		if _, err := time.Parse(time.DateOnly, in.ITRDate); err != nil {
			f.add("fin_itr_date", "must be a date (YYYY-MM-DD)")
		} else if in.ITRDate > today {
			f.add("fin_itr_date", "cannot be in the future")
		} else {
			out.ITRDate = &in.ITRDate
		}
	}
	if err := f.err(); err != nil {
		return nil, err
	}
	return out, nil
}

// PrepareCertificates tidies the certificate slots: unknown slots are refused, captions trimmed
// (at most 100 characters) and required for an "other" slot that has a file.
func PrepareCertificates(in domain_vendor.Certificates) (domain_vendor.Certificates, error) {
	f := newFieldErrors(nil)
	known := map[string]bool{}
	for _, s := range domain_vendor.CertificateSlots {
		known[s] = true
	}
	for slot := range in {
		if !known[slot] {
			f.add(slot, "is not a certificate")
		}
	}
	out := domain_vendor.Certificates{}
	for _, slot := range domain_vendor.CertificateSlots {
		c := in[slot]
		c.File = strings.TrimSpace(c.File)
		c.Caption = normalizeName(c.Caption)
		if !domain_vendor.IsOther(slot) {
			c.Caption = ""
		} else {
			if len(c.Caption) > 100 {
				f.add(slot+".caption", "must be at most 100 characters")
			}
			if c.File != "" && c.Caption == "" {
				f.add(slot+".caption", "is required: say what this certificate is")
			}
		}
		out[slot] = c
	}
	if err := f.err(); err != nil {
		return nil, err
	}
	return out, nil
}
