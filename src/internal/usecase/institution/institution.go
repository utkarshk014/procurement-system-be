package usecase_institution

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
	domain_codelist "github.com/utkarshk014/procurement-system-be/src/internal/domain/codelist"
	domain_finyear "github.com/utkarshk014/procurement-system-be/src/internal/domain/finyear"
	domain_institution "github.com/utkarshk014/procurement-system-be/src/internal/domain/institution"
	domain_trust "github.com/utkarshk014/procurement-system-be/src/internal/domain/trust"
	usecase_common "github.com/utkarshk014/procurement-system-be/src/internal/usecase/common"
	"github.com/utkarshk014/procurement-system-be/src/packages/validation"
)

// Lookups are the lists an institution points into.
type Lookups struct {
	Trusts   domain_trust.Repository
	Places   domain_codelist.Repository
	Branches domain_codelist.Repository
	Regions  domain_codelist.Repository
	FinYears domain_finyear.Repository
}

type institutionUsecase struct {
	repo     domain_institution.Repository
	lookups  Lookups
	location *time.Location
	logger   *slog.Logger
	now      func() time.Time
}

// NewInstitutionUsecase builds Form 5's institutions; location decides what "today" is for reg_date.
func NewInstitutionUsecase(repo domain_institution.Repository, lookups Lookups, location *time.Location, logger *slog.Logger) domain_institution.Usecase {
	return &institutionUsecase{repo: repo, lookups: lookups, location: location, logger: logger, now: time.Now}
}

func (uc *institutionUsecase) List(ctx context.Context, f domain_institution.Filter) (*domain_institution.Page, error) {
	if _, err := domain.TenantPrincipal(ctx); err != nil {
		return nil, err
	}
	f.TrustCode = domain_institution.NormalizeCode(f.TrustCode)
	f.BranchCode = domain_institution.NormalizeCode(f.BranchCode)
	f.PlaceCode = domain_institution.NormalizeCode(f.PlaceCode)
	f.Q = strings.TrimSpace(f.Q)
	if f.Page < 1 {
		f.Page = 1
	}
	switch {
	case f.PageSize < 1:
		f.PageSize = domain_institution.DefaultPageSize
	case f.PageSize > domain_institution.MaxPageSize:
		f.PageSize = domain_institution.MaxPageSize
	}
	return uc.repo.List(ctx, f)
}

func (uc *institutionUsecase) Get(ctx context.Context, code string) (*domain_institution.Institution, error) {
	if _, err := domain.TenantPrincipal(ctx); err != nil {
		return nil, err
	}
	code = domain_institution.NormalizeCode(code)
	if !domain_institution.CodeRe.MatchString(code) {
		return nil, domain_institution.ErrNotFound
	}
	return uc.repo.Get(ctx, code)
}

func (uc *institutionUsecase) NextCodes(ctx context.Context, trustCode, regionCode string) (*domain_institution.NextCodes, error) {
	if _, err := domain.TenantPrincipal(ctx); err != nil {
		return nil, err
	}
	trustCode = domain_institution.NormalizeCode(trustCode)
	regionCode = domain_institution.NormalizeCode(regionCode)
	var fields []domain.FieldError
	if trustCode != "" && !domain_trust.CodeRe.MatchString(trustCode) {
		fields = append(fields, domain.FieldError{Field: "trust_code", Message: "must be exactly 3 letters"})
	}
	if regionCode != "" && !domain_codelist.CodeRe.MatchString(regionCode) {
		fields = append(fields, domain.FieldError{Field: "regn_code", Message: "must be exactly 3 letters"})
	}
	if len(fields) > 0 {
		return nil, domain.Validation(fields...)
	}
	code, reg, err := uc.repo.NextNumbers(ctx, trustCode, regionCode)
	if err != nil {
		return nil, err
	}
	out := &domain_institution.NextCodes{}
	if trustCode != "" && code <= 999 {
		out.Code = fmt.Sprintf("%s-%03d", trustCode, code)
	}
	if regionCode != "" && reg <= 99 {
		out.RegCode = fmt.Sprintf("%s-%02d", regionCode, reg)
	}
	return out, nil
}

func (uc *institutionUsecase) Validate(ctx context.Context, in domain_institution.Input) (*domain_institution.Institution, error) {
	if _, err := domain.TenantPrincipal(ctx); err != nil {
		return nil, err
	}
	i, err := uc.prepare(ctx, in)
	if err != nil {
		return nil, err
	}
	// The same checks Create runs inside its transaction.
	if field, by, err := uc.repo.Taken(ctx, i); err != nil {
		return nil, err
	} else if field != "" {
		return nil, domain_institution.CodeTaken(field, by)
	}
	res, err := uc.repo.CheckWithFunction(ctx, i)
	if err != nil {
		return nil, err
	}
	if !res.Valid {
		return nil, domain_institution.FunctionError(res.Message)
	}
	return i, nil
}

func (uc *institutionUsecase) Create(ctx context.Context, in domain_institution.Input) (*domain_institution.Institution, error) {
	p, err := domain.TenantPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	i, err := uc.prepare(ctx, in)
	if err != nil {
		return nil, err
	}
	if err := uc.repo.Create(ctx, i); err != nil {
		return nil, err
	}
	uc.logger.InfoContext(ctx, "institution created", "tenant", p.TenantCode, "user_id", p.UserID, "inst_code", i.Code)
	return uc.repo.Get(ctx, i.Code)
}

// prepare normalises the body, checks every rule and every lookup, and returns all field
// problems in one 422. A missing reg_date means today.
func (uc *institutionUsecase) prepare(ctx context.Context, in domain_institution.Input) (*domain_institution.Institution, error) {
	today := uc.now().In(uc.location).Format(domain_institution.DateLayout)
	in = Normalize(in)
	fields := Check(in, today)
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

	// Every code must point at a row that exists.
	if in.TrustCode != "" && !has["trust_code"] {
		if _, err := uc.lookups.Trusts.Get(ctx, in.TrustCode); err != nil {
			if !domain.IsKind(err, domain.KindNotFound) {
				return nil, err
			}
			add("trust_code", "must be an existing trust")
		}
	}
	lookups := []struct {
		field, code string
		repo        domain_codelist.Repository
		what        string
	}{
		{"place_code", in.PlaceCode, uc.lookups.Places, "place"},
		{"branch_code", in.BranchCode, uc.lookups.Branches, "branch"},
	}
	if m := domain_institution.RegCodeRe.FindStringSubmatch(in.RegCode); m != nil {
		lookups = append(lookups, struct {
			field, code string
			repo        domain_codelist.Repository
			what        string
		}{"inst_reg_code", m[1], uc.lookups.Regions, "region code (" + m[1] + " is not one)"})
	}
	for _, l := range lookups {
		if l.code == "" || has[l.field] {
			continue
		}
		if _, err := l.repo.Get(ctx, l.code); err != nil {
			if !domain.IsKind(err, domain.KindNotFound) {
				return nil, err
			}
			msg := "must be an existing " + l.what
			if l.field == "inst_reg_code" {
				msg = "must start with an existing " + l.what
			}
			add(l.field, msg)
		}
	}
	if in.FromFinYr != "" && !has["from_fin_yr"] {
		if _, err := uc.lookups.FinYears.Get(ctx, in.FromFinYr); err != nil {
			if !domain.IsKind(err, domain.KindNotFound) {
				return nil, err
			}
			add("from_fin_yr", "must be an existing financial year")
		}
	}
	if len(fields) > 0 {
		return nil, domain.Validation(fields...)
	}

	regDate := in.RegDate
	if regDate == "" {
		regDate = today
	}
	return &domain_institution.Institution{
		Code: in.Code, RegCode: in.RegCode, FullCode: domain_institution.FullCode(in.Code, in.RegCode),
		Name: in.Name, Address: in.Address, Locality: in.Locality,
		PlaceCode: in.PlaceCode, BranchCode: in.BranchCode, TrustCode: in.TrustCode,
		RegDate: regDate, FromFinYr: in.FromFinYr,
		Email: in.Email, Tel: in.Tel,
		HeadName: in.HeadName, HeadDesig: in.HeadDesig, HeadMobile: in.HeadMobile, HeadEmail: in.HeadEmail,
		AcntName: in.AcntName, AcntMobile: in.AcntMobile, AcntEmail: in.AcntEmail,
		LiasonName: in.LiasonName, LiasonDesig: in.LiasonDesig, LiasonMobile: in.LiasonMobile, LiasonEmail: in.LiasonEmail,
	}, nil
}

// Normalize trims text, upper-cases codes, stores mobiles as 10 digits and emails in lower case.
func Normalize(in domain_institution.Input) domain_institution.Input {
	code := domain_institution.NormalizeCode
	text := func(s string) string { return strings.Join(strings.Fields(s), " ") }
	email := func(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
	mobile := func(s string) string {
		if s = strings.TrimSpace(s); s == "" {
			return ""
		}
		return validation.NormalizeIndianMobile(s)
	}
	in.TrustCode, in.Code, in.RegCode = code(in.TrustCode), code(in.Code), code(in.RegCode)
	in.PlaceCode, in.BranchCode = code(in.PlaceCode), code(in.BranchCode)
	in.FromFinYr, in.RegDate = strings.TrimSpace(in.FromFinYr), strings.TrimSpace(in.RegDate)
	in.Name, in.Address, in.Locality = text(in.Name), text(in.Address), text(in.Locality)
	in.Tel = text(in.Tel)
	in.HeadName, in.HeadDesig = text(in.HeadName), text(in.HeadDesig)
	in.AcntName = text(in.AcntName)
	in.LiasonName, in.LiasonDesig = text(in.LiasonName), text(in.LiasonDesig)
	in.Email, in.HeadEmail, in.AcntEmail, in.LiasonEmail = email(in.Email), email(in.HeadEmail), email(in.AcntEmail), email(in.LiasonEmail)
	in.HeadMobile, in.AcntMobile, in.LiasonMobile = mobile(in.HeadMobile), mobile(in.AcntMobile), mobile(in.LiasonMobile)
	return in
}

// Check returns every rule the normalised input breaks that needs no database; today is
// YYYY-MM-DD in the tenant's zone.
func Check(in domain_institution.Input, today string) []domain.FieldError {
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
	if in.TrustCode != "" && !domain_trust.CodeRe.MatchString(in.TrustCode) {
		add("trust_code", "must be exactly 3 letters")
	}
	if in.Code != "" {
		m := domain_institution.CodeRe.FindStringSubmatch(in.Code)
		switch {
		case m == nil:
			add("inst_code", "must be the trust code, a dash and 3 digits, e.g. SAT-001")
		case in.TrustCode != "" && m[1] != in.TrustCode:
			add("inst_code", "must start with the trust code "+in.TrustCode+"-")
		}
	}
	if in.RegCode != "" && !domain_institution.RegCodeRe.MatchString(in.RegCode) {
		add("inst_reg_code", "must be a region code, a dash and 2 digits, e.g. MYS-01")
	}
	for _, c := range []struct{ field, code string }{{"place_code", in.PlaceCode}, {"branch_code", in.BranchCode}} {
		if c.code != "" && !domain_codelist.CodeRe.MatchString(c.code) {
			add(c.field, "must be exactly 3 letters")
		}
	}
	if in.RegDate != "" && !has["reg_date"] && in.RegDate > today {
		add("reg_date", "cannot be in the future")
	}
	return fields
}
