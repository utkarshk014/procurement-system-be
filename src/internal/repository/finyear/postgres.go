package repository_finyear

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
	domain_finyear "github.com/utkarshk014/procurement-system-be/src/internal/domain/finyear"
	"github.com/utkarshk014/procurement-system-be/src/internal/repository/pgutil"
)

type postgresFinYearRepository struct {
	pools pgutil.TenantPools
}

// NewPostgresFinYearRepository uses pms.finyr_master in the caller's tenant database.
func NewPostgresFinYearRepository(pools pgutil.TenantPools) domain_finyear.Repository {
	return &postgresFinYearRepository{pools: pools}
}

// lockSQL serialises creates so the DBA's duplicate and overlap checks cannot race.
const lockSQL = `SELECT pg_advisory_xact_lock(hashtext('pms.finyr_master'))`

const selectFinYear = `
	SELECT finyr,
	       COALESCE(to_char(finyr_st_date, 'YYYY-MM-DD'), ''),
	       COALESCE(to_char(finyr_end_date, 'YYYY-MM-DD'), ''),
	       COALESCE(finyr_is_current, FALSE),
	       rec_created_at
	FROM pms.finyr_master`

func scanFinYear(row interface{ Scan(...any) error }, fy *domain_finyear.FinancialYear) error {
	return row.Scan(&fy.FinYr, &fy.StartDate, &fy.EndDate, &fy.IsCurrent, &fy.CreatedAt)
}

func (r *postgresFinYearRepository) List(ctx context.Context) ([]domain_finyear.FinancialYear, error) {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, selectFinYear+` ORDER BY finyr DESC`)
	if err != nil {
		return nil, domain.Internal("Could not list financial years", err)
	}
	defer rows.Close()

	years := []domain_finyear.FinancialYear{}
	for rows.Next() {
		var fy domain_finyear.FinancialYear
		if err := scanFinYear(rows, &fy); err != nil {
			return nil, domain.Internal("Could not read financial year", err)
		}
		years = append(years, fy)
	}
	if err := rows.Err(); err != nil {
		return nil, domain.Internal("Could not list financial years", err)
	}
	return years, nil
}

func (r *postgresFinYearRepository) Get(ctx context.Context, finYr string) (*domain_finyear.FinancialYear, error) {
	return r.getOne(ctx, selectFinYear+` WHERE finyr = $1`, finYr)
}

func (r *postgresFinYearRepository) Current(ctx context.Context) (*domain_finyear.FinancialYear, error) {
	return r.getOne(ctx, selectFinYear+` WHERE finyr_is_current ORDER BY finyr DESC LIMIT 1`)
}

func (r *postgresFinYearRepository) getOne(ctx context.Context, sql string, args ...any) (*domain_finyear.FinancialYear, error) {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return nil, err
	}
	fy := &domain_finyear.FinancialYear{}
	if err := scanFinYear(pool.QueryRow(ctx, sql, args...), fy); err != nil {
		if pgutil.IsNoRows(err) {
			return nil, domain.NotFound("Financial year not found")
		}
		return nil, domain.Internal("Could not load financial year", err)
	}
	return fy, nil
}

func (r *postgresFinYearRepository) CheckWithFunction(ctx context.Context, fy *domain_finyear.FinancialYear) (string, error) {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return "", err
	}
	return checkWithFunction(ctx, pool, fy)
}

func checkWithFunction(ctx context.Context, q pgutil.Querier, fy *domain_finyear.FinancialYear) (string, error) {
	var msg string
	err := q.QueryRow(ctx,
		`SELECT pms.finyr_master_validate($1::varchar, $2::date, $3::date, $4::boolean)`,
		fy.FinYr, pgutil.NullString(fy.StartDate), pgutil.NullString(fy.EndDate), fy.IsCurrent,
	).Scan(&msg)
	if err != nil {
		return "", domain.Internal("Could not run financial year validation", err)
	}
	return msg, nil
}

func (r *postgresFinYearRepository) SetCurrent(ctx context.Context, finYr string) (bool, error) {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return false, err
	}
	changed := false
	err = pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		// The same lock as create, so a create deciding "is any year current?" cannot interleave.
		if _, err := tx.Exec(ctx, lockSQL); err != nil {
			return domain.Internal("Could not lock financial years", err)
		}
		var isCurrent bool
		err := tx.QueryRow(ctx,
			`SELECT COALESCE(finyr_is_current, FALSE) FROM pms.finyr_master WHERE finyr = $1`, finYr).Scan(&isCurrent)
		switch {
		case pgutil.IsNoRows(err):
			return domain.Validation(domain.FieldError{Field: "finyr", Message: "must be an existing financial year"})
		case err != nil:
			return domain.Internal("Could not load financial year", err)
		case isCurrent:
			return nil // already current: no change
		}
		if _, err := tx.Exec(ctx, `CALL pms.mst_finyr_set_current($1::varchar)`, finYr); err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "P0001" {
				return domain_finyear.FunctionError(pgErr.Message)
			}
			return domain.Internal("Could not change the current financial year", err)
		}
		changed = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return changed, nil
}

func (r *postgresFinYearRepository) Create(ctx context.Context, fy *domain_finyear.FinancialYear, decideCurrent domain_finyear.CurrentRule) error {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, lockSQL); err != nil {
			return domain.Internal("Could not lock financial years", err)
		}
		var hasCurrent bool
		if err := tx.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM pms.finyr_master WHERE finyr_is_current)`).Scan(&hasCurrent); err != nil {
			return domain.Internal("Could not check current financial year", err)
		}
		fy.IsCurrent = decideCurrent(hasCurrent)
		msg, err := checkWithFunction(ctx, tx, fy)
		if err != nil {
			return err
		}
		if msg != domain_finyear.FunctionSuccess {
			return domain_finyear.FunctionError(msg)
		}
		_, err = tx.Exec(ctx,
			`CALL pms.mst_finyr_insert($1::varchar, $2::date, $3::date, $4::boolean)`,
			fy.FinYr, pgutil.NullString(fy.StartDate), pgutil.NullString(fy.EndDate), fy.IsCurrent)
		if err != nil {
			// The procedure RAISEs its own checks as SQLSTATE P0001.
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "P0001" {
				return domain_finyear.FunctionError(pgErr.Message)
			}
			return domain.Internal("Could not save financial year", err)
		}
		return nil
	})
}
