// Package validation is the walkthrough's field-check library (email, PAN, GST, mobile, PIN) plus the DTO validator.
package validation

import (
	"regexp"
	"strings"
)

var (
	mobileRe = regexp.MustCompile(`^[6-9][0-9]{9}$`)
	pinRe    = regexp.MustCompile(`^[1-9][0-9]{5}$`)
	panRe    = regexp.MustCompile(`^[A-Z]{5}[0-9]{4}[A-Z]$`)
	gstinRe  = regexp.MustCompile(`^[0-9]{2}[A-Z]{5}[0-9]{4}[A-Z][1-9A-Z]Z[0-9A-Z]$`)
	emailRe  = regexp.MustCompile(`^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$`)
	domainRe = regexp.MustCompile(`^(?i)([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,}(/.*)?$`)
)

// NormalizeIndianMobile keeps digits and drops a +91/91/0 prefix: "+91 98765-43210" -> "9876543210".
func NormalizeIndianMobile(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	digits := b.String()
	switch {
	case len(digits) == 12 && strings.HasPrefix(digits, "91"):
		return digits[2:]
	case len(digits) == 11 && strings.HasPrefix(digits, "0"):
		return digits[1:]
	}
	return digits
}

// IsIndianMobile reports whether s is a 10-digit Indian mobile number.
func IsIndianMobile(s string) bool { return mobileRe.MatchString(s) }

// IsIndianPIN reports whether s is a 6-digit PIN code that does not start with 0.
func IsIndianPIN(s string) bool { return pinRe.MatchString(s) }

// IsEmail uses the same pattern as pms.company_master_validate.
func IsEmail(s string) bool { return emailRe.MatchString(s) }

// IsPAN reports whether s looks like a PAN, e.g. ABCDE1234F.
func IsPAN(s string) bool { return panRe.MatchString(s) }

// IsGSTIN checks the GSTIN format and its check digit.
func IsGSTIN(s string) bool {
	if !gstinRe.MatchString(s) {
		return false
	}
	return gstinCheckDigit(s[:14]) == s[14]
}

const gstinCharset = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"

// gstinCheckDigit implements the GSTN mod-36 checksum over the first 14 characters.
func gstinCheckDigit(first14 string) byte {
	sum := 0
	for i := 0; i < 14; i++ {
		v := strings.IndexByte(gstinCharset, first14[i])
		factor := 1
		if i%2 == 1 {
			factor = 2
		}
		p := v * factor
		sum += p/36 + p%36
	}
	return gstinCharset[(36-sum%36)%36]
}

// NormalizeWebsite trims whitespace and a trailing slash.
func NormalizeWebsite(s string) string {
	return strings.TrimRight(strings.TrimSpace(s), "/")
}

// IsWebsite accepts a bare domain ("esis.in") or one with an http(s) scheme.
func IsWebsite(s string) bool {
	rest := s
	for _, scheme := range []string{"https://", "http://"} {
		if len(rest) > len(scheme) && strings.EqualFold(rest[:len(scheme)], scheme) {
			rest = rest[len(scheme):]
			break
		}
	}
	return domainRe.MatchString(rest)
}

// IndianStates lists the 28 states and 8 union territories.
var IndianStates = []string{
	"Andhra Pradesh", "Arunachal Pradesh", "Assam", "Bihar", "Chhattisgarh",
	"Goa", "Gujarat", "Haryana", "Himachal Pradesh", "Jharkhand", "Karnataka",
	"Kerala", "Madhya Pradesh", "Maharashtra", "Manipur", "Meghalaya", "Mizoram",
	"Nagaland", "Odisha", "Punjab", "Rajasthan", "Sikkim", "Tamil Nadu",
	"Telangana", "Tripura", "Uttar Pradesh", "Uttarakhand", "West Bengal",
	"Andaman and Nicobar Islands", "Chandigarh",
	"Dadra and Nagar Haveli and Daman and Diu", "Delhi", "Jammu and Kashmir",
	"Ladakh", "Lakshadweep", "Puducherry",
}

// CanonicalIndianState returns the list spelling for a case-insensitive match.
func CanonicalIndianState(s string) (string, bool) {
	for _, st := range IndianStates {
		if strings.EqualFold(st, strings.TrimSpace(s)) {
			return st, true
		}
	}
	return "", false
}
