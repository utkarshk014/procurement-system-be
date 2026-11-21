package repository_vendor

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
	domain_vendor "github.com/utkarshk014/procurement-system-be/src/internal/domain/vendor"
	"github.com/utkarshk014/procurement-system-be/src/internal/repository/pgutil"
)

// ---- Financials

// financials lists the vendor's years, newest first; finYear limits it to one year.
func financials(ctx context.Context, q pgutil.Querier, vendorID int64, finYear string) ([]domain_vendor.Financial, error) {
	sql := `SELECT fin_year, fin_turnover_declared::float8, fin_profit_declared::float8, COALESCE(fin_itr_no, ''),
	               to_char(fin_itr_date, 'YYYY-MM-DD'), COALESCE(fin_itr_s3_bucket_reference, '')
	        FROM pms.mst_vendor_financials WHERE vendor_cd = $1`
	args := []any{vendorID}
	if finYear != "" {
		sql += ` AND fin_year = $2`
		args = append(args, finYear)
	}
	rows, err := q.Query(ctx, sql+` ORDER BY fin_year DESC`, args...)
	if err != nil {
		return nil, domain.Internal("Could not load the vendor's financials", err)
	}
	defer rows.Close()
	out := []domain_vendor.Financial{}
	for rows.Next() {
		var f domain_vendor.Financial
		if err := rows.Scan(&f.FinYear, &f.Turnover, &f.Profit, &f.ITRNo, &f.ITRDate, &f.ITRFile); err != nil {
			return nil, domain.Internal("Could not read the vendor's financials", err)
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, domain.Internal("Could not load the vendor's financials", err)
	}
	return out, nil
}

func (r *repository) GetFinancial(ctx context.Context, vendorID int64, finYear string) (*domain_vendor.Financial, error) {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return nil, err
	}
	list, err := financials(ctx, pool, vendorID, finYear)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, domain_vendor.ErrFinancialNotFound
	}
	return &list[0], nil
}

func financialColumns(vendorID int64, f *domain_vendor.Financial) []column {
	var date any
	if f.ITRDate != nil {
		date = *f.ITRDate
	}
	return []column{
		{"vendor_cd", "integer", vendorID}, {"fin_year", "varchar", f.FinYear},
		{"fin_turnover_declared", "numeric", f.Turnover}, {"fin_profit_declared", "numeric", f.Profit},
		{"fin_itr_no", "varchar", pgutil.NullString(f.ITRNo)}, {"fin_itr_date", "date", date},
		{"fin_itr_s3_bucket_reference", "varchar", pgutil.NullString(f.ITRFile)},
	}
}

// childError maps a failed call of a financials or certificates procedure.
func childError(err error, notFound error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "P0001" {
		msg := strings.TrimSpace(strings.ReplaceAll(pgErr.Message, `""`, `"`))
		lower := strings.ToLower(msg)
		switch {
		case strings.HasPrefix(msg, "Vendor code") && strings.Contains(lower, "does not exist"):
			return domain_vendor.ErrVendorNotFound
		case strings.Contains(lower, "not found"), strings.Contains(lower, "does not exist"), strings.Contains(lower, "no longer exists"):
			return notFound
		case strings.Contains(lower, "already exist"):
			return domain.Conflict(msg, domain.FieldError{Field: "fin_year", Message: "already has financials"})
		case strings.HasPrefix(msg, "Financial year"):
			return domain.Validation(domain.FieldError{Field: "fin_year", Message: msg})
		case strings.Contains(lower, "turnover"):
			return domain.Validation(domain.FieldError{Field: "fin_turnover_declared", Message: msg})
		case strings.Contains(lower, "profit"):
			return domain.Validation(domain.FieldError{Field: "fin_profit_declared", Message: msg})
		case strings.HasPrefix(msg, "ITR"):
			return domain.Validation(domain.FieldError{Field: "fin_itr_no", Message: msg})
		}
		return domain.Validation(domain.FieldError{Message: msg})
	}
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return domain.Conflict("Financials for that year already exist", domain.FieldError{Field: "fin_year", Message: "already has financials"})
	}
	if errors.As(err, &pgErr) && pgErr.Code == "23503" {
		return domain_vendor.ErrVendorNotFound
	}
	return domain.Internal("Could not save the vendor's details", err)
}

// lockedVendor takes the vendor lock and checks the vendor exists.
func lockedVendor(ctx context.Context, tx pgx.Tx, vendorID int64) error {
	if err := lock(ctx, tx); err != nil {
		return err
	}
	var ok bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pms.mst_vendor WHERE vendor_cd = $1)`, vendorID).Scan(&ok); err != nil {
		return domain.Internal("Could not load vendor", err)
	}
	if !ok {
		return domain_vendor.ErrVendorNotFound
	}
	return nil
}

// storedFinancialFile returns the year's file under a row lock, or ErrFinancialNotFound.
func storedFinancialFile(ctx context.Context, tx pgx.Tx, vendorID int64, finYear string) (string, error) {
	var file string
	err := tx.QueryRow(ctx, `SELECT COALESCE(fin_itr_s3_bucket_reference, '') FROM pms.mst_vendor_financials
		WHERE vendor_cd = $1 AND fin_year = $2 FOR UPDATE`, vendorID, finYear).Scan(&file)
	if pgutil.IsNoRows(err) {
		return "", domain_vendor.ErrFinancialNotFound
	}
	if err != nil {
		return "", domain.Internal("Could not load the vendor's financials", err)
	}
	return file, nil
}

func (r *repository) CreateFinancial(ctx context.Context, vendorID int64, f *domain_vendor.Financial) error {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if err := lockedVendor(ctx, tx, vendorID); err != nil {
			return err
		}
		if _, err := storedFinancialFile(ctx, tx, vendorID, f.FinYear); err == nil {
			return domain.Conflict("Financials for that year already exist", domain.FieldError{Field: "fin_year", Message: "already has financials"})
		} else if !errors.Is(err, domain_vendor.ErrFinancialNotFound) {
			return err
		}
		sql, args := namedCall("pms.insert_mst_vendor_financials", "take_", financialColumns(vendorID, f))
		if _, err := tx.Exec(ctx, sql, args...); err != nil {
			return childError(err, domain_vendor.ErrFinancialNotFound)
		}
		return nil
	})
}

func (r *repository) UpdateFinancial(ctx context.Context, vendorID int64, f *domain_vendor.Financial) (string, error) {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return "", err
	}
	var old string
	err = pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if err := lockedVendor(ctx, tx, vendorID); err != nil {
			return err
		}
		if old, err = storedFinancialFile(ctx, tx, vendorID, f.FinYear); err != nil {
			return err
		}
		sql, args := namedCall("pms.update_mst_vendor_financials", "take_", financialColumns(vendorID, f))
		if _, err := tx.Exec(ctx, sql, args...); err != nil {
			return childError(err, domain_vendor.ErrFinancialNotFound)
		}
		return nil
	})
	return old, err
}

func (r *repository) DeleteFinancial(ctx context.Context, vendorID int64, finYear string) (string, error) {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return "", err
	}
	var old string
	err = pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if err := lockedVendor(ctx, tx, vendorID); err != nil {
			return err
		}
		if old, err = storedFinancialFile(ctx, tx, vendorID, finYear); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `CALL pms.delete_mst_vendor_financials(take_vendor_cd => $1::integer, take_fin_year => $2::varchar)`, vendorID, finYear); err != nil {
			return childError(err, domain_vendor.ErrFinancialNotFound)
		}
		return nil
	})
	return old, err
}

// ---- Certificates

// certificateSelect reads every slot's file and the three captions, in CertificateSlots order.
var certificateSelect = func() string {
	var cols []string
	for _, s := range domain_vendor.CertificateSlots {
		cols = append(cols, "COALESCE("+s+"_s3_bucket_reference, '')")
		if domain_vendor.IsOther(s) {
			cols = append(cols, "COALESCE("+s+"_caption, '')")
		}
	}
	return `SELECT ` + strings.Join(cols, ", ") + ` FROM pms.mst_vendor_certificates WHERE vendor_cd = $1`
}()

// certificates returns the vendor's slots, every slot present (empty when there is no row).
func certificates(ctx context.Context, q pgutil.Querier, vendorID int64, forUpdate bool) (domain_vendor.Certificates, error) {
	out := domain_vendor.Certificates{}
	files := make([]string, len(domain_vendor.CertificateSlots))
	captions := map[string]*string{}
	var dest []any
	for i, s := range domain_vendor.CertificateSlots {
		dest = append(dest, &files[i])
		if domain_vendor.IsOther(s) {
			c := new(string)
			captions[s] = c
			dest = append(dest, c)
		}
	}
	sql := certificateSelect
	if forUpdate {
		sql += ` FOR UPDATE`
	}
	err := q.QueryRow(ctx, sql, vendorID).Scan(dest...)
	if err != nil && !pgutil.IsNoRows(err) {
		return nil, domain.Internal("Could not load the vendor's certificates", err)
	}
	for i, s := range domain_vendor.CertificateSlots {
		c := domain_vendor.Certificate{File: files[i]}
		if p := captions[s]; p != nil {
			c.Caption = *p
		}
		out[s] = c
	}
	return out, nil
}

func (r *repository) GetCertificates(ctx context.Context, vendorID int64) (domain_vendor.Certificates, error) {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return nil, err
	}
	return certificates(ctx, pool, vendorID, false)
}

func (r *repository) SaveCertificates(ctx context.Context, vendorID int64, c domain_vendor.Certificates) ([]string, error) {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return nil, err
	}
	var old []string
	err = pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if err := lockedVendor(ctx, tx, vendorID); err != nil {
			return err
		}
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pms.mst_vendor_certificates WHERE vendor_cd = $1)`, vendorID).Scan(&exists); err != nil {
			return domain.Internal("Could not load the vendor's certificates", err)
		}
		if exists {
			stored, err := certificates(ctx, tx, vendorID, true)
			if err != nil {
				return err
			}
			old = stored.Files()
		}
		cols := []column{{"vendor_cd", "integer", vendorID}}
		for _, s := range domain_vendor.CertificateSlots {
			slot := c[s]
			cols = append(cols, column{s + "_uploaded", "boolean", slot.File != ""})
			if domain_vendor.IsOther(s) {
				cols = append(cols, column{s + "_caption", "varchar", pgutil.NullString(slot.Caption)})
			}
			cols = append(cols, column{s + "_s3_bucket_reference", "varchar", pgutil.NullString(slot.File)})
		}
		proc := "pms.insert_mst_vendor_certificates"
		if exists {
			proc = "pms.update_mst_vendor_certificates"
		}
		sql, args := namedCall(proc, "take_", cols)
		if _, err := tx.Exec(ctx, sql, args...); err != nil {
			return certificateError(err)
		}
		return nil
	})
	return old, err
}

// certificateError maps the DBA's certificate messages ("Other certificate 1 caption is
// required …", "oth_1 caption …") to the slot's field.
func certificateError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "P0001" {
		msg := strings.TrimSpace(pgErr.Message)
		field := ""
		for _, s := range domain_vendor.CertificateSlots {
			if strings.HasPrefix(msg, s+" ") || strings.HasPrefix(msg, domain_vendor.CertificateLabels[s]+" ") {
				field = s + ".file"
				if strings.Contains(msg, "caption") {
					field = s + ".caption"
				}
			}
		}
		if strings.HasPrefix(msg, "Vendor code") {
			return domain_vendor.ErrVendorNotFound
		}
		return domain.Validation(domain.FieldError{Field: field, Message: msg})
	}
	return childError(err, domain_vendor.ErrVendorNotFound)
}
