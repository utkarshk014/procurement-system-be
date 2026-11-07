package repository_company

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
	domain_company "github.com/utkarshk014/procurement-system-be/src/internal/domain/company"
	"github.com/utkarshk014/procurement-system-be/src/internal/repository/pgutil"
)

type postgresCompanyRepository struct {
	pools pgutil.TenantPools
}

// NewPostgresCompanyRepository uses pms.company_master in the caller's tenant database.
func NewPostgresCompanyRepository(pools pgutil.TenantPools) domain_company.Repository {
	return &postgresCompanyRepository{pools: pools}
}

// lockSQL serialises company saves so concurrent first saves agree on create vs update.
const lockSQL = `SELECT pg_advisory_xact_lock(hashtext('pms.company_master'))`

const selectCompany = `
	SELECT sl,
	       COALESCE(name, ''), COALESCE(address, ''), COALESCE(city, ''), COALESCE(state, ''),
	       COALESCE(country, ''), COALESCE(pin, ''), COALESCE(mobile, ''), COALESCE(email, ''),
	       COALESCE(website, ''), COALESCE(header_1, ''), COALESCE(header_2, ''),
	       COALESCE(shortname, ''), COALESCE(to_char(fin_st_dt, 'YYYY-MM-DD'), ''),
	       time_zone, rec_created_at
	FROM pms.company_master
	WHERE sl = $1`

func scanCompany(row interface{ Scan(...any) error }, c *domain_company.Company) error {
	return row.Scan(&c.SL, &c.Name, &c.Address, &c.City, &c.State, &c.Country, &c.Pin,
		&c.Mobile, &c.Email, &c.Website, &c.Header1, &c.Header2, &c.ShortName,
		&c.FinStartDate, &c.TimeZone, &c.CreatedAt)
}

func (r *postgresCompanyRepository) Get(ctx context.Context) (*domain_company.Company, error) {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return nil, err
	}
	c := &domain_company.Company{}
	if err := scanCompany(pool.QueryRow(ctx, selectCompany, domain_company.SL), c); err != nil {
		if pgutil.IsNoRows(err) {
			return nil, domain.NotFound("Company is not set up yet")
		}
		return nil, domain.Internal("Could not load company", err)
	}
	return c, nil
}

func (r *postgresCompanyRepository) CheckWithProcedure(ctx context.Context, c *domain_company.Company) (*domain_company.ProcedureResult, error) {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return nil, err
	}
	return checkWithProcedure(ctx, pool, c)
}

func checkWithProcedure(ctx context.Context, q pgutil.Querier, c *domain_company.Company) (*domain_company.ProcedureResult, error) {
	res := &domain_company.ProcedureResult{}
	err := q.QueryRow(ctx, `
		SELECT is_valid, message
		FROM pms.company_master_validate($1::integer, $2::varchar, $3::varchar, $4::varchar,
		                                 $5::varchar, $6::varchar, $7::date)`,
		c.SL, pgutil.NullString(c.Name), pgutil.NullString(c.Mobile), pgutil.NullString(c.Email),
		pgutil.NullString(c.Pin), pgutil.NullString(c.ShortName), pgutil.NullString(c.FinStartDate),
	).Scan(&res.Valid, &res.Message)
	if err != nil {
		return nil, domain.Internal("Could not run company validation", err)
	}
	return res, nil
}

func (r *postgresCompanyRepository) Save(ctx context.Context, c *domain_company.Company) (bool, error) {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return false, err
	}
	c.SL = domain_company.SL
	created := false
	err = pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, lockSQL); err != nil {
			return domain.Internal("Could not lock company", err)
		}
		var timeZone *time.Time
		err := tx.QueryRow(ctx,
			`SELECT time_zone FROM pms.company_master WHERE sl = $1 FOR UPDATE`, c.SL).Scan(&timeZone)
		switch {
		case err == nil:
			c.TimeZone = timeZone // keep the original stamp on updates
		case pgutil.IsNoRows(err):
			created = true
		default:
			return domain.Internal("Could not load company", err)
		}

		// pms.company_master_insert does not validate on its own.
		res, err := checkWithProcedure(ctx, tx, c)
		if err != nil {
			return err
		}
		if !res.Valid {
			return domain.Validation(domain_company.ProcedureFieldError(res.Message))
		}
		_, err = tx.Exec(ctx, `
			CALL pms.company_master_insert($1::integer, $2::varchar, $3::varchar, $4::varchar,
			                               $5::varchar, $6::varchar, $7::varchar, $8::varchar,
			                               $9::varchar, $10::varchar, $11::varchar, $12::varchar,
			                               $13::varchar, $14::date, $15::timestamptz)`,
			c.SL, pgutil.NullString(c.Name), pgutil.NullString(c.Address), pgutil.NullString(c.City),
			pgutil.NullString(c.State), pgutil.NullString(c.Country), pgutil.NullString(c.Pin),
			pgutil.NullString(c.Mobile), pgutil.NullString(c.Email), pgutil.NullString(c.Website),
			pgutil.NullString(c.Header1), pgutil.NullString(c.Header2), pgutil.NullString(c.ShortName),
			pgutil.NullString(c.FinStartDate), c.TimeZone,
		)
		if err != nil {
			return domain.Internal("Could not save company", err)
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	return created, nil
}
