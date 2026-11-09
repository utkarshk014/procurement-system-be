package repository_taxdetails

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
	domain_taxdetails "github.com/utkarshk014/procurement-system-be/src/internal/domain/taxdetails"
	"github.com/utkarshk014/procurement-system-be/src/internal/repository/pgutil"
)

type postgresTaxDetailsRepository struct {
	pools pgutil.TenantPools
}

// NewPostgresTaxDetailsRepository uses pms.mst_company_tax_details in the caller's tenant database.
func NewPostgresTaxDetailsRepository(pools pgutil.TenantPools) domain_taxdetails.Repository {
	return &postgresTaxDetailsRepository{pools: pools}
}

// lockSQL serialises saves so concurrent first saves agree on create vs update.
const lockSQL = `SELECT pg_advisory_xact_lock(hashtext('pms.mst_company_tax_details'))`

// selectTaxDetails never reads the password, only whether one is stored.
const selectTaxDetails = `
	SELECT sl,
	       COALESCE(pan_enabled, FALSE), to_char(pan_enabled_dt, 'YYYY-MM-DD'), COALESCE(pan, ''),
	       COALESCE(gstin_enabled, FALSE), to_char(gstin_enabled_dt, 'YYYY-MM-DD'), COALESCE(gstin, ''),
	       COALESCE(gstin_state, ''), COALESCE(comm_mail_id, ''),
	       COALESCE(comm_mail_pwd, '') <> '',
	       COALESCE(comm_mobile, ''), COALESCE(comm_whatsapp, ''),
	       rec_created_at
	FROM pms.mst_company_tax_details
	WHERE sl = $1`

func (r *postgresTaxDetailsRepository) Get(ctx context.Context) (*domain_taxdetails.TaxDetails, error) {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return nil, err
	}
	t := &domain_taxdetails.TaxDetails{}
	err = pool.QueryRow(ctx, selectTaxDetails, domain_taxdetails.SL).Scan(
		&t.SL, &t.PANEnabled, &t.PANEnabledDate, &t.PAN,
		&t.GSTINEnabled, &t.GSTINEnabledDate, &t.GSTIN, &t.GSTINState,
		&t.CommMailID, &t.CommMailPwdSet, &t.CommMobile, &t.CommWhatsapp, &t.CreatedAt)
	if err != nil {
		if pgutil.IsNoRows(err) {
			return nil, domain.NotFound("Company tax details are not saved yet")
		}
		return nil, domain.Internal("Could not load company tax details", err)
	}
	return t, nil
}

const companyExistsSQL = `SELECT EXISTS (SELECT 1 FROM pms.company_master WHERE sl = $1)`

func (r *postgresTaxDetailsRepository) CompanyExists(ctx context.Context) (bool, error) {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return false, err
	}
	var ok bool
	if err := pool.QueryRow(ctx, companyExistsSQL, domain_taxdetails.SL).Scan(&ok); err != nil {
		return false, domain.Internal("Could not check company", err)
	}
	return ok, nil
}

func (r *postgresTaxDetailsRepository) Save(ctx context.Context, t *domain_taxdetails.TaxDetails, pwd domain_taxdetails.PasswordChange) (bool, error) {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return false, err
	}
	t.SL = domain_taxdetails.SL
	created := false
	err = pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, lockSQL); err != nil {
			return domain.Internal("Could not lock company tax details", err)
		}
		var companyOK bool
		if err := tx.QueryRow(ctx, companyExistsSQL, t.SL).Scan(&companyOK); err != nil {
			return domain.Internal("Could not check company", err)
		}
		if !companyOK {
			return domain_taxdetails.ErrCompanyFirst
		}

		// The procedure replaces every column, so read the stored password to keep it.
		var stored *string
		err := tx.QueryRow(ctx,
			`SELECT comm_mail_pwd FROM pms.mst_company_tax_details WHERE sl = $1 FOR UPDATE`, t.SL).Scan(&stored)
		switch {
		case err == nil:
		case pgutil.IsNoRows(err):
			created = true
		default:
			return domain.Internal("Could not load company tax details", err)
		}
		var password any // NULL clears it
		switch {
		case pwd.Keep && stored != nil:
			password = *stored
		case !pwd.Keep && pwd.Value != "":
			password = pwd.Value
		}

		_, err = tx.Exec(ctx, `
			CALL pms.mst_company_tax_details_insert($1::integer, $2::boolean, $3::date, $4::varchar,
			                                        $5::boolean, $6::date, $7::varchar, $8::varchar,
			                                        $9::varchar, $10::varchar, $11::varchar, $12::varchar)`,
			t.SL, t.PANEnabled, t.PANEnabledDate, pgutil.NullString(t.PAN),
			t.GSTINEnabled, t.GSTINEnabledDate, pgutil.NullString(t.GSTIN), pgutil.NullString(t.GSTINState),
			pgutil.NullString(t.CommMailID), password,
			pgutil.NullString(t.CommMobile), pgutil.NullString(t.CommWhatsapp),
		)
		if err != nil {
			return domain.Internal("Could not save company tax details", err)
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	return created, nil
}
