package usecase_stock

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
	domain_stock "github.com/utkarshk014/procurement-system-be/src/internal/domain/stock"
	"github.com/utkarshk014/procurement-system-be/src/packages/sheet"
)

// names maps a list's names (compared ignoring case and spaces) to ids.
type names map[string]int64

func entryNames(entries []domain_stock.Entry) names {
	n := names{}
	for _, e := range entries {
		n[sheet.NameKey(e.Name)] = e.ID
	}
	return n
}

// itemLinkColumns are the import's list columns, the field each fills and the list it names.
var itemLinkColumns = []struct{ header, field, list string }{
	{"item_group", "grp_cd", "item groups"},
	{"category", "cat_cd", "categories"},
	{"manufacturer", "manf_cd", "manufacturers"},
	{"uom", "uom_cd", "units"},
}

func (uc *itemUsecase) Import(ctx context.Context, rows [][]string, dryRun bool) (*domain.ImportReport, error) {
	p, err := domain.TenantPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	data, err := sheet.Parse(rows, domain_stock.ItemFields, "items", domain_stock.MaxItemImportRows)
	if err != nil {
		return nil, domain.Validation(domain.FieldError{Field: "file", Message: err.Error()})
	}

	// ---- the lists and the items there already, loaded once
	lists := map[string]names{}
	for header, repo := range map[string]domain_stock.ListRepository{
		"item_group": uc.lookups.Groups, "category": uc.lookups.Categories, "manufacturer": uc.lookups.Manufacturers,
	} {
		entries, err := repo.List(ctx)
		if err != nil {
			return nil, err
		}
		lists[header] = entryNames(entries)
	}
	uoms, err := uc.lookups.UOMs.List(ctx)
	if err != nil {
		return nil, err
	}
	lists["uom"] = names{}
	for _, u := range uoms {
		lists["uom"][sheet.NameKey(u.UOM)] = u.ID
	}
	existing, err := uc.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	descUsed, codeUsed := map[string]string{}, map[string]string{}
	for _, it := range existing {
		descUsed[sheet.NameKey(it.Desc)] = it.Code
		codeUsed[strings.ToUpper(strings.TrimSpace(it.Code))] = it.Desc
	}

	report := &domain.ImportReport{DryRun: dryRun, Total: len(data), Problems: []domain.ImportProblem{}, Missing: map[string][]domain.NameCount{}}
	missing := map[string]domain.MissingNames{}
	firstDesc, firstCode := map[string]int{}, map[string]int{}
	type candidate struct {
		row  int
		item domain_stock.Item
	}
	var ready []candidate

	for _, d := range data {
		var errs []domain.FieldError
		flagged := map[string]bool{}
		flag := func(field, msg string) {
			if !flagged[field] {
				errs = append(errs, domain.FieldError{Field: field, Message: msg})
				flagged[field] = true
			}
		}
		in := domain_stock.ItemInput{Desc: d.Get("item_desc"), Code: d.Get("item_code"), Type: d.Get("goods_service_item"), HSNCode: d.Get("gst_hsn_code")}

		ids := map[string]*int64{}
		for _, lc := range itemLinkColumns {
			text := d.Get(lc.header)
			if text == "" {
				continue
			}
			if id, ok := lists[lc.header][sheet.NameKey(text)]; ok {
				ids[lc.header] = &id
				continue
			}
			if missing[lc.header] == nil {
				missing[lc.header] = domain.MissingNames{}
			}
			missing[lc.header].Add(text)
			flag(lc.field, `"`+strings.Join(strings.Fields(text), " ")+`" is not in the `+lc.list+` list`)
		}
		in.GroupID, in.CategoryID, in.ManufacturerID, in.UOMID = ids["item_group"], ids["category"], ids["manufacturer"], ids["uom"]

		for _, b := range []struct {
			header string
			def    bool
			dst    *bool
		}{{"item_consumable", false, &in.Consumable}, {"item_is_spare", false, &in.Spare}, {"track_serial", false, &in.TrackSerial}} {
			v, ok := sheet.Bool(d.Get(b.header), b.def)
			if !ok {
				flag(b.header, "must be yes or no")
			}
			*b.dst = v
		}
		gst, ok := sheet.Bool(d.Get("gst_applicable"), true)
		if !ok {
			flag("gst_applicable", "must be yes or no")
		}
		in.GSTApplicable = &gst
		for _, n := range []struct {
			header string
			dst    **float64
		}{{"gst_percentage", &in.GST}, {"sgst_percentage", &in.SGST}, {"cgst_percentage", &in.CGST}} {
			v, ok := sheet.Number(d.Get(n.header))
			if !ok {
				flag(n.header, "must be a number")
			}
			*n.dst = v
		}

		it, err := PrepareItem(in)
		if err != nil {
			ae, ok := domain.AsAppError(err)
			if !ok || ae.Kind != domain.KindValidation {
				return nil, err
			}
			for _, f := range ae.Fields {
				flag(f.Field, f.Message)
			}
		}
		desc := domain_stock.NormalizeName(in.Desc)
		code := strings.ToUpper(strings.TrimSpace(in.Code))
		if desc != "" {
			if by, ok := descUsed[sheet.NameKey(desc)]; ok {
				flag("item_desc", "is already used by item "+by)
			} else if first, ok := firstDesc[sheet.NameKey(desc)]; ok {
				flag("item_desc", "is also in row "+strconv.Itoa(first)+" of this file")
			} else {
				firstDesc[sheet.NameKey(desc)] = d.Number
			}
		}
		if code != "" {
			if by, ok := codeUsed[code]; ok {
				flag("item_code", "is already used by "+by)
			} else if first, ok := firstCode[code]; ok {
				flag("item_code", "is also in row "+strconv.Itoa(first)+" of this file")
			} else {
				firstCode[code] = d.Number
			}
		}
		if len(errs) > 0 || it == nil {
			report.Problems = append(report.Problems, domain.ImportProblem{Row: d.Number, Key: code, Name: desc, Errors: errs})
			continue
		}
		ready = append(ready, candidate{row: d.Number, item: *it})
	}

	// ---- the DBA's own check for every row that passed ours, in one round trip
	if len(ready) > 0 {
		list := make([]domain_stock.Item, len(ready))
		for i, c := range ready {
			list[i] = c.item
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
			if ae, ok := domain.AsAppError(domain_stock.ItemFunctionError(res.Message)); ok {
				if len(ae.Fields) > 0 {
					fe = ae.Fields[0]
				} else {
					fe.Message = ae.Message
				}
			}
			report.Problems = append(report.Problems, domain.ImportProblem{Row: ready[i].row, Key: ready[i].item.Code, Name: ready[i].item.Desc, Errors: []domain.FieldError{fe}})
		}
		ready = passed
	}

	sort.Slice(report.Problems, func(i, j int) bool { return report.Problems[i].Row < report.Problems[j].Row })
	for header, m := range missing {
		report.Missing[header] = m.Sorted()
	}
	report.Ready = len(ready)
	report.Skipped = len(report.Problems)

	if !dryRun && len(ready) > 0 {
		list := make([]domain_stock.Item, len(ready))
		for i, c := range ready {
			list[i] = c.item
		}
		if err := uc.repo.CreateMany(ctx, list); err != nil {
			return nil, err
		}
		report.Imported = len(list)
		uc.logger.InfoContext(ctx, "items imported", "tenant", p.TenantCode, "user_id", p.UserID,
			"imported", report.Imported, "skipped", report.Skipped)
	}
	return report, nil
}

// Export writes every item in the import layout, so the file can be edited and imported again
// (rows whose code is already used are reported, not duplicated).
func (uc *itemUsecase) Export(ctx context.Context) ([]byte, error) {
	if _, err := domain.TenantPrincipal(ctx); err != nil {
		return nil, err
	}
	items, err := uc.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	num := func(v *float64) string {
		if v == nil {
			return ""
		}
		return strconv.FormatFloat(*v, 'f', -1, 64)
	}
	rows := make([][]string, len(items))
	for i, it := range items {
		rows[i] = []string{
			it.Desc, it.Code, it.Type, it.GroupName, it.UOMName, it.CategoryName, it.ManufacturerName,
			sheet.YesNo(it.GSTApplicable), it.HSNCode, num(it.GST), num(it.SGST), num(it.CGST),
			sheet.YesNo(it.Consumable), sheet.YesNo(it.Spare), sheet.YesNo(it.TrackSerial),
		}
	}
	data, err := sheet.Export("Items", domain_stock.ItemFields, rows)
	if err != nil {
		return nil, domain.Internal("Could not build the export", err)
	}
	return data, nil
}
