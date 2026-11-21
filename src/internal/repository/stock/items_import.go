package repository_stock

import (
	"context"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
	domain_stock "github.com/utkarshk014/procurement-system-be/src/internal/domain/stock"
	"github.com/utkarshk014/procurement-system-be/src/internal/repository/pgutil"
)

// checkMany sends the DBA's validate function for every item in one batch. His function returns
// is_valid and a message instead of raising, so one bad row does not stop the others.
func (r *itemRepository) checkMany(ctx context.Context, q interface {
	SendBatch(context.Context, *pgx.Batch) pgx.BatchResults
}, items []domain_stock.Item) ([]domain_stock.FunctionResult, error) {
	b := &pgx.Batch{}
	for i := range items {
		it := items[i]
		it.ID = 0
		b.Queue(`SELECT is_valid, message FROM pms.fn_validate_mst_stk_items($16::integer, `+itemTypes+`)`, append(args(&it), int64(0))...)
	}
	br := q.SendBatch(ctx, b)
	defer br.Close()
	out := make([]domain_stock.FunctionResult, len(items))
	for i := range items {
		if err := br.QueryRow().Scan(&out[i].Valid, &out[i].Message); err != nil {
			return nil, domain.Internal("Could not run item validation", err)
		}
	}
	return out, nil
}

func (r *itemRepository) CheckManyWithFunction(ctx context.Context, items []domain_stock.Item) ([]domain_stock.FunctionResult, error) {
	if len(items) == 0 {
		return nil, nil
	}
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return nil, err
	}
	return r.checkMany(ctx, pool, items)
}

// errChanged is the 409 for an import whose data changed between the check and the save.
func errChanged(detail string) error {
	return domain.Conflict("The items changed since the file was checked: " + detail + ". Check the file again.")
}

func (r *itemRepository) CreateMany(ctx context.Context, items []domain_stock.Item) error {
	if len(items) == 0 {
		return nil
	}
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if err := lock(ctx, tx, itemsTable); err != nil {
			return err
		}
		// Links that still exist, one query per list.
		for li := 0; li < 4; li++ {
			var ids []int64
			var link domain_stock.ItemLink
			for i := range items {
				l := items[i].Links()[li]
				link = l
				if l.ID != nil {
					ids = append(ids, *l.ID)
				}
			}
			if len(ids) == 0 {
				continue
			}
			var absent []int64
			if err := tx.QueryRow(ctx, `SELECT COALESCE(array_agg(DISTINCT id), '{}') FROM unnest($1::int[]) id
				WHERE NOT EXISTS (SELECT 1 FROM `+link.Table+` t WHERE t.`+link.IDCol+` = id)`, ids).Scan(&absent); err != nil {
				return domain.Internal("Could not check "+link.Label, err)
			}
			if len(absent) > 0 {
				return errChanged(strings.ToLower(link.Label) + " #" + strconv.FormatInt(absent[0], 10) + " was deleted")
			}
		}
		// Descriptions and codes still free.
		descs := make([]string, len(items))
		codes := make([]string, len(items))
		for i := range items {
			descs[i] = strings.ToLower(items[i].Desc)
			codes[i] = items[i].Code
		}
		var clash string
		err := tx.QueryRow(ctx, `SELECT COALESCE(item_desc, item_code) FROM pms.mst_stk_items
			WHERE regexp_replace(LOWER(TRIM(item_desc)), '\s+', ' ', 'g') = ANY($1::text[]) OR UPPER(TRIM(item_code)) = ANY($2::text[])
			LIMIT 1`, descs, codes).Scan(&clash)
		switch {
		case err == nil:
			return errChanged(`"` + clash + `" was added meanwhile`)
		case !pgutil.IsNoRows(err):
			return domain.Internal("Could not check items", err)
		}
		// The DBA's check again, then his insert for every item.
		results, err := r.checkMany(ctx, tx, items)
		if err != nil {
			return err
		}
		for i, res := range results {
			if !res.Valid {
				return errChanged(items[i].Code + ": " + res.Message)
			}
		}
		b := &pgx.Batch{}
		for i := range items {
			b.Queue(`CALL pms.insert_mst_stk_items(`+itemTypes+`)`, args(&items[i])...)
		}
		br := tx.SendBatch(ctx, b)
		for i := range items {
			if _, err := br.Exec(); err != nil {
				_ = br.Close()
				return saveError(err)
			}
			_ = i
		}
		return br.Close()
	})
}
