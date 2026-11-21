package repository_vendor

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
	domain_vendor "github.com/utkarshk014/procurement-system-be/src/internal/domain/vendor"
	"github.com/utkarshk014/procurement-system-be/src/internal/repository/pgutil"
)

func (r *repository) Groups(ctx context.Context) ([]domain_vendor.Group, error) {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, `SELECT grp_cd, COALESCE(item_group, '') FROM pms.mst_stk_group ORDER BY LOWER(item_group)`)
	if err != nil {
		return nil, domain.Internal("Could not list item groups", err)
	}
	defer rows.Close()
	out := []domain_vendor.Group{}
	for rows.Next() {
		var g domain_vendor.Group
		if err := rows.Scan(&g.ID, &g.Name); err != nil {
			return nil, domain.Internal("Could not read item group", err)
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func errChanged(detail string) error {
	return domain.Conflict("The vendors changed since the file was checked: " + detail + ". Check the file again.")
}

// newVendorID is the id the DBA's insert just gave, for the group rows that follow it.
const newVendorID = `currval(pg_get_serial_sequence('pms.mst_vendor', 'vendor_cd'))::integer`

func (r *repository) CreateMany(ctx context.Context, vendors []domain_vendor.Vendor) error {
	if len(vendors) == 0 {
		return nil
	}
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if err := lock(ctx, tx); err != nil {
			return err
		}
		var names, pans, gstins []string
		var groups []int64
		for _, v := range vendors {
			names = append(names, strings.ToLower(v.Name))
			if v.PAN != "" {
				pans = append(pans, v.PAN)
			}
			if v.GSTIN != "" {
				gstins = append(gstins, v.GSTIN)
			}
			groups = append(groups, v.GroupIDs()...)
		}
		var clash string
		err := tx.QueryRow(ctx, `SELECT COALESCE(name, '#' || vendor_cd) FROM pms.mst_vendor
			WHERE regexp_replace(LOWER(BTRIM(name)), '\s+', ' ', 'g') = ANY($1::text[])
			   OR UPPER(BTRIM(pan)) = ANY($2::text[]) OR UPPER(BTRIM(gstin)) = ANY($3::text[]) LIMIT 1`, names, pans, gstins).Scan(&clash)
		switch {
		case err == nil:
			return errChanged(`vendor "` + clash + `" was added meanwhile`)
		case !pgutil.IsNoRows(err):
			return domain.Internal("Could not check vendors", err)
		}
		if len(groups) > 0 {
			var absent int
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM unnest($1::int[]) id
				WHERE NOT EXISTS (SELECT 1 FROM pms.mst_stk_group g WHERE g.grp_cd = id)`, groups).Scan(&absent); err != nil {
				return domain.Internal("Could not check item groups", err)
			}
			if absent > 0 {
				return errChanged("an item group was deleted")
			}
		}
		// The DBA's insert for each vendor (it runs his check), then his group insert for each of
		// its groups, all in one round trip.
		b := &pgx.Batch{}
		for i := range vendors {
			sql, args := namedCall("pms.insert_mst_vendor", "take_", columns(&vendors[i]))
			b.Queue(sql, args...)
			for _, g := range vendors[i].GroupIDs() {
				b.Queue(`CALL pms.insert_mst_vendor_item_group(take_vendor_cd => `+newVendorID+`, take_grp_cd => $1::integer)`, g)
			}
		}
		br := tx.SendBatch(ctx, b)
		for i := range vendors {
			for range 1 + len(vendors[i].GroupIDs()) {
				if _, err := br.Exec(); err != nil {
					_ = br.Close()
					return errChanged(vendors[i].Name + ": " + saveError(err).Error())
				}
			}
		}
		return br.Close()
	})
}
