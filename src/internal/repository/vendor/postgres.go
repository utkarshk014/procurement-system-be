// Package repository_vendor reads the DBA's vendor tables and writes them only through his
// procedures, always by named arguments so a change in his parameter order cannot shift values.
// Every write takes one advisory lock on pms.mst_vendor, so our duplicate checks, his checks and
// the write see the same rows, and a vendor cannot be deleted while its financials are saved.
package repository_vendor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
	domain_vendor "github.com/utkarshk014/procurement-system-be/src/internal/domain/vendor"
	"github.com/utkarshk014/procurement-system-be/src/internal/repository/pgutil"
)

const lockKey = "pms.mst_vendor"

type repository struct {
	pools pgutil.TenantPools
}

// NewPostgresVendorRepository uses pms.mst_vendor and its three child tables.
func NewPostgresVendorRepository(pools pgutil.TenantPools) domain_vendor.Repository {
	return &repository{pools: pools}
}

func lock(ctx context.Context, tx pgx.Tx) error {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, lockKey); err != nil {
		return domain.Internal("Could not lock vendors", err)
	}
	return nil
}

// ---- Reading

const selectVendor = `
	SELECT v.vendor_cd, COALESCE(v.name, ''), COALESCE(v.address, ''), COALESCE(v.city, ''), COALESCE(v.state, ''),
	       COALESCE(v.country, ''), COALESCE(v.pin, ''), COALESCE(v.vendor_type, ''), COALESCE(v.mobile, ''),
	       COALESCE(v.email, ''), COALESCE(v.website, ''), to_char(v.reg_date, 'YYYY-MM-DD'),
	       COALESCE(v.pan, ''), COALESCE(v.gstin, ''), COALESCE(v.gstin_state, ''), COALESCE(v.tan, ''),
	       COALESCE(v.refered_by, ''), COALESCE(v.correspond_email, ''), COALESCE(v.correspond_mobile, ''),
	       COALESCE(v.supply_type, ''), COALESCE(v.is_contruction_comp, FALSE), COALESCE(v.is_manufacturer, FALSE),
	       COALESCE(v.company_since, ''),
	       COALESCE(v.contact_1_person, ''), COALESCE(v.contact_1_designation, ''), COALESCE(v.contact_1_mobile, ''), COALESCE(v.contact_1_email, ''),
	       COALESCE(v.contact_2_person, ''), COALESCE(v.contact_2_designation, ''), COALESCE(v.contact_2_mobile, ''), COALESCE(v.contact_2_email, ''),
	       COALESCE(v.contact_3_person, ''), COALESCE(v.contact_3_designation, ''), COALESCE(v.contact_3_mobile, ''), COALESCE(v.contact_3_email, ''),
	       COALESCE((SELECT json_agg(json_build_object('grp_cd', g.grp_cd, 'item_group', COALESCE(g.item_group, '')) ORDER BY LOWER(g.item_group), g.grp_cd)
	                 FROM pms.mst_vendor_item_group vg JOIN pms.mst_stk_group g ON g.grp_cd = vg.grp_cd
	                 WHERE vg.vendor_cd = v.vendor_cd), '[]')
	FROM pms.mst_vendor v`

func scanVendor(row pgx.Row, v *domain_vendor.Vendor) error {
	var groups []byte
	c := &v.Contacts
	if err := row.Scan(&v.ID, &v.Name, &v.Address, &v.City, &v.State, &v.Country, &v.Pin, &v.VendorType, &v.Mobile,
		&v.Email, &v.Website, &v.RegDate, &v.PAN, &v.GSTIN, &v.GSTINState, &v.TAN, &v.ReferredBy,
		&v.CorrespondEmail, &v.CorrespondMobile, &v.SupplyType, &v.IsConstruction, &v.IsManufacturer, &v.CompanySince,
		&c[0].Person, &c[0].Designation, &c[0].Mobile, &c[0].Email,
		&c[1].Person, &c[1].Designation, &c[1].Mobile, &c[1].Email,
		&c[2].Person, &c[2].Designation, &c[2].Mobile, &c[2].Email,
		&groups); err != nil {
		return err
	}
	v.Groups = []domain_vendor.Group{}
	return json.Unmarshal(groups, &v.Groups)
}

func (r *repository) List(ctx context.Context) ([]domain_vendor.Vendor, error) {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, selectVendor+` ORDER BY LOWER(v.name), v.vendor_cd`)
	if err != nil {
		return nil, domain.Internal("Could not list vendors", err)
	}
	defer rows.Close()
	out := []domain_vendor.Vendor{}
	for rows.Next() {
		var v domain_vendor.Vendor
		if err := scanVendor(rows, &v); err != nil {
			return nil, domain.Internal("Could not read vendor", err)
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, domain.Internal("Could not list vendors", err)
	}
	return out, nil
}

func (r *repository) Get(ctx context.Context, id int64) (*domain_vendor.VendorDetail, error) {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return nil, err
	}
	d := &domain_vendor.VendorDetail{}
	if err := scanVendor(pool.QueryRow(ctx, selectVendor+` WHERE v.vendor_cd = $1`, id), &d.Vendor); err != nil {
		if pgutil.IsNoRows(err) {
			return nil, domain_vendor.ErrVendorNotFound
		}
		return nil, domain.Internal("Could not load vendor", err)
	}
	if d.Financials, err = financials(ctx, pool, id, ""); err != nil {
		return nil, err
	}
	if d.Certificates, err = certificates(ctx, pool, id, false); err != nil {
		return nil, err
	}
	return d, nil
}

func (r *repository) Exists(ctx context.Context, id int64) (bool, error) {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return false, err
	}
	var ok bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pms.mst_vendor WHERE vendor_cd = $1)`, id).Scan(&ok); err != nil {
		return false, domain.Internal("Could not load vendor", err)
	}
	return ok, nil
}

// ---- Checks

// sameName compares names ignoring case and runs of spaces, as we store them.
const sameName = `regexp_replace(LOWER(BTRIM(name)), '\s+', ' ', 'g') = LOWER($1)`

func (r *repository) Problems(ctx context.Context, v *domain_vendor.Vendor) ([]domain.FieldError, []domain.FieldError, error) {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return nil, nil, err
	}
	return problems(ctx, pool, v)
}

func problems(ctx context.Context, q pgutil.Querier, v *domain_vendor.Vendor) (missing, taken []domain.FieldError, err error) {
	if ids := v.GroupIDs(); len(ids) > 0 {
		var absent []int64
		if err := q.QueryRow(ctx, `SELECT COALESCE(array_agg(id ORDER BY id), '{}') FROM unnest($1::int[]) id
			WHERE NOT EXISTS (SELECT 1 FROM pms.mst_stk_group g WHERE g.grp_cd = id)`, ids).Scan(&absent); err != nil {
			return nil, nil, domain.Internal("Could not check item groups", err)
		}
		if len(absent) > 0 {
			names := make([]string, len(absent))
			for i, id := range absent {
				names[i] = "#" + strconv.FormatInt(id, 10)
			}
			missing = append(missing, domain.FieldError{Field: "grp_cds", Message: "item group " + strings.Join(names, ", ") + " does not exist"})
		}
	}
	check := func(field, where, value string) error {
		if value == "" {
			return nil
		}
		var other string
		err := q.QueryRow(ctx, `SELECT COALESCE(name, '#' || vendor_cd) FROM pms.mst_vendor WHERE `+where+` AND vendor_cd <> $2 ORDER BY vendor_cd LIMIT 1`, value, v.ID).Scan(&other)
		switch {
		case err == nil:
			taken = append(taken, domain.FieldError{Field: field, Message: "is already used by " + other})
		case !pgutil.IsNoRows(err):
			return domain.Internal("Could not check vendor "+field, err)
		}
		return nil
	}
	if err := check("name", sameName, v.Name); err != nil {
		return nil, nil, err
	}
	if err := check("pan", `UPPER(BTRIM(pan)) = $1`, v.PAN); err != nil {
		return nil, nil, err
	}
	if err := check("gstin", `UPPER(BTRIM(gstin)) = $1`, v.GSTIN); err != nil {
		return nil, nil, err
	}
	return missing, taken, nil
}

// columns are the vendor's columns (after the id) with their values; empty text is sent as NULL.
func columns(v *domain_vendor.Vendor) []column {
	s := pgutil.NullString
	c := v.Contacts
	var reg any
	if v.RegDate != nil {
		reg = *v.RegDate
	}
	return []column{
		{"name", "varchar", v.Name}, {"address", "varchar", s(v.Address)}, {"city", "varchar", s(v.City)},
		{"state", "varchar", s(v.State)}, {"country", "varchar", s(v.Country)}, {"pin", "varchar", s(v.Pin)},
		{"vendor_type", "varchar", s(v.VendorType)}, {"mobile", "varchar", s(v.Mobile)}, {"email", "varchar", s(v.Email)},
		{"website", "varchar", s(v.Website)}, {"reg_date", "date", reg}, {"pan", "varchar", s(v.PAN)},
		{"gstin", "varchar", s(v.GSTIN)}, {"gstin_state", "varchar", s(v.GSTINState)}, {"tan", "varchar", s(v.TAN)},
		{"refered_by", "varchar", s(v.ReferredBy)}, {"correspond_email", "varchar", s(v.CorrespondEmail)},
		{"correspond_mobile", "varchar", s(v.CorrespondMobile)}, {"supply_type", "varchar", s(v.SupplyType)},
		{"is_contruction_comp", "boolean", v.IsConstruction}, {"is_manufacturer", "boolean", v.IsManufacturer},
		{"company_since", "varchar", s(v.CompanySince)},
		{"contact_1_person", "varchar", s(c[0].Person)}, {"contact_1_designation", "varchar", s(c[0].Designation)},
		{"contact_1_mobile", "varchar", s(c[0].Mobile)}, {"contact_1_email", "varchar", s(c[0].Email)},
		{"contact_2_person", "varchar", s(c[1].Person)}, {"contact_2_designation", "varchar", s(c[1].Designation)},
		{"contact_2_mobile", "varchar", s(c[1].Mobile)}, {"contact_2_email", "varchar", s(c[1].Email)},
		{"contact_3_person", "varchar", s(c[2].Person)}, {"contact_3_designation", "varchar", s(c[2].Designation)},
		{"contact_3_mobile", "varchar", s(c[2].Mobile)}, {"contact_3_email", "varchar", s(c[2].Email)},
	}
}

type column struct {
	name, typ string
	value     any
}

// namedCall builds `CALL proc(prefix<col> => $n::type, …)` and its arguments.
func namedCall(proc, prefix string, cols []column) (string, []any) {
	parts := make([]string, len(cols))
	args := make([]any, len(cols))
	for i, c := range cols {
		parts[i] = fmt.Sprintf("%s%s => $%d::%s", prefix, c.name, i+1, c.typ)
		args[i] = c.value
	}
	return "CALL " + proc + "(" + strings.Join(parts, ", ") + ")", args
}

func (r *repository) CheckWithFunction(ctx context.Context, v *domain_vendor.Vendor) error {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return err
	}
	return checkWithFunction(ctx, pool, v)
}

// checkWithFunction runs the DBA's fn_validate_mst_vendor, which takes a whole pms.mst_vendor
// row and RAISEs its first problem. The row is built from JSON by column name, so the order of
// his columns does not matter.
func checkWithFunction(ctx context.Context, q pgutil.Querier, v *domain_vendor.Vendor) error {
	row := map[string]any{}
	for _, c := range columns(v) {
		row[c.name] = c.value
	}
	if v.ID > 0 {
		row["vendor_cd"] = v.ID
	}
	body, err := json.Marshal(row)
	if err != nil {
		return domain.Internal("Could not build the vendor check", err)
	}
	if _, err := q.Exec(ctx, `SELECT pms.fn_validate_mst_vendor(jsonb_populate_record(NULL::pms.mst_vendor, $1::jsonb))`, string(body)); err != nil {
		return saveError(err)
	}
	return nil
}

// ---- Errors

var tenDigits = regexp.MustCompile(`^(correspond_mobile|contact_([123])_mobile) must contain exactly 10 digits`)
var emailIn = regexp.MustCompile(`^Invalid email address in (\w+)`)

// FunctionError maps the DBA's vendor messages to fields. His create and update word them
// differently; a duplicate is a 409, the rest a 422.
func FunctionError(message string) error {
	message = strings.TrimSpace(strings.ReplaceAll(message, `""`, `"`))
	lower := strings.ToLower(message)
	if strings.HasPrefix(message, "Vendor code") && strings.Contains(lower, "does not exist") {
		return domain_vendor.ErrVendorNotFound
	}
	field := ""
	switch {
	case strings.HasPrefix(lower, "vendor name"):
		field = "name"
	case strings.HasPrefix(message, "PAN"), strings.HasPrefix(message, "Invalid PAN"):
		field = "pan"
	case strings.HasPrefix(message, "GSTIN"), strings.HasPrefix(message, "Invalid GSTIN"):
		field = "gstin"
	case strings.HasPrefix(message, "TAN"), strings.HasPrefix(message, "Invalid TAN"):
		field = "tan"
	case strings.HasPrefix(message, "Invalid vendor mobile"):
		field = "mobile"
	case strings.HasPrefix(lower, "company since"), strings.HasPrefix(lower, "invalid company since"):
		field = "company_since"
	case strings.HasPrefix(message, "Item group"):
		field = "grp_cds"
	}
	if m := tenDigits.FindStringSubmatch(message); m != nil {
		field = m[1]
		if m[2] != "" {
			field = "contacts." + strconv.Itoa(int(m[2][0]-'1')) + ".mobile"
		}
	}
	if m := emailIn.FindStringSubmatch(message); m != nil {
		field = m[1]
		if strings.HasPrefix(field, "contact_") && len(field) > 8 {
			field = "contacts." + strconv.Itoa(int(field[8]-'1')) + ".email"
		}
	}
	fe := domain.FieldError{Field: field, Message: message}
	if strings.Contains(lower, "already exists") || strings.Contains(lower, "already registered") || strings.Contains(lower, "already assigned") {
		return domain.Conflict("Vendor already exists", fe)
	}
	return domain.Validation(fe)
}

// uniqueFields names the field behind each of the DBA's unique indexes.
var uniqueFields = map[string]string{"uq_mst_vendor_name": "name", "uq_mst_vendor_pan": "pan", "uq_mst_vendor_gstin": "gstin"}

// saveError maps a failed call of the DBA's vendor routines.
func saveError(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return domain.Internal("Could not save vendor", err)
	}
	switch pgErr.Code {
	case "P0001":
		return FunctionError(pgErr.Message)
	case "23505":
		if f, ok := uniqueFields[pgErr.ConstraintName]; ok {
			return domain.Conflict("Vendor already exists", domain.FieldError{Field: f, Message: "is already used by another vendor"})
		}
		return domain.Conflict("Vendor already exists")
	case "23503":
		return domain.Validation(domain.FieldError{Field: "grp_cds", Message: "an item group no longer exists"})
	}
	return domain.Internal("Could not save vendor", err)
}

// ---- Writing the vendor

// checkAll runs, inside the lock, everything a save needs: groups that exist, a free name, PAN
// and GSTIN.
func checkAll(ctx context.Context, tx pgx.Tx, v *domain_vendor.Vendor) error {
	missing, taken, err := problems(ctx, tx, v)
	if err != nil {
		return err
	}
	if len(missing) > 0 {
		return domain.Validation(missing...)
	}
	if len(taken) > 0 {
		return domain.Conflict("Vendor already exists", taken...)
	}
	return nil
}

func (r *repository) Create(ctx context.Context, v *domain_vendor.Vendor) (int64, error) {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return 0, err
	}
	v.ID = 0
	var id int64
	err = pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if err := lock(ctx, tx); err != nil {
			return err
		}
		if err := checkAll(ctx, tx, v); err != nil {
			return err
		}
		sql, args := namedCall("pms.insert_mst_vendor", "take_", columns(v))
		if _, err := tx.Exec(ctx, sql, args...); err != nil {
			return saveError(err)
		}
		if err := tx.QueryRow(ctx, `SELECT currval(pg_get_serial_sequence('pms.mst_vendor', 'vendor_cd'))`).Scan(&id); err != nil {
			return domain.Internal("Could not read the new vendor id", err)
		}
		return setGroups(ctx, tx, id, v.GroupIDs())
	})
	return id, err
}

func (r *repository) Update(ctx context.Context, v *domain_vendor.Vendor) error {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if err := lock(ctx, tx); err != nil {
			return err
		}
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pms.mst_vendor WHERE vendor_cd = $1 FOR UPDATE)`, v.ID).Scan(&exists); err != nil {
			return domain.Internal("Could not load vendor", err)
		}
		if !exists {
			return domain_vendor.ErrVendorNotFound
		}
		if err := checkAll(ctx, tx, v); err != nil {
			return err
		}
		cols := append([]column{{"vendor_cd", "integer", v.ID}}, columns(v)...)
		sql, args := namedCall("pms.update_mst_vendor", "p_", cols)
		if _, err := tx.Exec(ctx, sql, args...); err != nil {
			return saveError(err)
		}
		return setGroups(ctx, tx, v.ID, v.GroupIDs())
	})
}

// setGroups makes the vendor's groups exactly ids, through the DBA's insert and delete.
func setGroups(ctx context.Context, tx pgx.Tx, vendorID int64, ids []int64) error {
	var current []int64
	if err := tx.QueryRow(ctx, `SELECT COALESCE(array_agg(grp_cd), '{}') FROM pms.mst_vendor_item_group WHERE vendor_cd = $1`, vendorID).Scan(&current); err != nil {
		return domain.Internal("Could not load the vendor's item groups", err)
	}
	want := map[int64]bool{}
	for _, id := range ids {
		want[id] = true
	}
	have := map[int64]bool{}
	for _, id := range current {
		have[id] = true
		if !want[id] {
			if _, err := tx.Exec(ctx, `CALL pms.delete_mst_vendor_item_group(take_vendor_cd => $1::integer, take_grp_cd => $2::integer)`, vendorID, id); err != nil {
				return saveError(err)
			}
		}
	}
	for _, id := range ids {
		if !have[id] {
			if _, err := tx.Exec(ctx, `CALL pms.insert_mst_vendor_item_group(take_vendor_cd => $1::integer, take_grp_cd => $2::integer)`, vendorID, id); err != nil {
				return saveError(err)
			}
		}
	}
	return nil
}

func (r *repository) Delete(ctx context.Context, id int64) ([]string, error) {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return nil, err
	}
	var files []string
	err = pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if err := lock(ctx, tx); err != nil {
			return err
		}
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pms.mst_vendor WHERE vendor_cd = $1 FOR UPDATE)`, id).Scan(&exists); err != nil {
			return domain.Internal("Could not load vendor", err)
		}
		if !exists {
			return domain_vendor.ErrVendorNotFound
		}
		fins, err := financials(ctx, tx, id, "")
		if err != nil {
			return err
		}
		for _, f := range fins {
			if f.ITRFile != "" {
				files = append(files, f.ITRFile)
			}
		}
		certs, err := certificates(ctx, tx, id, true)
		if err != nil {
			return err
		}
		files = append(files, certs.Files()...)
		if _, err := tx.Exec(ctx, `CALL pms.delete_mst_vendor(p_vendor_cd => $1::integer)`, id); err != nil {
			var pgErr *pgconn.PgError
			switch {
			case errors.As(err, &pgErr) && pgErr.Code == "P0001":
				if strings.Contains(strings.ToLower(pgErr.Message), "does not exist") {
					return domain_vendor.ErrVendorNotFound
				}
				return domain.Conflict(strings.TrimSpace(pgErr.Message))
			case errors.As(err, &pgErr) && pgErr.Code == "23503":
				return domain.Conflict("Cannot delete! This vendor is still in use")
			}
			return domain.Internal("Could not delete vendor", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}
