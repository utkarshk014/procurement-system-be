package domain_vendor

import (
	"strconv"
	"strings"

	"github.com/utkarshk014/procurement-system-be/src/packages/sheet"
)

// MaxImportRows bounds one vendor import file.
const MaxImportRows = 2000

// ImportFields is the vendor import and export layout: the vendor's own columns, its three
// contacts and its item groups by name. Financials and certificates are not part of it.
var ImportFields = func() []sheet.Field {
	f := []sheet.Field{
		{Header: "name", Aliases: []string{"vendor_name", "vendor", "supplier", "supplier_name"}, Required: true, Example: "ABC Computers", Help: "Vendor name, unique"},
		{Header: "vendor_type", Aliases: []string{"type", "firm_type"}, Example: "Proprietor", Help: "Proprietor, Partnership, Ltd or Public"},
		{Header: "supply_type", Aliases: []string{"supplies", "supply"}, Example: "Sales and service", Help: "Sales, Service or Sales and service"},
		{Header: "item_groups", Aliases: []string{"groups", "item_group"}, Example: "Computers, Printers", Help: "Item group names from the item groups list, separated by commas"},
		{Header: "address", Example: "12, MG Road", Help: "Address"},
		{Header: "city", Example: "Bengaluru", Help: "City"},
		{Header: "state", Example: "Karnataka", Help: "State"},
		{Header: "country", Example: "India", Help: "Country (empty: India)"},
		{Header: "pin", Aliases: []string{"pincode", "pin_code"}, Example: "560001", Help: "PIN code, 6 digits in India"},
		{Header: "mobile", Aliases: []string{"phone"}, Example: "080 2345 6789", Help: "Vendor phone"},
		{Header: "email", Example: "sales@abc.in", Help: "Vendor email"},
		{Header: "website", Example: "abc.in", Help: "Website"},
		{Header: "reg_date", Aliases: []string{"registered_on"}, Example: "2020-04-01", Help: "Registered on, YYYY-MM-DD"},
		{Header: "company_since", Aliases: []string{"in_business_since"}, Example: "1998", Help: "Year the business started"},
		{Header: "refered_by", Aliases: []string{"referred_by"}, Help: "Referred by"},
		{Header: "is_manufacturer", Aliases: []string{"manufacturer"}, Example: "no", Help: "yes or no"},
		{Header: "is_contruction_comp", Aliases: []string{"is_construction_comp", "construction_company", "construction"}, Example: "no", Help: "yes or no"},
		{Header: "pan", Example: "ABCDE1234F", Help: "PAN, unique"},
		{Header: "gstin", Example: "29ABCDE1234F1ZW", Help: "GSTIN, unique; must carry the PAN"},
		{Header: "tan", Example: "BLRA12345B", Help: "TAN"},
		{Header: "correspond_email", Aliases: []string{"correspondence_email"}, Example: "accounts@abc.in", Help: "Correspondence email"},
		{Header: "correspond_mobile", Aliases: []string{"correspondence_mobile"}, Example: "9845012345", Help: "Correspondence mobile, 10 digits"},
	}
	examples := [4]string{"Ravi Kumar", "Sales manager", "9845012345", "ravi@abc.in"}
	for i := 1; i <= 3; i++ {
		n := strconv.Itoa(i)
		ex := [4]string{}
		if i == 1 {
			ex = examples
		}
		f = append(f,
			sheet.Field{Header: "contact_" + n + "_person", Example: ex[0], Help: "Contact " + n + ": name (needed when any contact " + n + " column is filled)"},
			sheet.Field{Header: "contact_" + n + "_designation", Example: ex[1], Help: "Contact " + n + ": designation"},
			sheet.Field{Header: "contact_" + n + "_mobile", Example: ex[2], Help: "Contact " + n + ": mobile, 10 digits"},
			sheet.Field{Header: "contact_" + n + "_email", Example: ex[3], Help: "Contact " + n + ": email"},
		)
	}
	return f
}()

// ImportColumn turns a form field ("contacts.0.mobile", "grp_cds") into the import column that
// holds it ("contact_1_mobile", "item_groups").
func ImportColumn(field string) string {
	if field == "grp_cds" {
		return "item_groups"
	}
	if rest, ok := strings.CutPrefix(field, "contacts."); ok {
		if i, part, ok := strings.Cut(rest, "."); ok {
			if n, err := strconv.Atoi(i); err == nil {
				return "contact_" + strconv.Itoa(n+1) + "_" + part
			}
		}
	}
	return field
}
