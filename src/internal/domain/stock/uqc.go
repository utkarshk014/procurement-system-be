package domain_stock

import "strings"

// UQCs is the GST list of unit quantity codes (as Tally shows it), stored as CODE-NAME in
// mst_stk_uom.uom_as_standard. The DBA asked for a fixed list here, not free text.
// Keep in step with the frontend's src/lib/uqc.ts.
var UQCs = []string{
	"BAG-BAGS", "BAL-BALE", "BDL-BUNDLES", "BKL-BUCKLES", "BOU-BILLION OF UNITS", "BOX-BOX",
	"BTL-BOTTLES", "BUN-BUNCHES", "CAN-CANS", "CBM-CUBIC METERS", "CCM-CUBIC CENTIMETERS",
	"CMS-CENTIMETERS", "CTN-CARTONS", "DOZ-DOZENS", "DRM-DRUMS", "GGK-GREAT GROSS", "GMS-GRAMMES",
	"GRS-GROSS", "GYD-GROSS YARDS", "KGS-KILOGRAMS", "KLR-KILOLITRE", "KME-KILOMETRE", "LTR-LITRES",
	"MLT-MILILITRE", "MTR-METERS", "MTS-METRIC TON", "NOS-NUMBERS", "OTH-OTHERS", "PAC-PACKS",
	"PCS-PIECES", "PRS-PAIRS", "QTL-QUINTAL", "ROL-ROLLS", "SET-SETS", "SQF-SQUARE FEET",
	"SQM-SQUARE METERS", "SQY-SQUARE YARDS", "TBS-TABLETS", "TGM-TEN GROSS", "THD-THOUSANDS",
	"TON-TONNES", "TUB-TUBES", "UGS-US GALLONS", "UNT-UNITS", "YDS-YARDS",
}

// StandardUQC returns the list's spelling of s (ignoring case and runs of spaces), or false.
func StandardUQC(s string) (string, bool) {
	s = NormalizeName(s)
	for _, u := range UQCs {
		if strings.EqualFold(u, s) {
			return u, true
		}
	}
	return "", false
}
