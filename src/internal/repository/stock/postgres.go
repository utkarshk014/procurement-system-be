// Package repository_stock reads the DBA's stock tables and writes them only through his
// procedures. His insert procedures return nothing, so the new row's id is read with currval on
// the table's identity sequence inside the same transaction: it is the value this session just
// used, whatever other sessions insert at the same moment.
package repository_stock

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
	domain_stock "github.com/utkarshk014/procurement-system-be/src/internal/domain/stock"
	"github.com/utkarshk014/procurement-system-be/src/internal/repository/pgutil"
)

// lock serialises every write to one table, so the duplicate checks and the write see the same
// rows. The DBA's tables have no unique constraints, so this is the only guard against two saves
// of the same name at the same moment.
func lock(ctx context.Context, tx pgx.Tx, table string) error {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, table); err != nil {
		return domain.Internal("Could not lock "+table, err)
	}
	return nil
}

// newID returns the id this transaction's insert into table just generated.
func newID(ctx context.Context, tx pgx.Tx, table, idCol string) (int64, error) {
	var id int64
	if err := tx.QueryRow(ctx, `SELECT currval(pg_get_serial_sequence($1, $2))`, table, idCol).Scan(&id); err != nil {
		return 0, domain.Internal("Could not read the new id", err)
	}
	return id, nil
}

// raised returns the message of a RAISE EXCEPTION from the DBA's procedure (SQLSTATE P0001).
func raised(err error) (string, bool) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "P0001" {
		return pgErr.Message, true
	}
	return "", false
}

func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}

// deleteError maps a failed CALL of the DBA's delete procedure. His procedures run their own
// check first and RAISE its message: "… does not exist" is notFound, any other refusal ("Cannot
// delete. Stock items are linked …", "… changed before deletion") is a 409 carrying his words.
func deleteError(err error, what string, notFound error) error {
	if msg, ok := raised(err); ok {
		if strings.Contains(strings.ToLower(msg), "does not exist") {
			return notFound
		}
		return domain.Conflict(strings.TrimSpace(msg))
	}
	if isForeignKeyViolation(err) {
		return domain.Conflict("Cannot delete! " + what + " is still in use")
	}
	return domain.Internal("Could not delete "+strings.ToLower(what), err)
}

// sameName compares names ignoring case and runs of spaces, as we store them.
func sameName(col string) string {
	return `regexp_replace(LOWER(TRIM(` + col + `)), '\s+', ' ', 'g') = regexp_replace(LOWER(TRIM($1)), '\s+', ' ', 'g')`
}

// ---- One-field lists

type listRepository struct {
	pools pgutil.TenantPools
	list  domain_stock.List
}

// NewPostgresListRepository uses one list's table; its names come from list, constants only.
func NewPostgresListRepository(pools pgutil.TenantPools, list domain_stock.List) domain_stock.ListRepository {
	return &listRepository{pools: pools, list: list}
}

func (r *listRepository) selectSQL() string {
	return `SELECT ` + r.list.IDCol + `, COALESCE(` + r.list.NameCol + `, ''), rec_created_at, last_update_at FROM ` + r.list.Table
}

func scanEntry(row pgx.Row, e *domain_stock.Entry) error {
	return row.Scan(&e.ID, &e.Name, &e.CreatedAt, &e.UpdatedAt)
}

func (r *listRepository) List(ctx context.Context) ([]domain_stock.Entry, error) {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, r.selectSQL()+` ORDER BY LOWER(`+r.list.NameCol+`), `+r.list.IDCol)
	if err != nil {
		return nil, domain.Internal("Could not list "+r.list.Plural, err)
	}
	defer rows.Close()
	out := []domain_stock.Entry{}
	for rows.Next() {
		var e domain_stock.Entry
		if err := scanEntry(rows, &e); err != nil {
			return nil, domain.Internal("Could not read "+r.list.Label, err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, domain.Internal("Could not list "+r.list.Plural, err)
	}
	return out, nil
}

func (r *listRepository) Get(ctx context.Context, id int64) (*domain_stock.Entry, error) {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return nil, err
	}
	return r.get(ctx, pool, id, "")
}

// get reads one row; suffix may add FOR UPDATE inside a transaction.
func (r *listRepository) get(ctx context.Context, q pgutil.Querier, id int64, suffix string) (*domain_stock.Entry, error) {
	e := &domain_stock.Entry{}
	if err := scanEntry(q.QueryRow(ctx, r.selectSQL()+` WHERE `+r.list.IDCol+` = $1`+suffix, id), e); err != nil {
		if pgutil.IsNoRows(err) {
			return nil, r.list.NotFound()
		}
		return nil, domain.Internal("Could not load "+r.list.Label, err)
	}
	return e, nil
}

func (r *listRepository) NameTaken(ctx context.Context, name string, exceptID int64) (int64, bool, error) {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return 0, false, err
	}
	return r.nameTaken(ctx, pool, name, exceptID)
}

func (r *listRepository) nameTaken(ctx context.Context, q pgutil.Querier, name string, exceptID int64) (int64, bool, error) {
	var id int64
	err := q.QueryRow(ctx, `SELECT `+r.list.IDCol+` FROM `+r.list.Table+` WHERE `+sameName(r.list.NameCol)+
		` AND `+r.list.IDCol+` <> $2 ORDER BY `+r.list.IDCol+` LIMIT 1`, name, exceptID).Scan(&id)
	switch {
	case err == nil:
		return id, true, nil
	case pgutil.IsNoRows(err):
		return 0, false, nil
	default:
		return 0, false, domain.Internal("Could not check "+r.list.Label+" name", err)
	}
}

func (r *listRepository) CheckWithFunction(ctx context.Context, id int64, name string) (*domain_stock.FunctionResult, error) {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return nil, err
	}
	return r.checkWithFunction(ctx, pool, id, name)
}

func (r *listRepository) checkWithFunction(ctx context.Context, q pgutil.Querier, id int64, name string) (*domain_stock.FunctionResult, error) {
	res := &domain_stock.FunctionResult{}
	if err := q.QueryRow(ctx, `SELECT is_valid, message FROM `+r.list.Validate+`($1::integer, $2::varchar)`, id, name).
		Scan(&res.Valid, &res.Message); err != nil {
		return nil, domain.Internal("Could not run "+r.list.Label+" validation", err)
	}
	return res, nil
}

// checkAll runs, inside the lock, everything a save needs: a free name, then the DBA's check.
func (r *listRepository) checkAll(ctx context.Context, tx pgx.Tx, id int64, name string) error {
	if other, taken, err := r.nameTaken(ctx, tx, name, id); err != nil {
		return err
	} else if taken {
		return r.list.NameTaken(other)
	}
	res, err := r.checkWithFunction(ctx, tx, id, name)
	if err != nil {
		return err
	}
	if !res.Valid {
		return r.list.FunctionError(res.Message)
	}
	return nil
}

func (r *listRepository) Create(ctx context.Context, name string) (int64, error) {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return 0, err
	}
	var id int64
	err = pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if err := lock(ctx, tx, r.list.Table); err != nil {
			return err
		}
		if err := r.checkAll(ctx, tx, 0, name); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `CALL `+r.list.Insert+`($1::varchar)`, name); err != nil {
			if msg, ok := raised(err); ok {
				return r.list.FunctionError(msg)
			}
			return domain.Internal("Could not save "+r.list.Label, err)
		}
		id, err = newID(ctx, tx, r.list.Table, r.list.IDCol)
		return err
	})
	return id, err
}

func (r *listRepository) Update(ctx context.Context, id int64, name string) error {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if err := lock(ctx, tx, r.list.Table); err != nil {
			return err
		}
		if _, err := r.get(ctx, tx, id, " FOR UPDATE"); err != nil {
			return err
		}
		if err := r.checkAll(ctx, tx, id, name); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `CALL `+r.list.Update+`($1::integer, $2::varchar)`, id, name); err != nil {
			if msg, ok := raised(err); ok {
				if strings.Contains(msg, "does not exist") {
					return r.list.NotFound()
				}
				return r.list.FunctionError(msg)
			}
			return domain.Internal("Could not save "+r.list.Label, err)
		}
		return nil
	})
}

// Delete takes the table lock (items take their own, so an item saved at this moment still
// meets the DBA's check or the foreign key), reads the stored name, then calls his procedure.
func (r *listRepository) Delete(ctx context.Context, id int64) error {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if err := lock(ctx, tx, r.list.Table); err != nil {
			return err
		}
		e, err := r.get(ctx, tx, id, " FOR UPDATE")
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `CALL `+r.list.Delete+`($1::integer, $2::varchar)`, id, e.Name); err != nil {
			return deleteError(err, r.list.Label, r.list.NotFound())
		}
		return nil
	})
}

// ---- Units of measurement

const uomTable = "pms.mst_stk_uom"

type uomRepository struct {
	pools pgutil.TenantPools
}

// NewPostgresUOMRepository uses pms.mst_stk_uom.
func NewPostgresUOMRepository(pools pgutil.TenantPools) domain_stock.UOMRepository {
	return &uomRepository{pools: pools}
}

// number_decimal is VARCHAR(1); anything but a digit reads as 0.
const selectUOM = `
	SELECT uom_cd, COALESCE(uom, ''), COALESCE(uom_as_standard, ''), COALESCE(enable_decimal, FALSE),
	       CASE WHEN number_decimal ~ '^[0-9]$' THEN number_decimal::int ELSE 0 END,
	       rec_created_at, last_update_at
	FROM pms.mst_stk_uom`

func scanUOM(row pgx.Row, u *domain_stock.UOM) error {
	return row.Scan(&u.ID, &u.UOM, &u.Standard, &u.EnableDecimal, &u.NumberDecimal, &u.CreatedAt, &u.UpdatedAt)
}

func (r *uomRepository) List(ctx context.Context) ([]domain_stock.UOM, error) {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, selectUOM+` ORDER BY LOWER(uom), uom_cd`)
	if err != nil {
		return nil, domain.Internal("Could not list units of measurement", err)
	}
	defer rows.Close()
	out := []domain_stock.UOM{}
	for rows.Next() {
		var u domain_stock.UOM
		if err := scanUOM(rows, &u); err != nil {
			return nil, domain.Internal("Could not read unit of measurement", err)
		}
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, domain.Internal("Could not list units of measurement", err)
	}
	return out, nil
}

func (r *uomRepository) Get(ctx context.Context, id int64) (*domain_stock.UOM, error) {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return nil, err
	}
	return r.get(ctx, pool, id, "")
}

func (r *uomRepository) get(ctx context.Context, q pgutil.Querier, id int64, suffix string) (*domain_stock.UOM, error) {
	u := &domain_stock.UOM{}
	if err := scanUOM(q.QueryRow(ctx, selectUOM+` WHERE uom_cd = $1`+suffix, id), u); err != nil {
		if pgutil.IsNoRows(err) {
			return nil, domain_stock.ErrUOMNotFound
		}
		return nil, domain.Internal("Could not load unit of measurement", err)
	}
	return u, nil
}

func (r *uomRepository) Taken(ctx context.Context, u *domain_stock.UOM) ([]domain.FieldError, error) {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return nil, err
	}
	return r.taken(ctx, pool, u)
}

func (r *uomRepository) taken(ctx context.Context, q pgutil.Querier, u *domain_stock.UOM) ([]domain.FieldError, error) {
	var out []domain.FieldError
	for _, c := range []struct{ field, col, value string }{
		{"uom", "uom", u.UOM}, {"uom_as_standard", "uom_as_standard", u.Standard},
	} {
		var other string
		err := q.QueryRow(ctx, `SELECT COALESCE(uom, '') FROM pms.mst_stk_uom WHERE `+sameName(c.col)+
			` AND uom_cd <> $2 ORDER BY uom_cd LIMIT 1`, c.value, u.ID).Scan(&other)
		switch {
		case err == nil:
			out = append(out, domain.FieldError{Field: c.field, Message: "is already used by " + other})
		case !pgutil.IsNoRows(err):
			return nil, domain.Internal("Could not check unit of measurement", err)
		}
	}
	return out, nil
}

func (r *uomRepository) CheckWithFunction(ctx context.Context, u *domain_stock.UOM) (*domain_stock.FunctionResult, error) {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return nil, err
	}
	return r.checkWithFunction(ctx, pool, u)
}

func (r *uomRepository) checkWithFunction(ctx context.Context, q pgutil.Querier, u *domain_stock.UOM) (*domain_stock.FunctionResult, error) {
	res := &domain_stock.FunctionResult{}
	if err := q.QueryRow(ctx, `SELECT is_valid, message FROM pms.fn_validate_mst_stk_uom($1::integer, $2::varchar, $3::varchar, $4::boolean, $5::varchar)`,
		u.ID, u.UOM, u.Standard, u.EnableDecimal, strconv.Itoa(u.NumberDecimal)).Scan(&res.Valid, &res.Message); err != nil {
		return nil, domain.Internal("Could not run unit of measurement validation", err)
	}
	return res, nil
}

func (r *uomRepository) checkAll(ctx context.Context, tx pgx.Tx, u *domain_stock.UOM) error {
	taken, err := r.taken(ctx, tx, u)
	if err != nil {
		return err
	}
	if len(taken) > 0 {
		return domain.Conflict("Unit of measurement already exists", taken...)
	}
	res, err := r.checkWithFunction(ctx, tx, u)
	if err != nil {
		return err
	}
	if !res.Valid {
		return domain_stock.UOMFunctionError(res.Message)
	}
	return nil
}

func (r *uomRepository) Create(ctx context.Context, u *domain_stock.UOM) (int64, error) {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return 0, err
	}
	var id int64
	err = pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if err := lock(ctx, tx, uomTable); err != nil {
			return err
		}
		if err := r.checkAll(ctx, tx, u); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `CALL pms.insert_mst_stk_uom($1::varchar, $2::varchar, $3::boolean, $4::varchar)`,
			u.UOM, u.Standard, u.EnableDecimal, strconv.Itoa(u.NumberDecimal)); err != nil {
			if msg, ok := raised(err); ok {
				return domain_stock.UOMFunctionError(msg)
			}
			return domain.Internal("Could not save unit of measurement", err)
		}
		id, err = newID(ctx, tx, uomTable, "uom_cd")
		return err
	})
	return id, err
}

func (r *uomRepository) Update(ctx context.Context, u *domain_stock.UOM) error {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if err := lock(ctx, tx, uomTable); err != nil {
			return err
		}
		if _, err := r.get(ctx, tx, u.ID, " FOR UPDATE"); err != nil {
			return err
		}
		if err := r.checkAll(ctx, tx, u); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `CALL pms.update_mst_stk_uom($1::integer, $2::varchar, $3::varchar, $4::boolean, $5::varchar)`,
			u.ID, u.UOM, u.Standard, u.EnableDecimal, strconv.Itoa(u.NumberDecimal)); err != nil {
			if msg, ok := raised(err); ok {
				if strings.Contains(msg, "does not exist") {
					return domain_stock.ErrUOMNotFound
				}
				return domain_stock.UOMFunctionError(msg)
			}
			return domain.Internal("Could not save unit of measurement", err)
		}
		return nil
	})
}

func (r *uomRepository) Delete(ctx context.Context, id int64) error {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if err := lock(ctx, tx, uomTable); err != nil {
			return err
		}
		u, err := r.get(ctx, tx, id, " FOR UPDATE")
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `CALL pms.delete_mst_stk_uom($1::integer, $2::varchar)`, id, u.UOM); err != nil {
			return deleteError(err, "Unit of measurement", domain_stock.ErrUOMNotFound)
		}
		return nil
	})
}
