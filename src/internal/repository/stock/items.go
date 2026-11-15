package repository_stock

import (
	"context"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
	domain_stock "github.com/utkarshk014/procurement-system-be/src/internal/domain/stock"
	"github.com/utkarshk014/procurement-system-be/src/internal/repository/pgutil"
)

const itemsTable = "pms.mst_stk_items"

type itemRepository struct {
	pools pgutil.TenantPools
}

// NewPostgresItemRepository uses pms.mst_stk_items. Inserting an item also makes its stock row
// in pms.mst_stk_items_transactions (the DBA's trigger).
func NewPostgresItemRepository(pools pgutil.TenantPools) domain_stock.ItemRepository {
	return &itemRepository{pools: pools}
}

// selectItem joins the linked rows for their names. Percentages are NUMERIC(5,2): float64 holds
// every such value exactly enough to print it back unchanged.
const selectItem = `
	SELECT i.item_cd, COALESCE(i.item_desc, ''), COALESCE(i.item_code, ''),
	       i.grp_cd, COALESCE(g.item_group, ''), i.cat_cd, COALESCE(c.category, ''),
	       i.manf_cd, COALESCE(m.manufacturer, ''), i.uom_cd, COALESCE(u.uom, ''),
	       COALESCE(i.goods_service_item, ''), COALESCE(i.item_consumable, FALSE), COALESCE(i.item_is_spare, FALSE),
	       COALESCE(i.gst_applicable, FALSE), COALESCE(i.gst_hsn_code, ''),
	       i.gst_percentage::float8, i.sgst_percentage::float8, i.cgst_percentage::float8,
	       COALESCE(i.track_serial, FALSE), i.rec_created_at, i.last_update_at
	FROM pms.mst_stk_items i
	LEFT JOIN pms.mst_stk_group g ON g.grp_cd = i.grp_cd
	LEFT JOIN pms.mst_stk_category c ON c.cat_cd = i.cat_cd
	LEFT JOIN pms.mst_stk_manufacturer m ON m.manf_cd = i.manf_cd
	LEFT JOIN pms.mst_stk_uom u ON u.uom_cd = i.uom_cd`

func scanItem(row pgx.Row, it *domain_stock.Item) error {
	return row.Scan(&it.ID, &it.Desc, &it.Code,
		&it.GroupID, &it.GroupName, &it.CategoryID, &it.CategoryName,
		&it.ManufacturerID, &it.ManufacturerName, &it.UOMID, &it.UOMName,
		&it.Type, &it.Consumable, &it.Spare, &it.GSTApplicable, &it.HSNCode,
		&it.GST, &it.SGST, &it.CGST, &it.TrackSerial, &it.CreatedAt, &it.UpdatedAt)
}

func (r *itemRepository) List(ctx context.Context) ([]domain_stock.Item, error) {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, selectItem+` ORDER BY LOWER(i.item_desc), i.item_cd`)
	if err != nil {
		return nil, domain.Internal("Could not list items", err)
	}
	defer rows.Close()
	out := []domain_stock.Item{}
	for rows.Next() {
		var it domain_stock.Item
		if err := scanItem(rows, &it); err != nil {
			return nil, domain.Internal("Could not read item", err)
		}
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		return nil, domain.Internal("Could not list items", err)
	}
	return out, nil
}

func (r *itemRepository) Get(ctx context.Context, id int64) (*domain_stock.Item, error) {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return nil, err
	}
	it := &domain_stock.Item{}
	if err := scanItem(pool.QueryRow(ctx, selectItem+` WHERE i.item_cd = $1`, id), it); err != nil {
		if pgutil.IsNoRows(err) {
			return nil, domain_stock.ErrItemNotFound
		}
		return nil, domain.Internal("Could not load item", err)
	}
	return it, nil
}

func (r *itemRepository) Problems(ctx context.Context, it *domain_stock.Item) ([]domain.FieldError, []domain.FieldError, error) {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return nil, nil, err
	}
	return r.problems(ctx, pool, it)
}

func (r *itemRepository) problems(ctx context.Context, q pgutil.Querier, it *domain_stock.Item) (missing, taken []domain.FieldError, err error) {
	for _, l := range it.Links() {
		if l.ID == nil {
			continue
		}
		var ok bool
		if err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM `+l.Table+` WHERE `+l.IDCol+` = $1)`, *l.ID).Scan(&ok); err != nil {
			return nil, nil, domain.Internal("Could not check "+l.Label, err)
		}
		if !ok {
			missing = append(missing, domain.FieldError{Field: l.Field, Message: "does not exist"})
		}
	}

	// The DBA's procedure accepts duplicates; two items with one description or code would make
	// every later purchase ambiguous.
	var id int64
	err = q.QueryRow(ctx, `SELECT item_cd FROM pms.mst_stk_items WHERE `+sameName("item_desc")+` AND item_cd <> $2 ORDER BY item_cd LIMIT 1`, it.Desc, it.ID).Scan(&id)
	switch {
	case err == nil:
		taken = append(taken, domain.FieldError{Field: "item_desc", Message: "is already used by item #" + strconv.FormatInt(id, 10)})
	case !pgutil.IsNoRows(err):
		return nil, nil, domain.Internal("Could not check item description", err)
	}
	if it.Code != "" {
		err = q.QueryRow(ctx, `SELECT item_cd FROM pms.mst_stk_items WHERE UPPER(TRIM(item_code)) = $1 AND item_cd <> $2 ORDER BY item_cd LIMIT 1`, it.Code, it.ID).Scan(&id)
		switch {
		case err == nil:
			taken = append(taken, domain.FieldError{Field: "item_code", Message: "is already used by item #" + strconv.FormatInt(id, 10)})
		case !pgutil.IsNoRows(err):
			return nil, nil, domain.Internal("Could not check item code", err)
		}
	}
	return missing, taken, nil
}

// args are the item's columns in the order both DBA routines take them (after item_cd for the
// validate function).
func args(it *domain_stock.Item) []any {
	return []any{
		it.Desc, pgutil.NullString(it.Code), it.GroupID, it.CategoryID, it.ManufacturerID, it.UOMID,
		it.Type, it.Consumable, it.Spare, it.GSTApplicable, pgutil.NullString(it.HSNCode),
		it.GST, it.SGST, it.CGST, it.TrackSerial,
	}
}

const itemTypes = `$1::varchar, $2::varchar, $3::integer, $4::integer, $5::integer, $6::integer,
	$7::varchar, $8::boolean, $9::boolean, $10::boolean, $11::varchar,
	$12::numeric, $13::numeric, $14::numeric, $15::boolean`

func (r *itemRepository) CheckWithFunction(ctx context.Context, it *domain_stock.Item) (*domain_stock.FunctionResult, error) {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return nil, err
	}
	return r.checkWithFunction(ctx, pool, it)
}

func (r *itemRepository) checkWithFunction(ctx context.Context, q pgutil.Querier, it *domain_stock.Item) (*domain_stock.FunctionResult, error) {
	res := &domain_stock.FunctionResult{}
	if err := q.QueryRow(ctx, `SELECT is_valid, message FROM pms.fn_validate_mst_stk_items($16::integer, `+itemTypes+`)`, append(args(it), it.ID)...).
		Scan(&res.Valid, &res.Message); err != nil {
		return nil, domain.Internal("Could not run item validation", err)
	}
	return res, nil
}

// checkAll runs, inside the lock, everything a save needs: links that exist, a free description
// and code, then the DBA's check.
func (r *itemRepository) checkAll(ctx context.Context, tx pgx.Tx, it *domain_stock.Item) error {
	missing, taken, err := r.problems(ctx, tx, it)
	if err != nil {
		return err
	}
	if len(missing) > 0 {
		return domain.Validation(missing...)
	}
	if len(taken) > 0 {
		return domain.Conflict("Item already exists", taken...)
	}
	res, err := r.checkWithFunction(ctx, tx, it)
	if err != nil {
		return err
	}
	if !res.Valid {
		return domain_stock.ItemFunctionError(res.Message)
	}
	return nil
}

// saveError maps a failed CALL of the DBA's insert or update procedure.
func saveError(err error) error {
	if msg, ok := raised(err); ok {
		return domain_stock.ItemFunctionError(msg)
	}
	if isForeignKeyViolation(err) { // a linked row vanished after our check
		return domain.Validation(domain.FieldError{Message: "A linked group, category, manufacturer or unit no longer exists"})
	}
	return domain.Internal("Could not save item", err)
}

func (r *itemRepository) Create(ctx context.Context, it *domain_stock.Item) (int64, error) {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return 0, err
	}
	it.ID = 0
	var id int64
	err = pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if err := lock(ctx, tx, itemsTable); err != nil {
			return err
		}
		if err := r.checkAll(ctx, tx, it); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `CALL pms.insert_mst_stk_items(`+itemTypes+`)`, args(it)...); err != nil {
			return saveError(err)
		}
		id, err = newID(ctx, tx, itemsTable, "item_cd")
		return err
	})
	return id, err
}

func (r *itemRepository) Update(ctx context.Context, it *domain_stock.Item) error {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if err := lock(ctx, tx, itemsTable); err != nil {
			return err
		}
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pms.mst_stk_items WHERE item_cd = $1 FOR UPDATE)`, it.ID).Scan(&exists); err != nil {
			return domain.Internal("Could not load item", err)
		}
		if !exists {
			return domain_stock.ErrItemNotFound
		}
		if err := r.checkAll(ctx, tx, it); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `CALL pms.update_mst_stk_items($16::integer, `+itemTypes+`)`, append(args(it), it.ID)...); err != nil {
			return saveError(err)
		}
		return nil
	})
}

// Delete reads the item's stored description and code under the items lock and calls the DBA's
// delete procedure with them. His procedure checks first and refuses an item with stock or
// purchase information; his trigger removes the item's stock row.
func (r *itemRepository) Delete(ctx context.Context, id int64) error {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if err := lock(ctx, tx, itemsTable); err != nil {
			return err
		}
		var desc, code string
		err := tx.QueryRow(ctx, `SELECT COALESCE(item_desc, ''), COALESCE(item_code, '') FROM pms.mst_stk_items WHERE item_cd = $1 FOR UPDATE`, id).
			Scan(&desc, &code)
		if pgutil.IsNoRows(err) {
			return domain_stock.ErrItemNotFound
		}
		if err != nil {
			return domain.Internal("Could not load item", err)
		}
		if _, err := tx.Exec(ctx, `CALL pms.delete_mst_stk_items($1::integer, $2::varchar, $3::varchar)`, id, desc, code); err != nil {
			return deleteError(err, "Item", domain_stock.ErrItemNotFound)
		}
		return nil
	})
}
