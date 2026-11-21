package usecase_vendor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
	domain_vendor "github.com/utkarshk014/procurement-system-be/src/internal/domain/vendor"
	repository_vendor "github.com/utkarshk014/procurement-system-be/src/internal/repository/vendor"
)

const today = "2026-11-21"

func fields(t *testing.T, err error) map[string]string {
	t.Helper()
	app, ok := domain.AsAppError(err)
	require.True(t, ok, "%v", err)
	out := map[string]string{}
	for _, f := range app.Fields {
		out[f.Field] = f.Message
	}
	return out
}

func TestPrepareVendorTidiesInput(t *testing.T) {
	v, err := PrepareVendor(domain_vendor.VendorInput{
		Name: "  ABC   Computers ", PAN: " abcde1234f ", GSTIN: "29abcde1234f1zw", TAN: "blrA12345b",
		Mobile: "080-2345 6789", CorrespondMobile: "+91 98450 12345", Email: " Sales@ABC.in ",
		VendorType: "proprietor", SupplyType: "sales and SERVICE", Pin: "560001", RegDate: "2020-04-01",
		CompanySince: "1998", Website: "https://abc.in/",
		Contacts: []domain_vendor.Contact{{Person: " Ravi  K ", Mobile: "09845012345", Email: "RAVI@abc.in"}},
		GroupIDs: []int64{3, 1, 3},
	}, today)
	require.NoError(t, err)
	assert.Equal(t, "ABC Computers", v.Name)
	assert.Equal(t, "ABCDE1234F", v.PAN, "upper-cased: the DBA's update refuses lower case")
	assert.Equal(t, "29ABCDE1234F1ZW", v.GSTIN)
	assert.Equal(t, "Karnataka", v.GSTINState, "state taken from the GSTIN")
	assert.Equal(t, "BLRA12345B", v.TAN)
	assert.Equal(t, "08023456789", v.Mobile)
	assert.Equal(t, "9845012345", v.CorrespondMobile)
	assert.Equal(t, "sales@abc.in", v.Email)
	assert.Equal(t, "Proprietor", v.VendorType)
	assert.Equal(t, "Sales and service", v.SupplyType)
	assert.Equal(t, "https://abc.in", v.Website)
	assert.Equal(t, domain_vendor.Contact{Person: "Ravi K", Mobile: "9845012345", Email: "ravi@abc.in"}, v.Contacts[0])
	assert.Equal(t, []int64{3, 1}, v.GroupIDs(), "duplicates dropped, order kept")
}

func TestPrepareVendorReportsEveryProblem(t *testing.T) {
	_, err := PrepareVendor(domain_vendor.VendorInput{
		PAN: "ABCDE1234", GSTIN: "29ABCDE1234F1ZX", TAN: "AB12", Mobile: "12345", Email: "x@",
		VendorType: "LLP", SupplyType: "Rent", Pin: "5600", RegDate: "2027-01-01", CompanySince: "3000",
		CorrespondMobile: "12345", Website: "not a site",
		Contacts: []domain_vendor.Contact{{Mobile: "9845012345"}, {Person: "A", Email: "bad"}},
		GroupIDs: []int64{0},
	}, today)
	f := fields(t, err)
	for _, k := range []string{"name", "pan", "gstin", "tan", "mobile", "email", "vendor_type", "supply_type", "pin",
		"reg_date", "company_since", "correspond_mobile", "website", "contacts.0.person", "contacts.1.email", "grp_cds"} {
		assert.Contains(t, f, k)
	}
}

func TestPrepareVendorGSTINMustCarryPAN(t *testing.T) {
	_, err := PrepareVendor(domain_vendor.VendorInput{Name: "X", PAN: "AAPFU0939F", GSTIN: "29ABCDE1234F1ZW"}, today)
	assert.Equal(t, "PAN inside GSTIN must match the PAN", fields(t, err)["gstin"])
}

func TestPrepareVendorForeignPin(t *testing.T) {
	_, err := PrepareVendor(domain_vendor.VendorInput{Name: "X", Country: "Germany", Pin: "10115"}, today)
	assert.NoError(t, err, "a 5-digit postcode is fine outside India")
	_, err = PrepareVendor(domain_vendor.VendorInput{Name: "X", Pin: "10115"}, today)
	assert.Contains(t, fields(t, err), "pin")
}

func TestPrepareFinancial(t *testing.T) {
	turnover, profit := 1250000.5, -3000.0
	f, err := PrepareFinancial(domain_vendor.FinancialInput{FinYear: " 2025-2026 ", Turnover: &turnover, Profit: &profit,
		ITRNo: " ack123 ", ITRDate: "2026-07-31"}, today, "2026-2027")
	require.NoError(t, err)
	assert.Equal(t, "2025-2026", f.FinYear)
	assert.Equal(t, "ACK123", f.ITRNo)
	assert.Equal(t, "2026-07-31", *f.ITRDate)

	neg, tooFine := -1.0, 1.005
	_, err = PrepareFinancial(domain_vendor.FinancialInput{FinYear: "2025-2027", Turnover: &neg, Profit: &tooFine, ITRDate: "2026-12-01"}, today, "2026-2027")
	f2 := fields(t, err)
	assert.Equal(t, "must be two consecutive years, like 2025-2026", f2["fin_year"])
	assert.Equal(t, "cannot be negative", f2["fin_turnover_declared"])
	assert.Equal(t, "may have at most 2 decimals", f2["fin_profit_declared"])
	assert.Equal(t, "cannot be in the future", f2["fin_itr_date"])

	_, err = PrepareFinancial(domain_vendor.FinancialInput{FinYear: "2027-2028"}, today, "2026-2027")
	assert.Contains(t, fields(t, err)["fin_year"], "cannot be after the current financial year")
	_, err = PrepareFinancial(domain_vendor.FinancialInput{FinYear: "25-26"}, today, "2026-2027")
	assert.Equal(t, "must be like 2025-2026", fields(t, err)["fin_year"])
}

func TestPrepareCertificates(t *testing.T) {
	c, err := PrepareCertificates(domain_vendor.Certificates{
		"gst":   {File: " t/vendors/1/a.pdf ", Caption: "ignored"},
		"oth_1": {File: "t/vendors/1/b.pdf", Caption: "  ISO  9001 "},
	})
	require.NoError(t, err)
	assert.Equal(t, domain_vendor.Certificate{File: "t/vendors/1/a.pdf"}, c["gst"], "only other slots have a caption")
	assert.Equal(t, "ISO 9001", c["oth_1"].Caption)
	assert.Len(t, c, len(domain_vendor.CertificateSlots), "every slot present")

	_, err = PrepareCertificates(domain_vendor.Certificates{"oth_2": {File: "k"}, "bank": {File: "k"}})
	f := fields(t, err)
	assert.Contains(t, f, "oth_2.caption")
	assert.Contains(t, f, "bank")
}

func TestSafeName(t *testing.T) {
	assert.Equal(t, "GST-certificate-2024", safeName(`C:\Users\me\GST certificate (2024).pdf`))
	assert.Equal(t, "file", safeName("..pdf"))
}

func TestDBAMessagesMapToFields(t *testing.T) {
	cases := []struct {
		msg, field string
		kind       domain.ErrorKind
	}{
		{`Vendor name ""ABC"" already exists.`, "name", domain.KindConflict},
		{"PAN already registered: ABCDE1234F", "pan", domain.KindConflict},
		{"Invalid GSTIN: 29X", "gstin", domain.KindValidation},
		{"GSTIN does not match the supplied PAN.", "gstin", domain.KindValidation},
		{"Invalid TAN: X", "tan", domain.KindValidation},
		{"Invalid vendor mobile number.", "mobile", domain.KindValidation},
		{"correspond_mobile must contain exactly 10 digits.", "correspond_mobile", domain.KindValidation},
		{"contact_2_mobile must contain exactly 10 digits.", "contacts.1.mobile", domain.KindValidation},
		{"Invalid email address in contact_3_email.", "contacts.2.email", domain.KindValidation},
		{"Invalid email address in email.", "email", domain.KindValidation},
		{"Company since must be a four-digit year.", "company_since", domain.KindValidation},
	}
	for _, c := range cases {
		app, ok := domain.AsAppError(repository_vendor.FunctionError(c.msg))
		require.True(t, ok)
		assert.Equal(t, c.kind, app.Kind, c.msg)
		require.Len(t, app.Fields, 1)
		assert.Equal(t, c.field, app.Fields[0].Field, c.msg)
		assert.NotContains(t, app.Fields[0].Message, `""`, "doubled quotes cleaned")
	}
	assert.Equal(t, domain_vendor.ErrVendorNotFound, repository_vendor.FunctionError("Vendor code 7 does not exist"))
}

func TestDownloadName(t *testing.T) {
	assert.Equal(t, "GST-certificate.pdf", downloadName("esis/vendors/3/9f86d081884c7d65-GST-certificate.pdf"))
	assert.Equal(t, "odd.pdf", downloadName("esis/vendors/3/odd.pdf"))
}

func TestImportColumnNames(t *testing.T) {
	assert.Equal(t, "contact_2_mobile", domain_vendor.ImportColumn("contacts.1.mobile"))
	assert.Equal(t, "item_groups", domain_vendor.ImportColumn("grp_cds"))
	assert.Equal(t, "pan", domain_vendor.ImportColumn("pan"))
	assert.Equal(t, []string{"Computers", "Printers", "Furniture"}, splitNames(" Computers, Printers;Furniture ,"))
}
