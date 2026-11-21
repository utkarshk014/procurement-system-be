package usecase_vendor

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
	domain_vendor "github.com/utkarshk014/procurement-system-be/src/internal/domain/vendor"
	"github.com/utkarshk014/procurement-system-be/src/packages/sheet"
)

// splitNames reads "Computers, Printers; Furniture" as three names.
func splitNames(s string) []string {
	var out []string
	for _, part := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ';' }) {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func (uc *vendorUsecase) Import(ctx context.Context, rows [][]string, dryRun bool) (*domain.ImportReport, error) {
	p, err := domain.TenantPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	data, err := sheet.Parse(rows, domain_vendor.ImportFields, "vendors", domain_vendor.MaxImportRows)
	if err != nil {
		return nil, domain.Validation(domain.FieldError{Field: "file", Message: err.Error()})
	}

	groups, err := uc.repo.Groups(ctx)
	if err != nil {
		return nil, err
	}
	groupID := map[string]int64{}
	for _, g := range groups {
		groupID[sheet.NameKey(g.Name)] = g.ID
	}
	existing, err := uc.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	nameUsed, panUsed, gstinUsed := map[string]bool{}, map[string]string{}, map[string]string{}
	for _, v := range existing {
		nameUsed[sheet.NameKey(v.Name)] = true
		if v.PAN != "" {
			panUsed[strings.ToUpper(v.PAN)] = v.Name
		}
		if v.GSTIN != "" {
			gstinUsed[strings.ToUpper(v.GSTIN)] = v.Name
		}
	}

	today := uc.today()
	report := &domain.ImportReport{DryRun: dryRun, Total: len(data), Problems: []domain.ImportProblem{}, Missing: map[string][]domain.NameCount{}}
	missingGroups := domain.MissingNames{}
	firstName, firstPAN, firstGSTIN := map[string]int{}, map[string]int{}, map[string]int{}
	type candidate struct {
		row    int
		vendor domain_vendor.Vendor
	}
	var ready []candidate

	for _, d := range data {
		var errs []domain.FieldError
		flagged := map[string]bool{}
		flag := func(field, msg string) {
			field = domain_vendor.ImportColumn(field)
			if !flagged[field] {
				errs = append(errs, domain.FieldError{Field: field, Message: msg})
				flagged[field] = true
			}
		}
		in := domain_vendor.VendorInput{
			Name: d.Get("name"), VendorType: d.Get("vendor_type"), SupplyType: d.Get("supply_type"),
			Address: d.Get("address"), City: d.Get("city"), State: d.Get("state"), Country: d.Get("country"), Pin: d.Get("pin"),
			Mobile: d.Get("mobile"), Email: d.Get("email"), Website: d.Get("website"),
			RegDate: sheet.Date(d.Get("reg_date")), CompanySince: d.Get("company_since"), ReferredBy: d.Get("refered_by"),
			PAN: d.Get("pan"), GSTIN: d.Get("gstin"), TAN: d.Get("tan"),
			CorrespondEmail: d.Get("correspond_email"), CorrespondMobile: d.Get("correspond_mobile"),
		}
		for i := 1; i <= 3; i++ {
			n := "contact_" + strconv.Itoa(i) + "_"
			in.Contacts = append(in.Contacts, domain_vendor.Contact{
				Person: d.Get(n + "person"), Designation: d.Get(n + "designation"), Mobile: d.Get(n + "mobile"), Email: d.Get(n + "email"),
			})
		}
		var ok bool
		if in.IsManufacturer, ok = sheet.Bool(d.Get("is_manufacturer"), false); !ok {
			flag("is_manufacturer", "must be yes or no")
		}
		if in.IsConstruction, ok = sheet.Bool(d.Get("is_contruction_comp"), false); !ok {
			flag("is_contruction_comp", "must be yes or no")
		}
		var unknown []string
		for _, name := range splitNames(d.Get("item_groups")) {
			if id, ok := groupID[sheet.NameKey(name)]; ok {
				in.GroupIDs = append(in.GroupIDs, id)
			} else {
				missingGroups.Add(name)
				unknown = append(unknown, `"`+strings.Join(strings.Fields(name), " ")+`"`)
			}
		}
		if len(unknown) > 0 {
			flag("item_groups", strings.Join(unknown, ", ")+" not in the item groups list")
		}

		v, err := PrepareVendor(in, today)
		if err != nil {
			ae, ok := domain.AsAppError(err)
			if !ok || ae.Kind != domain.KindValidation {
				return nil, err
			}
			for _, f := range ae.Fields {
				flag(f.Field, f.Message)
			}
		}
		name := strings.Join(strings.Fields(in.Name), " ")
		pan, gstin := strings.ToUpper(strings.TrimSpace(in.PAN)), strings.ToUpper(strings.TrimSpace(in.GSTIN))
		dup := func(field, key string, used map[string]string, first map[string]int) {
			if key == "" {
				return
			}
			if by, ok := used[key]; ok {
				flag(field, "is already used by "+by)
			} else if row, ok := first[key]; ok {
				flag(field, "is also in row "+strconv.Itoa(row)+" of this file")
			} else {
				first[key] = d.Number
			}
		}
		if name != "" {
			if nameUsed[sheet.NameKey(name)] {
				flag("name", "is already a vendor")
			} else if row, ok := firstName[sheet.NameKey(name)]; ok {
				flag("name", "is also in row "+strconv.Itoa(row)+" of this file")
			} else {
				firstName[sheet.NameKey(name)] = d.Number
			}
		}
		dup("pan", pan, panUsed, firstPAN)
		dup("gstin", gstin, gstinUsed, firstGSTIN)

		if len(errs) == 0 && v != nil {
			// The DBA's check, one vendor at a time (his function raises instead of returning).
			if err := uc.repo.CheckWithFunction(ctx, v); err != nil {
				ae, ok := domain.AsAppError(err)
				if !ok || ae.Kind == domain.KindInternal {
					return nil, err
				}
				if len(ae.Fields) > 0 {
					for _, f := range ae.Fields {
						flag(f.Field, f.Message)
					}
				} else {
					flag("", ae.Message)
				}
			}
		}
		if len(errs) > 0 || v == nil {
			report.Problems = append(report.Problems, domain.ImportProblem{Row: d.Number, Key: pan, Name: name, Errors: errs})
			continue
		}
		ready = append(ready, candidate{row: d.Number, vendor: *v})
	}

	sort.Slice(report.Problems, func(i, j int) bool { return report.Problems[i].Row < report.Problems[j].Row })
	if len(missingGroups) > 0 {
		report.Missing["item_groups"] = missingGroups.Sorted()
	}
	report.Ready = len(ready)
	report.Skipped = len(report.Problems)

	if !dryRun && len(ready) > 0 {
		list := make([]domain_vendor.Vendor, len(ready))
		for i, c := range ready {
			list[i] = c.vendor
		}
		if err := uc.repo.CreateMany(ctx, list); err != nil {
			return nil, err
		}
		report.Imported = len(list)
		uc.logger.InfoContext(ctx, "vendors imported", "tenant", p.TenantCode, "user_id", p.UserID,
			"imported", report.Imported, "skipped", report.Skipped)
	}
	return report, nil
}

// Export writes every vendor in the import layout.
func (uc *vendorUsecase) Export(ctx context.Context) ([]byte, error) {
	if _, err := domain.TenantPrincipal(ctx); err != nil {
		return nil, err
	}
	vendors, err := uc.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	rows := make([][]string, len(vendors))
	for i, v := range vendors {
		groups := make([]string, len(v.Groups))
		for j, g := range v.Groups {
			groups[j] = g.Name
		}
		reg := ""
		if v.RegDate != nil {
			reg = *v.RegDate
		}
		row := []string{
			v.Name, v.VendorType, v.SupplyType, strings.Join(groups, ", "), v.Address, v.City, v.State, v.Country, v.Pin,
			v.Mobile, v.Email, v.Website, reg, v.CompanySince, v.ReferredBy,
			sheet.YesNo(v.IsManufacturer), sheet.YesNo(v.IsConstruction), v.PAN, v.GSTIN, v.TAN, v.CorrespondEmail, v.CorrespondMobile,
		}
		for _, c := range v.Contacts {
			row = append(row, c.Person, c.Designation, c.Mobile, c.Email)
		}
		rows[i] = row
	}
	data, err := sheet.Export("Vendors", domain_vendor.ImportFields, rows)
	if err != nil {
		return nil, domain.Internal("Could not build the export", err)
	}
	return data, nil
}
