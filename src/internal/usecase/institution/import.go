package usecase_institution

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
	domain_codelist "github.com/utkarshk014/procurement-system-be/src/internal/domain/codelist"
	domain_institution "github.com/utkarshk014/procurement-system-be/src/internal/domain/institution"
)

// memRefs answers lookups from lists loaded once, so a 571-row file costs five queries, not 2,855.
type memRefs map[string]map[string]bool

func (m memRefs) exists(_ context.Context, list, code string) (bool, error) {
	return m[list][code], nil
}

// headerKey makes "Inst Code", "inst_code" and "INSTCODE" the same key.
func headerKey(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// nameKey compares names ignoring case and runs of spaces, like the code-list duplicate check.
func nameKey(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), " ")) }

// resolver finds a place or branch by its code or by its name.
type resolver struct {
	codes  map[string]bool
	byName map[string]string
}

func newResolver(entries []domain_codelist.Entry) resolver {
	r := resolver{codes: map[string]bool{}, byName: map[string]string{}}
	for _, e := range entries {
		r.codes[e.Code] = true
		r.byName[nameKey(e.Name)] = e.Code
	}
	return r
}

// find returns the code, and whether the text was a code (rather than a name).
func (r resolver) find(text string) (code string, wasCode, ok bool) {
	if c := strings.ToUpper(strings.TrimSpace(text)); r.codes[c] {
		return c, true, true
	}
	c, ok := r.byName[nameKey(text)]
	return c, false, ok
}

// importDate accepts YYYY-MM-DD, DD-MM-YYYY and DD/MM/YYYY; anything else is passed through for
// the usual date check to reject.
func importDate(s string) string {
	s = strings.TrimSpace(s)
	for _, layout := range []string{time.DateOnly, "02-01-2006", "02/01/2006", "2/1/2006"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format(time.DateOnly)
		}
	}
	return s
}

func (uc *institutionUsecase) Import(ctx context.Context, rows [][]string, dryRun bool) (*domain_institution.ImportReport, error) {
	p, err := domain.TenantPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	fileErr := func(msg string) error { return domain.Validation(domain.FieldError{Field: "file", Message: msg}) }

	// ---- header: which column holds what
	if len(rows) == 0 {
		return nil, fileErr("The sheet is empty. The first row must be the column headers.")
	}
	index := map[string]int{} // canonical header -> column number
	aliases := map[string]string{}
	for _, c := range domain_institution.ImportColumns {
		aliases[headerKey(c.Header)] = c.Header
		for _, a := range c.Aliases {
			aliases[headerKey(a)] = c.Header
		}
	}
	for i, h := range rows[0] {
		if canon, ok := aliases[headerKey(h)]; ok {
			if _, seen := index[canon]; !seen {
				index[canon] = i
			}
		}
	}
	var missing []string
	for _, c := range domain_institution.ImportColumns {
		if _, ok := index[c.Header]; c.Required && !ok {
			missing = append(missing, c.Header)
		}
	}
	if len(missing) > 0 {
		return nil, fileErr("Missing column(s): " + strings.Join(missing, ", ") + ". Download the template to see every column.")
	}

	// ---- data rows (blank rows are ignored)
	type dataRow struct {
		number int
		cells  []string
	}
	var data []dataRow
	for i, r := range rows[1:] {
		if strings.TrimSpace(strings.Join(r, "")) != "" {
			data = append(data, dataRow{number: i + 2, cells: r})
		}
	}
	if len(data) == 0 {
		return nil, fileErr("The sheet has a header but no institutions under it.")
	}
	if len(data) > domain_institution.MaxImportRows {
		return nil, fileErr("The sheet has more than 5,000 institutions; split it into smaller files.")
	}

	// ---- everything the rows point into, loaded once
	trusts, err := uc.lookups.Trusts.List(ctx)
	if err != nil {
		return nil, err
	}
	places, err := uc.lookups.Places.List(ctx)
	if err != nil {
		return nil, err
	}
	branches, err := uc.lookups.Branches.List(ctx)
	if err != nil {
		return nil, err
	}
	regions, err := uc.lookups.Regions.List(ctx)
	if err != nil {
		return nil, err
	}
	years, err := uc.lookups.FinYears.List(ctx)
	if err != nil {
		return nil, err
	}
	refs := memRefs{refTrust: {}, refPlace: {}, refBranch: {}, refRegion: {}, refYear: {}}
	for _, t := range trusts {
		refs[refTrust][t.Code] = true
	}
	for _, e := range places {
		refs[refPlace][e.Code] = true
	}
	for _, e := range branches {
		refs[refBranch][e.Code] = true
	}
	for _, e := range regions {
		refs[refRegion][e.Code] = true
	}
	defaultYear := ""
	for _, y := range years {
		refs[refYear][y.FinYr] = true
		if y.IsCurrent {
			defaultYear = y.FinYr
		}
	}
	placeOf, branchOf := newResolver(places), newResolver(branches)
	codesInUse, regsInUse, err := uc.repo.ExistingCodes(ctx)
	if err != nil {
		return nil, err
	}

	// ---- every row, every rule
	today := uc.now().In(uc.location).Format(domain_institution.DateLayout)
	report := &domain_institution.ImportReport{DryRun: dryRun, Total: len(data), DefaultYear: defaultYear,
		Problems: []domain_institution.ImportProblem{}, MissingPlaces: []domain_institution.NameCount{},
		MissingBranches: []domain_institution.NameCount{}}
	// Missing names, grouped like the lists compare them (case and spaces ignored); the first
	// spelling seen is the one shown.
	missingPlaces, missingBranches := missingNames{}, missingNames{}
	firstCode, firstReg := map[string]int{}, map[string]int{}

	type candidate struct {
		row  int
		inst domain_institution.Institution
	}
	var ready []candidate

	for _, d := range data {
		get := func(header string) string {
			i, ok := index[header]
			if !ok || i >= len(d.cells) {
				return ""
			}
			return strings.TrimSpace(d.cells[i])
		}
		var errs []domain.FieldError
		flagged := map[string]bool{}
		flag := func(field, msg string) {
			if !flagged[field] {
				errs = append(errs, domain.FieldError{Field: field, Message: msg})
				flagged[field] = true
			}
		}

		in := domain_institution.Input{
			TrustCode: get("trust_code"), Code: get("inst_code"), RegCode: get("inst_reg_code"), Name: get("inst_name"),
			FromFinYr: get("from_fin_yr"), RegDate: importDate(get("reg_date")),
			Address: get("inst_add"), Locality: get("inst_place"), Tel: get("inst_tel"), Email: get("inst_email"),
			HeadName: get("inst_head_name"), HeadDesig: get("inst_head_desig"), HeadMobile: get("inst_head_mobile"), HeadEmail: get("inst_head_email"),
			AcntName: get("inst_acnt_name"), AcntMobile: get("inst_acnt_mobile"), AcntEmail: get("inst_acnt_email"),
			LiasonName: get("inst_liason_name"), LiasonDesig: get("inst_liason_desig"), LiasonMobile: get("inst_liason_mobile"), LiasonEmail: get("inst_liason_email"),
		}
		if in.FromFinYr == "" {
			in.FromFinYr = defaultYear
		}
		// Place and branch may be a code or a name. A name also becomes the locality when the
		// file has none (the old system's sheet has only "B G Nagara", "Kaggere, Kunigal", …).
		if text := get("place"); text != "" {
			code, wasCode, ok := placeOf.find(text)
			switch {
			case ok:
				in.PlaceCode = code
			default:
				missingPlaces.add(text)
				flag("place_code", `"`+text+`" is not in the places list`)
			}
			if in.Locality == "" && !wasCode {
				in.Locality = text
			}
		}
		if text := get("branch"); text != "" {
			if code, _, ok := branchOf.find(text); ok {
				in.BranchCode = code
			} else {
				missingBranches.add(text)
				flag("branch_code", `"`+text+`" is not in the branches list`)
			}
		}

		inst, err := prepareWith(ctx, in, today, refs)
		if err != nil {
			ae, ok := domain.AsAppError(err)
			if !ok || ae.Kind != domain.KindValidation {
				return nil, err
			}
			for _, f := range ae.Fields {
				flag(f.Field, f.Message)
			}
		}
		code := domain_institution.NormalizeCode(in.Code)
		reg := domain_institution.NormalizeCode(in.RegCode)
		if full := get("inst_full_code"); full != "" && code != "" && reg != "" && !strings.EqualFold(full, domain_institution.FullCode(code, reg)) {
			flag("inst_full_code", "is "+full+", but the ACM and reg codes make "+domain_institution.FullCode(code, reg))
		}
		if short := get("regn_code"); short != "" && len(reg) >= 3 && !strings.EqualFold(short, reg[:3]) {
			flag("regn_code", "is "+short+", but the reg code starts with "+reg[:3])
		}
		if code != "" {
			if by, ok := codesInUse[code]; ok {
				flag("inst_code", "is already used by "+by)
			} else if first, ok := firstCode[code]; ok {
				flag("inst_code", "is also in row "+strconv.Itoa(first)+" of this file")
			} else {
				firstCode[code] = d.number
			}
		}
		if reg != "" {
			if by, ok := regsInUse[reg]; ok {
				flag("inst_reg_code", "is already used by "+by)
			} else if first, ok := firstReg[reg]; ok {
				flag("inst_reg_code", "is also in row "+strconv.Itoa(first)+" of this file")
			} else {
				firstReg[reg] = d.number
			}
		}

		if len(errs) > 0 || inst == nil {
			report.Problems = append(report.Problems, domain_institution.ImportProblem{
				Row: d.number, Code: code, Name: strings.Join(strings.Fields(in.Name), " "), Errors: errs})
			continue
		}
		ready = append(ready, candidate{row: d.number, inst: *inst})
	}

	// ---- the DBA's own check, for every row that passed ours, in one round trip
	if len(ready) > 0 {
		list := make([]domain_institution.Institution, len(ready))
		for i, c := range ready {
			list[i] = c.inst
		}
		results, err := uc.repo.CheckManyWithFunction(ctx, list)
		if err != nil {
			return nil, err
		}
		passed := ready[:0]
		for i, res := range results {
			if res.Valid {
				passed = append(passed, ready[i])
				continue
			}
			fe := domain.FieldError{Message: res.Message}
			if ae, ok := domain.AsAppError(domain_institution.FunctionError(res.Message)); ok && len(ae.Fields) > 0 {
				fe = ae.Fields[0]
			}
			report.Problems = append(report.Problems, domain_institution.ImportProblem{
				Row: ready[i].row, Code: ready[i].inst.Code, Name: ready[i].inst.Name, Errors: []domain.FieldError{fe}})
		}
		ready = passed
	}

	sort.Slice(report.Problems, func(i, j int) bool { return report.Problems[i].Row < report.Problems[j].Row })
	report.MissingPlaces = missingPlaces.sorted()
	report.MissingBranches = missingBranches.sorted()
	report.Ready = len(ready)
	report.Skipped = len(report.Problems)

	if !dryRun && len(ready) > 0 {
		list := make([]domain_institution.Institution, len(ready))
		for i, c := range ready {
			list[i] = c.inst
		}
		if err := uc.repo.CreateMany(ctx, list); err != nil {
			return nil, err
		}
		report.Imported = len(list)
		uc.logger.InfoContext(ctx, "institutions imported", "tenant", p.TenantCode, "user_id", p.UserID,
			"imported", report.Imported, "skipped", report.Skipped)
	}
	return report, nil
}

// missingNames counts names not found in a list, keyed by nameKey.
type missingNames map[string]*domain_institution.NameCount

func (m missingNames) add(text string) {
	k := nameKey(text)
	if m[k] == nil {
		m[k] = &domain_institution.NameCount{Name: strings.Join(strings.Fields(text), " ")}
	}
	m[k].Count++
}

func (m missingNames) sorted() []domain_institution.NameCount {
	out := make([]domain_institution.NameCount, 0, len(m))
	for _, n := range m {
		out = append(out, *n)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Name < out[j].Name
	})
	return out
}
