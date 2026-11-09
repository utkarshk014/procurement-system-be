package usecase_taxdetails

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
	domain_taxdetails "github.com/utkarshk014/procurement-system-be/src/internal/domain/taxdetails"
	usecase_common "github.com/utkarshk014/procurement-system-be/src/internal/usecase/common"
	"github.com/utkarshk014/procurement-system-be/src/packages/validation"
)

type taxDetailsUsecase struct {
	repo     domain_taxdetails.Repository
	location *time.Location
	logger   *slog.Logger
	now      func() time.Time
}

// NewTaxDetailsUsecase builds Form 3; location decides what "today" is for the enabled dates.
func NewTaxDetailsUsecase(repo domain_taxdetails.Repository, location *time.Location, logger *slog.Logger) domain_taxdetails.Usecase {
	return &taxDetailsUsecase{repo: repo, location: location, logger: logger, now: time.Now}
}

func (uc *taxDetailsUsecase) Get(ctx context.Context) (*domain_taxdetails.TaxDetails, error) {
	if _, err := domain.TenantPrincipal(ctx); err != nil {
		return nil, err
	}
	return uc.repo.Get(ctx)
}

func (uc *taxDetailsUsecase) Validate(ctx context.Context, in domain_taxdetails.TaxDetailsInput) (*domain_taxdetails.TaxDetails, error) {
	if _, err := domain.TenantPrincipal(ctx); err != nil {
		return nil, err
	}
	ok, err := uc.repo.CompanyExists(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain_taxdetails.ErrCompanyFirst
	}
	t, pwd, err := uc.prepare(in)
	if err != nil {
		return nil, err
	}
	// Report whether a password would be stored after this save.
	if pwd.Keep {
		stored, err := uc.repo.Get(ctx)
		switch {
		case err == nil:
			t.CommMailPwdSet = stored.CommMailPwdSet
		case !domain.IsKind(err, domain.KindNotFound):
			return nil, err
		}
	} else {
		t.CommMailPwdSet = pwd.Value != ""
	}
	return t, nil
}

func (uc *taxDetailsUsecase) Save(ctx context.Context, in domain_taxdetails.TaxDetailsInput) (*domain_taxdetails.TaxDetails, bool, error) {
	p, err := domain.TenantPrincipal(ctx)
	if err != nil {
		return nil, false, err
	}
	t, pwd, err := uc.prepare(in)
	if err != nil {
		return nil, false, err
	}
	created, err := uc.repo.Save(ctx, t, pwd)
	if err != nil {
		return nil, false, err
	}
	// Never log the password itself.
	uc.logger.InfoContext(ctx, "company tax details saved", "tenant", p.TenantCode, "user_id", p.UserID,
		"sl", domain_taxdetails.SL, "created", created, "mail_password_changed", !pwd.Keep)
	saved, err := uc.repo.Get(ctx)
	if err != nil {
		return nil, false, err
	}
	return saved, created, nil
}

// prepare normalises the input, checks every rule at once and builds the record to save.
func (uc *taxDetailsUsecase) prepare(in domain_taxdetails.TaxDetailsInput) (*domain_taxdetails.TaxDetails, domain_taxdetails.PasswordChange, error) {
	in = Normalize(in)
	today := uc.now().In(uc.location).Format(domain_taxdetails.DateLayout)
	if fields := Check(in, today); len(fields) > 0 {
		return nil, domain_taxdetails.PasswordChange{}, domain.Validation(fields...)
	}

	// Enabled with no date means enabled today.
	if in.PANEnabled && in.PANEnabledDate == nil {
		in.PANEnabledDate = &today
	}
	if in.GSTINEnabled && in.GSTINEnabledDate == nil {
		in.GSTINEnabledDate = &today
	}
	// With a GSTIN, an empty state is filled from its code.
	if in.GSTIN != "" && in.GSTINState == "" {
		in.GSTINState, _ = validation.GSTStateByCode(in.GSTIN[:2])
	}

	pwd := domain_taxdetails.PasswordChange{Keep: in.CommMailPwd == nil}
	if in.CommMailPwd != nil {
		pwd.Value = *in.CommMailPwd
	}
	return &domain_taxdetails.TaxDetails{
		SL:               domain_taxdetails.SL,
		PANEnabled:       in.PANEnabled,
		PANEnabledDate:   in.PANEnabledDate,
		PAN:              in.PAN,
		GSTINEnabled:     in.GSTINEnabled,
		GSTINEnabledDate: in.GSTINEnabledDate,
		GSTIN:            in.GSTIN,
		GSTINState:       in.GSTINState,
		CommMailID:       in.CommMailID,
		CommMobile:       in.CommMobile,
		CommWhatsapp:     in.CommWhatsapp,
	}, pwd, nil
}

// Normalize trims text, upper-cases PAN and GSTIN, lower-cases the email, keeps mobiles as
// 10 digits, spells the GST state officially and turns empty dates into null.
// The password is never trimmed.
func Normalize(in domain_taxdetails.TaxDetailsInput) domain_taxdetails.TaxDetailsInput {
	trim := strings.TrimSpace
	in.PAN = strings.ToUpper(trim(in.PAN))
	in.GSTIN = strings.ToUpper(trim(in.GSTIN))
	in.GSTINState = trim(in.GSTINState)
	if st, ok := validation.CanonicalGSTState(in.GSTINState); ok {
		in.GSTINState = st
	}
	in.CommMailID = strings.ToLower(trim(in.CommMailID))
	in.CommMobile = normaliseMobile(in.CommMobile)
	in.CommWhatsapp = normaliseMobile(in.CommWhatsapp)
	in.PANEnabledDate = trimDate(in.PANEnabledDate)
	in.GSTINEnabledDate = trimDate(in.GSTINEnabledDate)
	return in
}

func normaliseMobile(s string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	return validation.NormalizeIndianMobile(s)
}

func trimDate(d *string) *string {
	if d == nil {
		return nil
	}
	s := strings.TrimSpace(*d)
	if s == "" {
		return nil
	}
	return &s
}

// Check returns every rule the normalised input breaks; today is YYYY-MM-DD in the tenant's zone.
// No field is mandatory, not even the number when PAN/GSTIN is enabled (Uncle, 9 Nov 2026:
// "optional, but if they key PAN or GSTIN we should validate it"); anything entered must be valid.
func Check(in domain_taxdetails.TaxDetailsInput, today string) []domain.FieldError {
	fields := usecase_common.ToFieldErrors(validation.Struct(in))
	has := map[string]bool{}
	for _, f := range fields {
		has[f.Field] = true
	}
	add := func(field, msg string) {
		if !has[field] {
			fields = append(fields, domain.FieldError{Field: field, Message: msg})
			has[field] = true
		}
	}

	// PAN
	if in.PAN != "" && !validation.IsPAN(in.PAN) {
		add("pan", "must be a PAN like ABCDE1234F")
	}
	checkEnabledDate(in.PANEnabled, in.PANEnabledDate, "pan_enabled_dt", "PAN", today, add)

	// GSTIN
	gstinValid := false
	if in.GSTIN != "" {
		switch {
		case !validation.IsGSTIN(in.GSTIN):
			add("gstin", "is not a valid GSTIN")
		default:
			if _, ok := validation.GSTStateByCode(in.GSTIN[:2]); !ok {
				add("gstin", "starts with an unknown GST state code "+in.GSTIN[:2])
			} else {
				gstinValid = true
			}
		}
		if gstinValid && in.PAN != "" && validation.IsPAN(in.PAN) && !validation.GSTINMatchesPAN(in.GSTIN, in.PAN) {
			add("gstin", "PAN inside GSTIN must match the PAN")
		}
	}
	checkEnabledDate(in.GSTINEnabled, in.GSTINEnabledDate, "gstin_enabled_dt", "GSTIN", today, add)

	// GST state: official spelling, and the GSTIN's own state when there is one.
	if in.GSTINState != "" {
		if _, ok := validation.CanonicalGSTState(in.GSTINState); !ok {
			add("gstin_state", "must be a state from the GST state list")
		} else if gstinValid {
			code := in.GSTIN[:2]
			name, _ := validation.GSTStateByCode(code)
			if name != in.GSTINState {
				add("gstin_state", "does not match GSTIN state code "+code+" ("+name+")")
			}
		}
	}

	// Contact
	if in.CommMailID != "" && !validation.IsEmail(in.CommMailID) {
		add("comm_mail_id", "must be a valid email address")
	}
	if in.CommMobile != "" && !validation.IsIndianMobile(in.CommMobile) {
		add("comm_mobile", "must be a 10-digit Indian mobile number")
	}
	if in.CommWhatsapp != "" && !validation.IsIndianMobile(in.CommWhatsapp) {
		add("comm_whatsapp", "must be a 10-digit Indian mobile number")
	}
	return fields
}

// checkEnabledDate: a date is only allowed when the number is enabled, and cannot be after today.
func checkEnabledDate(enabled bool, date *string, field, what, today string, add func(field, msg string)) {
	if date == nil {
		return
	}
	if !enabled {
		add(field, "can only be set when "+what+" is enabled")
		return
	}
	if _, err := time.Parse(domain_taxdetails.DateLayout, *date); err != nil {
		return // the format error is already reported
	}
	if *date > today {
		add(field, "cannot be after today ("+today+")")
	}
}
