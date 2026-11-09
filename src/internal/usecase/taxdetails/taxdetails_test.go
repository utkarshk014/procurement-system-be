package usecase_taxdetails

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
	domain_taxdetails "github.com/utkarshk014/procurement-system-be/src/internal/domain/taxdetails"
)

const today = "2026-11-09"

func str(s string) *string { return &s }

func validInput() domain_taxdetails.TaxDetailsInput {
	return domain_taxdetails.TaxDetailsInput{
		PANEnabled: true, PANEnabledDate: str("2026-04-01"), PAN: " abcde1234f ",
		GSTINEnabled: true, GSTINEnabledDate: str("2026-04-01"), GSTIN: "29abcde1234f1zw", GSTINState: "karnataka",
		CommMailID: " Accounts@ESIS.in ", CommMailPwd: str(" secret pass "),
		CommMobile: "+91 98765-43210", CommWhatsapp: "09876543210",
	}
}

func checkFields(in domain_taxdetails.TaxDetailsInput) map[string]string {
	out := map[string]string{}
	for _, f := range Check(Normalize(in), today) {
		out[f.Field] = f.Message
	}
	return out
}

func fieldsOf(err error) map[string]string {
	out := map[string]string{}
	if ae, ok := domain.AsAppError(err); ok {
		for _, f := range ae.Fields {
			out[f.Field] = f.Message
		}
	}
	return out
}

func TestNormalize(t *testing.T) {
	n := Normalize(validInput())
	assert.Equal(t, "ABCDE1234F", n.PAN)
	assert.Equal(t, "29ABCDE1234F1ZW", n.GSTIN)
	assert.Equal(t, "Karnataka", n.GSTINState)
	assert.Equal(t, "accounts@esis.in", n.CommMailID)
	assert.Equal(t, "9876543210", n.CommMobile)
	assert.Equal(t, "9876543210", n.CommWhatsapp)
	assert.Equal(t, " secret pass ", *n.CommMailPwd, "the password is never trimmed")

	empty := Normalize(domain_taxdetails.TaxDetailsInput{PANEnabledDate: str("  "), CommMobile: "  "})
	assert.Nil(t, empty.PANEnabledDate, "an empty date is null")
	assert.Equal(t, "", empty.CommMobile)
}

func TestNothingIsMandatory(t *testing.T) {
	assert.Empty(t, Check(Normalize(domain_taxdetails.TaxDetailsInput{}), today))
	assert.Empty(t, Check(Normalize(validInput()), today))
}

func TestCheckRejectsMobilesWithNoDigits(t *testing.T) {
	f := checkFields(domain_taxdetails.TaxDetailsInput{CommMobile: "abc", CommWhatsapp: "--"})
	assert.Equal(t, "must be a 10-digit Indian mobile number", f["comm_mobile"])
	assert.Equal(t, "must be a 10-digit Indian mobile number", f["comm_whatsapp"])
}

func TestCheckReportsEveryProblemAtOnce(t *testing.T) {
	in := domain_taxdetails.TaxDetailsInput{
		PAN: "ABC123", GSTIN: "29ABCDE1234F1ZX", GSTINState: "Atlantis",
		CommMailID: "x@", CommMobile: "12345", CommWhatsapp: "5876543210",
		PANEnabledDate: str("2026-04-01"), GSTINEnabledDate: str("01-04-2026"),
	}
	f := checkFields(in)
	assert.Equal(t, "must be a PAN like ABCDE1234F", f["pan"])
	assert.Equal(t, "is not a valid GSTIN", f["gstin"])
	assert.Equal(t, "must be a state from the GST state list", f["gstin_state"])
	assert.Equal(t, "must be a valid email address", f["comm_mail_id"])
	assert.Equal(t, "must be a 10-digit Indian mobile number", f["comm_mobile"])
	assert.Equal(t, "must be a 10-digit Indian mobile number", f["comm_whatsapp"])
	assert.Equal(t, "can only be set when PAN is enabled", f["pan_enabled_dt"])
	assert.Equal(t, "must be a date in YYYY-MM-DD format", f["gstin_enabled_dt"])
}

// Uncle, 9 Nov 2026: the numbers are optional even when enabled, but validated when keyed.
func TestEnabledDoesNotRequireTheNumber(t *testing.T) {
	assert.Empty(t, checkFields(domain_taxdetails.TaxDetailsInput{PANEnabled: true, GSTINEnabled: true}))

	f := checkFields(domain_taxdetails.TaxDetailsInput{PANEnabled: true, PAN: "bad", GSTINEnabled: true, GSTIN: "29ABCDE1234F1ZX"})
	assert.Equal(t, "must be a PAN like ABCDE1234F", f["pan"])
	assert.Equal(t, "is not a valid GSTIN", f["gstin"])
}

func TestEnabledDateCannotBeInTheFuture(t *testing.T) {
	in := validInput()
	in.PANEnabledDate = str("2026-11-10")
	in.GSTINEnabledDate = str(today)
	f := checkFields(in)
	assert.Equal(t, "cannot be after today (2026-11-09)", f["pan_enabled_dt"])
	assert.NotContains(t, f, "gstin_enabled_dt", "today itself is fine")
}

func TestGSTINRules(t *testing.T) {
	in := validInput()
	in.PAN = "ABCDE1234G"
	assert.Equal(t, "PAN inside GSTIN must match the PAN", checkFields(in)["gstin"])

	in = validInput()
	in.GSTINState = "Kerala"
	assert.Equal(t, "does not match GSTIN state code 29 (Karnataka)", checkFields(in)["gstin_state"])

	in = validInput()
	in.GSTIN = "25ABCDE1234F1Z" // retired code 25, with a valid check digit appended below
	in.GSTIN += string(checkDigit(in.GSTIN))
	assert.Equal(t, "starts with an unknown GST state code 25", checkFields(in)["gstin"])

	in = validInput()
	in.PAN = ""
	in.PANEnabled = false
	in.PANEnabledDate = nil
	assert.Empty(t, checkFields(in), "a GSTIN without a PAN is fine")
}

func TestColumnLimits(t *testing.T) {
	in := validInput()
	in.CommMailPwd = str(strings.Repeat("p", 151))
	in.GSTINState = strings.Repeat("s", 101)
	f := checkFields(in)
	assert.Equal(t, "must be at most 150 characters", f["comm_mail_pwd"])
	assert.Equal(t, "must be at most 100 characters", f["gstin_state"])
}

// checkDigit mirrors validation.gstinCheckDigit for building test GSTINs.
func checkDigit(first14 string) byte {
	const cs = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	sum := 0
	for i := 0; i < 14; i++ {
		p := strings.IndexByte(cs, first14[i]) * (1 + i%2)
		sum += p/36 + p%36
	}
	return cs[(36-sum%36)%36]
}

// ---- use case with a fake repository ----------------------------------------

type fakeRepo struct {
	companyExists bool
	stored        *domain_taxdetails.TaxDetails
	password      *string
}

func (f *fakeRepo) Get(context.Context) (*domain_taxdetails.TaxDetails, error) {
	if f.stored == nil {
		return nil, domain.NotFound("Company tax details are not saved yet")
	}
	t := *f.stored
	t.CommMailPwdSet = f.password != nil && *f.password != ""
	return &t, nil
}

func (f *fakeRepo) CompanyExists(context.Context) (bool, error) { return f.companyExists, nil }

func (f *fakeRepo) Save(_ context.Context, t *domain_taxdetails.TaxDetails, pwd domain_taxdetails.PasswordChange) (bool, error) {
	if !f.companyExists {
		return false, domain_taxdetails.ErrCompanyFirst
	}
	created := f.stored == nil
	cp := *t
	f.stored = &cp
	if !pwd.Keep {
		v := pwd.Value
		f.password = &v
	}
	return created, nil
}

func tenantCtx() context.Context {
	return domain.WithPrincipal(context.Background(), &domain.Principal{
		UserID: 7, Role: domain.RoleTenantAdmin, TenantID: 1, TenantCode: "ESIS", DBKey: "esis",
	})
}

func newAt(repo *fakeRepo) *taxDetailsUsecase {
	ist := time.FixedZone("IST", 5*3600+1800)
	uc := NewTaxDetailsUsecase(repo, ist, slog.New(slog.NewTextHandler(io.Discard, nil))).(*taxDetailsUsecase)
	uc.now = func() time.Time { return time.Date(2026, 11, 9, 12, 0, 0, 0, ist) }
	return uc
}

func TestSaveNeedsTheCompanyFirst(t *testing.T) {
	uc := newAt(&fakeRepo{})
	_, _, err := uc.Save(tenantCtx(), validInput())
	assert.Equal(t, domain_taxdetails.ErrCompanyFirst, err)
	_, err = uc.Validate(tenantCtx(), validInput())
	assert.Equal(t, domain_taxdetails.ErrCompanyFirst, err)
}

func TestSaveCreatesThenUpdatesAndKeepsThePassword(t *testing.T) {
	repo := &fakeRepo{companyExists: true}
	uc := newAt(repo)

	saved, created, err := uc.Save(tenantCtx(), validInput())
	require.NoError(t, err)
	assert.True(t, created)
	assert.Equal(t, 1, saved.SL)
	assert.True(t, saved.CommMailPwdSet)
	assert.Equal(t, " secret pass ", *repo.password)

	in := validInput()
	in.CommMailPwd = nil // not sent: keep it
	in.CommMobile = "9123456780"
	saved, created, err = uc.Save(tenantCtx(), in)
	require.NoError(t, err)
	assert.False(t, created)
	assert.True(t, saved.CommMailPwdSet, "a missing password keeps the stored one")
	assert.Equal(t, "9123456780", saved.CommMobile)

	in.CommMailPwd = str("") // "" clears it
	saved, _, err = uc.Save(tenantCtx(), in)
	require.NoError(t, err)
	assert.False(t, saved.CommMailPwdSet)
}

func TestEnabledWithoutDateMeansToday(t *testing.T) {
	repo := &fakeRepo{companyExists: true}
	in := validInput()
	in.PANEnabledDate = nil
	in.GSTINEnabledDate = str("")
	saved, _, err := newAt(repo).Save(tenantCtx(), in)
	require.NoError(t, err)
	require.NotNil(t, saved.PANEnabledDate)
	assert.Equal(t, today, *saved.PANEnabledDate)
	assert.Equal(t, today, *saved.GSTINEnabledDate)
}

func TestEmptyStateIsFilledFromTheGSTIN(t *testing.T) {
	in := validInput()
	in.GSTINState = ""
	saved, _, err := newAt(&fakeRepo{companyExists: true}).Save(tenantCtx(), in)
	require.NoError(t, err)
	assert.Equal(t, "Karnataka", saved.GSTINState)
}

func TestValidateReportsWhetherAPasswordWouldBeStored(t *testing.T) {
	stored := "old"
	repo := &fakeRepo{companyExists: true, stored: &domain_taxdetails.TaxDetails{SL: 1}, password: &stored}
	in := validInput()
	in.CommMailPwd = nil
	got, err := newAt(repo).Validate(tenantCtx(), in)
	require.NoError(t, err)
	assert.True(t, got.CommMailPwdSet)

	in.CommMailPwd = str("")
	got, err = newAt(repo).Validate(tenantCtx(), in)
	require.NoError(t, err)
	assert.False(t, got.CommMailPwdSet)
	assert.Equal(t, "old", *repo.password, "validate writes nothing")
}

func TestValidateReturnsAllFieldErrors(t *testing.T) {
	in := validInput()
	in.PAN = "bad"
	in.CommMailID = "nope"
	_, err := newAt(&fakeRepo{companyExists: true}).Validate(tenantCtx(), in)
	assert.True(t, domain.IsKind(err, domain.KindValidation))
	f := fieldsOf(err)
	assert.Contains(t, f, "pan")
	assert.Contains(t, f, "comm_mail_id")
}

func TestTaxDetailsNeedTenantAccount(t *testing.T) {
	_, err := newAt(&fakeRepo{}).Get(context.Background())
	assert.True(t, domain.IsKind(err, domain.KindUnauthorized))
}
