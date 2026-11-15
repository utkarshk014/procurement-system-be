// Package domain_stock is the stock / inventory masters: item groups, categories,
// manufacturers, units of measurement (UOM) and items. The DBA's tables number their rows
// themselves (INT GENERATED ALWAYS AS IDENTITY), so every row is addressed by its id.
package domain_stock

import (
	"strconv"
	"strings"
	"time"

	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
)

// FunctionSuccess is what the DBA's validate functions return in message for a valid row.
const FunctionSuccess = "Validation successful"

// List names one of the three one-field lists and the DBA objects behind it. Every field is
// a constant, never input.
type List struct {
	Label    string // "Item group": our own messages
	Plural   string // "item groups"
	Table    string // pms.mst_stk_group
	IDCol    string // grp_cd
	NameCol  string // item_group
	Insert   string // pms.insert_mst_stk_group(name)
	Validate string // pms.fn_validate_mst_stk_group(id, name)
}

var (
	Group = List{Label: "Item group", Plural: "item groups", Table: "pms.mst_stk_group", IDCol: "grp_cd", NameCol: "item_group",
		Insert: "pms.insert_mst_stk_group", Validate: "pms.fn_validate_mst_stk_group"}
	Category = List{Label: "Category", Plural: "categories", Table: "pms.mst_stk_category", IDCol: "cat_cd", NameCol: "category",
		Insert: "pms.insert_mst_stk_category", Validate: "pms.fn_validate_mst_stk_category"}
	Manufacturer = List{Label: "Manufacturer", Plural: "manufacturers", Table: "pms.mst_stk_manufacturer", IDCol: "manf_cd", NameCol: "manufacturer",
		Insert: "pms.insert_mst_stk_manufacturer", Validate: "pms.fn_validate_mst_stk_manufacturer"}
)

// Entry is one row of a one-field list.
type Entry struct {
	ID        int64      `json:"id"`
	Name      string     `json:"name"`
	CreatedAt *time.Time `json:"rec_created_at,omitempty"`
	UpdatedAt *time.Time `json:"last_update_at,omitempty"`
}

// EntryInput is the body of POST /stock/groups (/categories, /manufacturers) and /validate.
type EntryInput struct {
	Name string `json:"name" validate:"required,max=250"`
}

// NotFound is the 404 for an unknown id.
func (l List) NotFound() error { return domain.NotFound(l.Label + " not found") }

// NameTaken is the 409 for a name another row already uses (ignoring case and spaces).
func (l List) NameTaken(otherID int64) error {
	return domain.Conflict(l.Label+" already exists",
		domain.FieldError{Field: "name", Message: "is already used by #" + strconv.FormatInt(otherID, 10)})
}

// FunctionError turns the DBA's message (his validate functions stop at the first problem;
// his insert procedures RAISE the same wording) into a field error on name. A duplicate is a
// 409, anything else a 422.
func (l List) FunctionError(message string) error {
	message = strings.TrimSpace(message)
	fe := domain.FieldError{Field: "name", Message: message}
	if strings.Contains(message, "already exists") {
		return domain.Conflict(l.Label+" already exists", fe)
	}
	return domain.Validation(fe)
}

// NormalizeName trims and collapses runs of spaces.
func NormalizeName(s string) string { return strings.Join(strings.Fields(s), " ") }

// ---- Units of measurement

// UOM is one row of pms.mst_stk_uom. NumberDecimal is how many decimals a quantity in this
// unit may have (0 when decimals are off).
type UOM struct {
	ID            int64      `json:"id"`
	UOM           string     `json:"uom"`
	Standard      string     `json:"uom_as_standard"`
	EnableDecimal bool       `json:"enable_decimal"`
	NumberDecimal int        `json:"number_decimal"`
	CreatedAt     *time.Time `json:"rec_created_at,omitempty"`
	UpdatedAt     *time.Time `json:"last_update_at,omitempty"`
}

// UOMInput is the body of POST /stock/uoms and /validate.
type UOMInput struct {
	UOM           string `json:"uom" validate:"required,max=5"`
	Standard      string `json:"uom_as_standard" validate:"required,max=50"`
	EnableDecimal bool   `json:"enable_decimal"`
	NumberDecimal *int   `json:"number_decimal"`
}

// ErrUOMNotFound is the 404 for an unknown UOM id.
var ErrUOMNotFound = domain.NotFound("Unit of measurement not found")

// UOMFunctionError maps the DBA's UOM messages to fields. A duplicate is a 409.
func UOMFunctionError(message string) error {
	message = strings.TrimSpace(message)
	field := ""
	switch {
	case strings.HasPrefix(message, "Standard UOM"):
		field = "uom_as_standard"
	case strings.HasPrefix(message, "UOM"):
		field = "uom"
	case strings.HasPrefix(message, "Number decimal"):
		field = "number_decimal"
	}
	fe := domain.FieldError{Field: field, Message: message}
	if strings.Contains(message, "already exists") {
		return domain.Conflict("Unit of measurement already exists", fe)
	}
	return domain.Validation(fe)
}

// ---- Items

// Item types, spelt as the DBA's CHECK requires.
const (
	TypeGoods   = "Goods"
	TypeService = "Service"
)

// Item is one row of pms.mst_stk_items, with the names of the rows it points at.
type Item struct {
	ID               int64      `json:"id"`
	Desc             string     `json:"item_desc"`
	Code             string     `json:"item_code"`
	GroupID          *int64     `json:"grp_cd"`
	GroupName        string     `json:"item_group,omitempty"`
	CategoryID       *int64     `json:"cat_cd"`
	CategoryName     string     `json:"category,omitempty"`
	ManufacturerID   *int64     `json:"manf_cd"`
	ManufacturerName string     `json:"manufacturer,omitempty"`
	UOMID            *int64     `json:"uom_cd"`
	UOMName          string     `json:"uom,omitempty"`
	Type             string     `json:"goods_service_item"`
	Consumable       bool       `json:"item_consumable"`
	Spare            bool       `json:"item_is_spare"`
	GSTApplicable    bool       `json:"gst_applicable"`
	HSNCode          string     `json:"gst_hsn_code"`
	GST              *float64   `json:"gst_percentage"`
	SGST             *float64   `json:"sgst_percentage"`
	CGST             *float64   `json:"cgst_percentage"`
	TrackSerial      bool       `json:"track_serial"`
	CreatedAt        *time.Time `json:"rec_created_at,omitempty"`
	UpdatedAt        *time.Time `json:"last_update_at,omitempty"`
}

// ItemInput is the body of POST /stock/items and /validate. GSTApplicable is a pointer so a
// missing value can default to true, as the table does.
type ItemInput struct {
	Desc           string   `json:"item_desc" validate:"required,max=250"`
	Code           string   `json:"item_code" validate:"max=20"`
	GroupID        *int64   `json:"grp_cd"`
	CategoryID     *int64   `json:"cat_cd"`
	ManufacturerID *int64   `json:"manf_cd"`
	UOMID          *int64   `json:"uom_cd"`
	Type           string   `json:"goods_service_item" validate:"required"`
	Consumable     bool     `json:"item_consumable"`
	Spare          bool     `json:"item_is_spare"`
	GSTApplicable  *bool    `json:"gst_applicable"`
	HSNCode        string   `json:"gst_hsn_code"`
	GST            *float64 `json:"gst_percentage"`
	SGST           *float64 `json:"sgst_percentage"`
	CGST           *float64 `json:"cgst_percentage"`
	TrackSerial    bool     `json:"track_serial"`
}

// ItemLink is one of an item's four links, in a fixed order.
type ItemLink struct {
	Field string // the JSON field: grp_cd, cat_cd, manf_cd or uom_cd
	Label string // "Item group", for messages
	Table string // the table the id must exist in
	IDCol string
	ID    *int64
}

// Links lists the item's links: group, category, manufacturer, UOM.
func (it *Item) Links() []ItemLink {
	return ItemLinks(it.GroupID, it.CategoryID, it.ManufacturerID, it.UOMID)
}

// ItemLinks lists an item's links: group, category, manufacturer, UOM.
func ItemLinks(group, category, manufacturer, uom *int64) []ItemLink {
	return []ItemLink{
		{"grp_cd", Group.Label, Group.Table, Group.IDCol, group},
		{"cat_cd", Category.Label, Category.Table, Category.IDCol, category},
		{"manf_cd", Manufacturer.Label, Manufacturer.Table, Manufacturer.IDCol, manufacturer},
		{"uom_cd", "Unit of measurement", "pms.mst_stk_uom", "uom_cd", uom},
	}
}

// ErrItemNotFound is the 404 for an unknown item id.
var ErrItemNotFound = domain.NotFound("Item not found")

// ItemFunctionError maps the DBA's item messages to fields. His checks never report a
// duplicate, so everything is a 422.
func ItemFunctionError(message string) error {
	message = strings.TrimSpace(message)
	field := ""
	switch {
	case strings.HasPrefix(message, "Item description"):
		field = "item_desc"
	case strings.HasPrefix(message, "Item type"), strings.HasPrefix(message, "Invalid item type"):
		field = "goods_service_item"
	case strings.HasPrefix(message, "Stock group"):
		field = "grp_cd"
	case strings.HasPrefix(message, "Stock category"):
		field = "cat_cd"
	case strings.HasPrefix(message, "Manufacturer"):
		field = "manf_cd"
	case strings.HasPrefix(message, "Unit of measurement"):
		field = "uom_cd"
	case strings.HasPrefix(message, "Consumable"):
		field = "goods_service_item"
	case strings.HasPrefix(message, "GST"):
		field = "gst_percentage"
	}
	if strings.Contains(message, "record not found") {
		return ErrItemNotFound
	}
	return domain.Validation(domain.FieldError{Field: field, Message: message})
}
