package validation

import "strings"

// GSTState is one GST state code and its name, spelled as in IndianStates.
type GSTState struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// GSTStates lists the GST state codes in use, sorted by code. The retired codes
// 25 (Daman and Diu) and 28 (old Andhra Pradesh) are not accepted.
var GSTStates = []GSTState{
	{"01", "Jammu and Kashmir"},
	{"02", "Himachal Pradesh"},
	{"03", "Punjab"},
	{"04", "Chandigarh"},
	{"05", "Uttarakhand"},
	{"06", "Haryana"},
	{"07", "Delhi"},
	{"08", "Rajasthan"},
	{"09", "Uttar Pradesh"},
	{"10", "Bihar"},
	{"11", "Sikkim"},
	{"12", "Arunachal Pradesh"},
	{"13", "Nagaland"},
	{"14", "Manipur"},
	{"15", "Mizoram"},
	{"16", "Tripura"},
	{"17", "Meghalaya"},
	{"18", "Assam"},
	{"19", "West Bengal"},
	{"20", "Jharkhand"},
	{"21", "Odisha"},
	{"22", "Chhattisgarh"},
	{"23", "Madhya Pradesh"},
	{"24", "Gujarat"},
	{"26", "Dadra and Nagar Haveli and Daman and Diu"},
	{"27", "Maharashtra"},
	{"29", "Karnataka"},
	{"30", "Goa"},
	{"31", "Lakshadweep"},
	{"32", "Kerala"},
	{"33", "Tamil Nadu"},
	{"34", "Puducherry"},
	{"35", "Andaman and Nicobar Islands"},
	{"36", "Telangana"},
	{"37", "Andhra Pradesh"},
	{"38", "Ladakh"},
	{"97", "Other Territory"},
}

// GSTStateByCode returns the state name for a 2-digit GST state code.
func GSTStateByCode(code string) (string, bool) {
	for _, s := range GSTStates {
		if s.Code == code {
			return s.Name, true
		}
	}
	return "", false
}

// CanonicalGSTState returns the list spelling of a GST state name, matched case-insensitively.
func CanonicalGSTState(name string) (string, bool) {
	for _, s := range GSTStates {
		if strings.EqualFold(s.Name, strings.TrimSpace(name)) {
			return s.Name, true
		}
	}
	return "", false
}

// GSTINMatchesPAN reports whether characters 3-12 of the GSTIN are the PAN.
func GSTINMatchesPAN(gstin, pan string) bool {
	return len(gstin) == 15 && len(pan) == 10 && gstin[2:12] == pan
}
