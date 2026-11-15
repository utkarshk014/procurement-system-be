package usecase_stock

import (
	"math"
	"regexp"
	"strings"

	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
	domain_stock "github.com/utkarshk014/procurement-system-be/src/internal/domain/stock"
	usecase_common "github.com/utkarshk014/procurement-system-be/src/internal/usecase/common"
	"github.com/utkarshk014/procurement-system-be/src/packages/validation"
)

var (
	// itemCodeRe: letters, digits and - / . (no spaces), so codes stay easy to type and search.
	itemCodeRe = regexp.MustCompile(`^[A-Z0-9][A-Z0-9./-]*$`)
	// hsnRe: an HSN (goods) or SAC (services) code is 4, 6 or 8 digits.
	hsnRe = regexp.MustCompile(`^([0-9]{4}|[0-9]{6}|[0-9]{8})$`)
)

// PrepareItem tidies the body and returns every field problem at once.
//
// The DBA's rules (insert procedure and validate function), which we apply first so all
// problems come back together: a description; type Goods or Service; a service is never
// consumable, a spare or serial-tracked; with GST, all three percentages and GST = SGST + CGST;
// without GST, no percentages. Ours on top: the table's lengths, item code and HSN format,
// percentages from 0 to 100 with at most 2 decimals (NUMERIC(5,2)), positive link ids, an item
// code (the DBA's insert requires one since 15 Nov, and his delete matches on it), and a
// group and a UOM on every item (Uncle, 15 Nov: "UOM is must", "group is needed").
func PrepareItem(in domain_stock.ItemInput) (*domain_stock.Item, error) {
	in.Desc = domain_stock.NormalizeName(in.Desc)
	in.Code = strings.ToUpper(strings.TrimSpace(in.Code))
	in.HSNCode = strings.TrimSpace(in.HSNCode)
	in.Type = canonicalType(in.Type)

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

	if in.Code != "" && !itemCodeRe.MatchString(in.Code) {
		add("item_code", "may contain only letters, digits and - / .")
	}
	if in.Type != "" && in.Type != domain_stock.TypeGoods && in.Type != domain_stock.TypeService {
		add("goods_service_item", "must be Goods or Service")
	}
	if in.Type == domain_stock.TypeService {
		if in.Consumable {
			add("item_consumable", "applies to goods only")
		}
		if in.Spare {
			add("item_is_spare", "applies to goods only")
		}
		if in.TrackSerial {
			add("track_serial", "applies to goods only")
		}
	}
	// Group and UOM are required (Uncle, 15 Nov); category and manufacturer are optional.
	if in.GroupID == nil {
		add("grp_cd", "is required")
	}
	if in.UOMID == nil {
		add("uom_cd", "is required")
	}
	for _, link := range domain_stock.ItemLinks(in.GroupID, in.CategoryID, in.ManufacturerID, in.UOMID) {
		if link.ID != nil && *link.ID <= 0 {
			add(link.Field, "does not exist")
		}
	}
	if in.HSNCode != "" && !hsnRe.MatchString(in.HSNCode) {
		add("gst_hsn_code", "must be 4, 6 or 8 digits")
	}

	gstApplicable := in.GSTApplicable == nil || *in.GSTApplicable
	percents := []struct {
		field string
		value *float64
	}{{"gst_percentage", in.GST}, {"sgst_percentage", in.SGST}, {"cgst_percentage", in.CGST}}
	if gstApplicable {
		allValid := true
		for _, p := range percents {
			switch {
			case p.value == nil:
				add(p.field, "is required when GST applies")
				allValid = false
			case *p.value < 0 || *p.value > 100:
				add(p.field, "must be from 0 to 100")
				allValid = false
			case !twoDecimals(*p.value):
				add(p.field, "may have at most 2 decimals")
				allValid = false
			}
		}
		if allValid && cents(*in.GST) != cents(*in.SGST)+cents(*in.CGST) {
			add("gst_percentage", "must equal SGST plus CGST")
		}
	} else {
		for _, p := range percents {
			if p.value != nil && *p.value != 0 {
				add(p.field, "must be empty when GST does not apply")
			}
		}
	}

	if len(fields) > 0 {
		return nil, domain.Validation(fields...)
	}
	it := &domain_stock.Item{
		Desc: in.Desc, Code: in.Code,
		GroupID: in.GroupID, CategoryID: in.CategoryID, ManufacturerID: in.ManufacturerID, UOMID: in.UOMID,
		Type: in.Type, Consumable: in.Consumable, Spare: in.Spare, TrackSerial: in.TrackSerial,
		GSTApplicable: gstApplicable, HSNCode: in.HSNCode,
	}
	if gstApplicable {
		it.GST, it.SGST, it.CGST = in.GST, in.SGST, in.CGST
	}
	return it, nil
}

// canonicalType accepts any case ("goods", "SERVICE") and returns the DBA's spelling.
func canonicalType(s string) string {
	s = strings.TrimSpace(s)
	switch strings.ToLower(s) {
	case "goods":
		return domain_stock.TypeGoods
	case "service":
		return domain_stock.TypeService
	}
	return s
}

// cents compares percentages exactly: 18 = 9 + 9 without floating-point noise.
func cents(v float64) int64 { return int64(math.Round(v * 100)) }

func twoDecimals(v float64) bool { return math.Abs(v*100-math.Round(v*100)) < 1e-6 }
