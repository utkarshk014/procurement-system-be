package usecase_stock

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
	domain_stock "github.com/utkarshk014/procurement-system-be/src/internal/domain/stock"
)

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

func f(v float64) *float64 { return &v }
func i64(v int64) *int64   { return &v }
func b(v bool) *bool       { return &v }
func n(v int) *int         { return &v }

func TestPrepareName(t *testing.T) {
	name, err := PrepareName(domain_stock.EntryInput{Name: "  Room   freshner "})
	require.NoError(t, err)
	assert.Equal(t, "Room freshner", name)

	_, err = PrepareName(domain_stock.EntryInput{Name: "   "})
	assert.Equal(t, "is required", fields(t, err)["name"])
}

func TestPrepareUOM(t *testing.T) {
	u, err := PrepareUOM(domain_stock.UOMInput{UOM: " kg ", Standard: " Kilo  grams ", EnableDecimal: true, NumberDecimal: n(3)})
	require.NoError(t, err)
	assert.Equal(t, domain_stock.UOM{UOM: "KG", Standard: "Kilo grams", EnableDecimal: true, NumberDecimal: 3}, *u)

	u, err = PrepareUOM(domain_stock.UOMInput{UOM: "nos", Standard: "Numbers"})
	require.NoError(t, err)
	assert.Equal(t, 0, u.NumberDecimal, "decimals off means 0")

	cases := []struct {
		in    domain_stock.UOMInput
		field string
		msg   string
	}{
		{domain_stock.UOMInput{Standard: "x"}, "uom", "is required"},
		{domain_stock.UOMInput{UOM: "LITRES", Standard: "x"}, "uom", "must be at most 5 characters"},
		{domain_stock.UOMInput{UOM: "K G", Standard: "x"}, "uom", "must not contain spaces"},
		{domain_stock.UOMInput{UOM: "KG"}, "uom_as_standard", "is required"},
		{domain_stock.UOMInput{UOM: "KG", Standard: "x", NumberDecimal: n(2)}, "number_decimal", "must be 0 when decimals are off"},
		{domain_stock.UOMInput{UOM: "KG", Standard: "x", EnableDecimal: true}, "number_decimal", "is required when decimals are on"},
		{domain_stock.UOMInput{UOM: "KG", Standard: "x", EnableDecimal: true, NumberDecimal: n(0)}, "number_decimal", "must be from 1 to 9"},
		{domain_stock.UOMInput{UOM: "KG", Standard: "x", EnableDecimal: true, NumberDecimal: n(10)}, "number_decimal", "must be from 1 to 9"},
	}
	for _, c := range cases {
		_, err := PrepareUOM(c.in)
		assert.Equal(t, c.msg, fields(t, err)[c.field], "%+v", c.in)
	}
}

func goods() domain_stock.ItemInput {
	return domain_stock.ItemInput{
		Desc: "  Lays  50g ", Code: " lay-50 ", GroupID: i64(1), UOMID: i64(2), Type: "goods",
		Consumable: true, HSNCode: " 1905 ", GST: f(18), SGST: f(9), CGST: f(9),
	}
}

func TestPrepareItemTidiesAndDefaults(t *testing.T) {
	it, err := PrepareItem(goods())
	require.NoError(t, err)
	assert.Equal(t, "Lays 50g", it.Desc)
	assert.Equal(t, "LAY-50", it.Code)
	assert.Equal(t, domain_stock.TypeGoods, it.Type, "any case is accepted, the DBA's spelling is stored")
	assert.Equal(t, "1905", it.HSNCode)
	assert.True(t, it.GSTApplicable, "GST applies unless told otherwise, like the table's default")

	in := goods()
	in.GSTApplicable, in.GST, in.SGST, in.CGST = b(false), f(0), nil, nil
	it, err = PrepareItem(in)
	require.NoError(t, err)
	assert.Nil(t, it.GST, "no GST stores no percentages")

	in = goods()
	in.Type, in.Consumable = "SERVICE", false
	in.HSNCode = "998719"
	it, err = PrepareItem(in)
	require.NoError(t, err)
	assert.Equal(t, domain_stock.TypeService, it.Type)

	in = goods()
	in.GST, in.SGST, in.CGST = f(0.3), f(0.1), f(0.2)
	_, err = PrepareItem(in)
	assert.NoError(t, err, "0.1 + 0.2 = 0.3 in cents, not in float64")
}

func TestPrepareItemRules(t *testing.T) {
	cases := []struct {
		name  string
		edit  func(*domain_stock.ItemInput)
		field string
		msg   string
	}{
		{"no description", func(in *domain_stock.ItemInput) { in.Desc = " " }, "item_desc", "is required"},
		{"code with space", func(in *domain_stock.ItemInput) { in.Code = "LAY 50" }, "item_code", "may contain only letters, digits and - / ."},
		{"long code", func(in *domain_stock.ItemInput) { in.Code = "ABCDEFGHIJKLMNOPQRSTU" }, "item_code", "must be at most 20 characters"},
		{"no type", func(in *domain_stock.ItemInput) { in.Type = "" }, "goods_service_item", "is required"},
		{"bad type", func(in *domain_stock.ItemInput) { in.Type = "Both" }, "goods_service_item", "must be Goods or Service"},
		{"consumable service", func(in *domain_stock.ItemInput) { in.Type = "Service" }, "item_consumable", "applies to goods only"},
		{"spare service", func(in *domain_stock.ItemInput) { in.Type, in.Consumable, in.Spare = "Service", false, true }, "item_is_spare", "applies to goods only"},
		{"serial service", func(in *domain_stock.ItemInput) { in.Type, in.Consumable, in.TrackSerial = "Service", false, true }, "track_serial", "applies to goods only"},
		{"zero group id", func(in *domain_stock.ItemInput) { in.GroupID = i64(0) }, "grp_cd", "does not exist"},
		{"no group", func(in *domain_stock.ItemInput) { in.GroupID = nil }, "grp_cd", "is required"},
		{"no UOM", func(in *domain_stock.ItemInput) { in.UOMID = nil }, "uom_cd", "is required"},
		{"short HSN", func(in *domain_stock.ItemInput) { in.HSNCode = "190" }, "gst_hsn_code", "must be 4, 6 or 8 digits"},
		{"HSN letters", func(in *domain_stock.ItemInput) { in.HSNCode = "19A5" }, "gst_hsn_code", "must be 4, 6 or 8 digits"},
		{"missing SGST", func(in *domain_stock.ItemInput) { in.SGST = nil }, "sgst_percentage", "is required when GST applies"},
		{"over 100", func(in *domain_stock.ItemInput) { in.GST = f(118) }, "gst_percentage", "must be from 0 to 100"},
		{"three decimals", func(in *domain_stock.ItemInput) { in.CGST = f(9.005) }, "cgst_percentage", "may have at most 2 decimals"},
		{"sum", func(in *domain_stock.ItemInput) { in.GST = f(12) }, "gst_percentage", "must equal SGST plus CGST"},
		{"no GST with rate", func(in *domain_stock.ItemInput) { in.GSTApplicable = b(false) }, "gst_percentage", "must be empty when GST does not apply"},
	}
	for _, c := range cases {
		in := goods()
		c.edit(&in)
		_, err := PrepareItem(in)
		assert.Equal(t, c.msg, fields(t, err)[c.field], c.name)
	}

	// Every problem at once, not the first one only (the DBA's check stops at its first).
	in := goods()
	in.Desc, in.HSNCode, in.SGST = "", "1", nil
	assert.Len(t, fields(t, func() error { _, err := PrepareItem(in); return err }()), 3)
}

func TestFunctionErrorsMapToFields(t *testing.T) {
	cases := []struct {
		err   error
		field string
		kind  domain.ErrorKind
	}{
		{domain_stock.Group.FunctionError("Item group already exists"), "name", domain.KindConflict},
		{domain_stock.Category.FunctionError("Category cannot be empty"), "name", domain.KindValidation},
		{domain_stock.UOMFunctionError("UOM already exists"), "uom", domain.KindConflict},
		{domain_stock.UOMFunctionError("Standard UOM already exists"), "uom_as_standard", domain.KindConflict},
		{domain_stock.UOMFunctionError("Number decimal must be a single digit from 0 to 9"), "number_decimal", domain.KindValidation},
		{domain_stock.ItemFunctionError("Item type must be Goods or Service"), "goods_service_item", domain.KindValidation},
		{domain_stock.ItemFunctionError("Unit of measurement not found"), "uom_cd", domain.KindValidation},
		{domain_stock.ItemFunctionError("GST must equal SGST plus CGST"), "gst_percentage", domain.KindValidation},
	}
	for _, c := range cases {
		app, ok := domain.AsAppError(c.err)
		require.True(t, ok)
		assert.Equal(t, c.kind, app.Kind, c.err.Error())
		require.Len(t, app.Fields, 1)
		assert.Equal(t, c.field, app.Fields[0].Field, c.err.Error())
	}
	assert.Equal(t, domain_stock.ErrItemNotFound, domain_stock.ItemFunctionError("Stock item record not found"))
}
