package repository_trust

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
	domain_trust "github.com/utkarshk014/procurement-system-be/src/internal/domain/trust"
	"github.com/utkarshk014/procurement-system-be/src/internal/repository/pgutil"
)

type postgresTrustRepository struct {
	pools pgutil.TenantPools
}

// NewPostgresTrustRepository uses pms.mst_trust in the caller's tenant database.
func NewPostgresTrustRepository(pools pgutil.TenantPools) domain_trust.Repository {
	return &postgresTrustRepository{pools: pools}
}

// lockSQL serialises trust writes. mst_trust_insert is an upsert, so without the lock a
// create could silently overwrite a trust created a moment earlier with the same code.
const lockSQL = `SELECT pg_advisory_xact_lock(hashtext('pms.mst_trust'))`

const selectTrust = `
	SELECT trust_code, trust_desc, COALESCE(wrk_st_finyr, ''),
	       COALESCE(to_char(wrk_st_date, 'YYYY-MM-DD'), ''),
	       rec_created_at, last_update_at
	FROM pms.mst_trust`

func scanTrust(row interface{ Scan(...any) error }, t *domain_trust.Trust) error {
	return row.Scan(&t.Code, &t.Desc, &t.StartFY, &t.StartDate, &t.CreatedAt, &t.UpdatedAt)
}

func (r *postgresTrustRepository) List(ctx context.Context) ([]domain_trust.Trust, error) {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, selectTrust+` ORDER BY trust_code`)
	if err != nil {
		return nil, domain.Internal("Could not list trusts", err)
	}
	defer rows.Close()
	trusts := []domain_trust.Trust{}
	for rows.Next() {
		var t domain_trust.Trust
		if err := scanTrust(rows, &t); err != nil {
			return nil, domain.Internal("Could not read trust", err)
		}
		trusts = append(trusts, t)
	}
	if err := rows.Err(); err != nil {
		return nil, domain.Internal("Could not list trusts", err)
	}
	return trusts, nil
}

func (r *postgresTrustRepository) Get(ctx context.Context, code string) (*domain_trust.Trust, error) {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return nil, err
	}
	t := &domain_trust.Trust{}
	if err := scanTrust(pool.QueryRow(ctx, selectTrust+` WHERE trust_code = $1`, code), t); err != nil {
		if pgutil.IsNoRows(err) {
			return nil, domain_trust.ErrNotFound
		}
		return nil, domain.Internal("Could not load trust", err)
	}
	return t, nil
}

func (r *postgresTrustRepository) DescTaken(ctx context.Context, desc, exceptCode string) (string, bool, error) {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return "", false, err
	}
	return descTaken(ctx, pool, desc, exceptCode)
}

// descTaken matches like Uncle's validate: LOWER(TRIM(...)), stricter than the table's UNIQUE.
func descTaken(ctx context.Context, q pgutil.Querier, desc, exceptCode string) (string, bool, error) {
	var code string
	err := q.QueryRow(ctx, `
		SELECT trust_code FROM pms.mst_trust
		WHERE LOWER(TRIM(trust_desc)) = LOWER(TRIM($1)) AND trust_code <> $2
		ORDER BY trust_code LIMIT 1`, desc, exceptCode).Scan(&code)
	switch {
	case err == nil:
		return code, true, nil
	case pgutil.IsNoRows(err):
		return "", false, nil
	default:
		return "", false, domain.Internal("Could not check trust description", err)
	}
}

func (r *postgresTrustRepository) CheckWithFunction(ctx context.Context, t *domain_trust.Trust) (*domain_trust.FunctionResult, error) {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return nil, err
	}
	return checkWithFunction(ctx, pool, t)
}

func checkWithFunction(ctx context.Context, q pgutil.Querier, t *domain_trust.Trust) (*domain_trust.FunctionResult, error) {
	res := &domain_trust.FunctionResult{}
	err := q.QueryRow(ctx, `
		SELECT is_valid, take_error_msg
		FROM pms.mst_trust_validate($1::varchar, $2::varchar, $3::varchar, $4::date)`,
		t.Code, t.Desc, pgutil.NullString(t.StartFY), pgutil.NullString(t.StartDate),
	).Scan(&res.Valid, &res.Message)
	if err != nil {
		return nil, domain.Internal("Could not run trust validation", err)
	}
	return res, nil
}

func (r *postgresTrustRepository) Create(ctx context.Context, t *domain_trust.Trust) error {
	return r.write(ctx, t, true)
}

func (r *postgresTrustRepository) Update(ctx context.Context, t *domain_trust.Trust) error {
	return r.write(ctx, t, false)
}

// write runs one locked transaction: the code must be free (create) or exist (update), the
// description must be free, Uncle's check is re-run, then mst_trust_insert saves the row.
func (r *postgresTrustRepository) write(ctx context.Context, t *domain_trust.Trust, create bool) error {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, lockSQL); err != nil {
			return domain.Internal("Could not lock trusts", err)
		}
		var exists bool
		if err := tx.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM pms.mst_trust WHERE trust_code = $1)`, t.Code).Scan(&exists); err != nil {
			return domain.Internal("Could not check trust code", err)
		}
		switch {
		case create && exists:
			return domain_trust.ErrCodeTaken
		case !create && !exists:
			return domain_trust.ErrNotFound
		}
		other, taken, err := descTaken(ctx, tx, t.Desc, t.Code)
		if err != nil {
			return err
		}
		if taken {
			return domain_trust.DescTakenError(other)
		}

		res, err := checkWithFunction(ctx, tx, t)
		if err != nil {
			return err
		}
		if !res.Valid {
			return domain_trust.FunctionErrors(res.Message)
		}
		_, err = tx.Exec(ctx, `CALL pms.mst_trust_insert($1::varchar, $2::varchar, $3::varchar, $4::date)`,
			t.Code, t.Desc, t.StartFY, t.StartDate)
		if err != nil {
			// Only trust_desc is UNIQUE besides the key, and both are checked above under the lock.
			if _, ok := pgutil.UniqueViolation(err); ok {
				return domain.Conflict("Trust description already exists",
					domain.FieldError{Field: "trust_desc", Message: "is already taken"})
			}
			return domain.Internal("Could not save trust", err)
		}
		return nil
	})
}
