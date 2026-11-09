package repository_codelist

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
	domain_codelist "github.com/utkarshk014/procurement-system-be/src/internal/domain/codelist"
	"github.com/utkarshk014/procurement-system-be/src/internal/repository/pgutil"
)

type postgresCodeListRepository struct {
	pools pgutil.TenantPools
	kind  domain_codelist.Kind
}

// NewPostgresCodeListRepository uses one list's table in the caller's tenant database. The
// table, column and routine names come from kind, which only holds constants.
func NewPostgresCodeListRepository(pools pgutil.TenantPools, kind domain_codelist.Kind) domain_codelist.Repository {
	return &postgresCodeListRepository{pools: pools, kind: kind}
}

func (r *postgresCodeListRepository) selectSQL() string {
	return `SELECT ` + r.kind.CodeCol + `, ` + r.kind.NameCol + `, rec_created_at, last_update_at FROM ` + r.kind.Table
}

func scanEntry(row interface{ Scan(...any) error }, e *domain_codelist.Entry) error {
	return row.Scan(&e.Code, &e.Name, &e.CreatedAt, &e.UpdatedAt)
}

func (r *postgresCodeListRepository) List(ctx context.Context) ([]domain_codelist.Entry, error) {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, r.selectSQL()+` ORDER BY LOWER(`+r.kind.NameCol+`), `+r.kind.CodeCol)
	if err != nil {
		return nil, domain.Internal("Could not list "+r.kind.Plural, err)
	}
	defer rows.Close()
	entries := []domain_codelist.Entry{}
	for rows.Next() {
		var e domain_codelist.Entry
		if err := scanEntry(rows, &e); err != nil {
			return nil, domain.Internal("Could not read "+r.kind.Label, err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, domain.Internal("Could not list "+r.kind.Plural, err)
	}
	return entries, nil
}

func (r *postgresCodeListRepository) Get(ctx context.Context, code string) (*domain_codelist.Entry, error) {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return nil, err
	}
	e := &domain_codelist.Entry{}
	if err := scanEntry(pool.QueryRow(ctx, r.selectSQL()+` WHERE `+r.kind.CodeCol+` = $1`, code), e); err != nil {
		if pgutil.IsNoRows(err) {
			return nil, r.kind.NotFound()
		}
		return nil, domain.Internal("Could not load "+r.kind.Label, err)
	}
	return e, nil
}

func (r *postgresCodeListRepository) NameTaken(ctx context.Context, name, exceptCode string) (string, bool, error) {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return "", false, err
	}
	return r.nameTaken(ctx, pool, name, exceptCode)
}

// nameTaken ignores case like the DBA's checks, and also runs of spaces, because we store names
// with single spaces: an older row "Mysore  Place" must still block "Mysore Place".
func (r *postgresCodeListRepository) nameTaken(ctx context.Context, q pgutil.Querier, name, exceptCode string) (string, bool, error) {
	var code string
	err := q.QueryRow(ctx, `
		SELECT `+r.kind.CodeCol+` FROM `+r.kind.Table+`
		WHERE regexp_replace(LOWER(TRIM(`+r.kind.NameCol+`)), '\s+', ' ', 'g') = regexp_replace(LOWER(TRIM($1)), '\s+', ' ', 'g')
		  AND `+r.kind.CodeCol+` <> $2
		ORDER BY `+r.kind.CodeCol+` LIMIT 1`, name, exceptCode).Scan(&code)
	switch {
	case err == nil:
		return code, true, nil
	case pgutil.IsNoRows(err):
		return "", false, nil
	default:
		return "", false, domain.Internal("Could not check "+r.kind.Label+" name", err)
	}
}

func (r *postgresCodeListRepository) CheckWithFunction(ctx context.Context, e *domain_codelist.Entry) (*domain_codelist.FunctionResult, error) {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return nil, err
	}
	return r.checkWithFunction(ctx, pool, e)
}

func (r *postgresCodeListRepository) checkWithFunction(ctx context.Context, q pgutil.Querier, e *domain_codelist.Entry) (*domain_codelist.FunctionResult, error) {
	res := &domain_codelist.FunctionResult{}
	err := q.QueryRow(ctx, `SELECT is_valid, error_message FROM `+r.kind.Validate+`($1::varchar, $2::varchar)`,
		e.Code, e.Name).Scan(&res.Valid, &res.Message)
	if err != nil {
		return nil, domain.Internal("Could not run "+r.kind.Label+" validation", err)
	}
	return res, nil
}

// lockSQL serialises writes to one list so the duplicate checks and the insert see the same rows.
func (r *postgresCodeListRepository) lockSQL() string {
	return `SELECT pg_advisory_xact_lock(hashtext('` + r.kind.Table + `'))`
}

// Create runs one locked transaction: the code and name must be free, the DBA's check is
// re-run, then his insert procedure saves the row.
func (r *postgresCodeListRepository) Create(ctx context.Context, e *domain_codelist.Entry) error {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, r.lockSQL()); err != nil {
			return domain.Internal("Could not lock "+r.kind.Plural, err)
		}
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM `+r.kind.Table+` WHERE `+r.kind.CodeCol+` = $1)`,
			e.Code).Scan(&exists); err != nil {
			return domain.Internal("Could not check "+r.kind.Label+" code", err)
		}
		if exists {
			return r.kind.CodeTaken()
		}
		other, taken, err := r.nameTaken(ctx, tx, e.Name, e.Code)
		if err != nil {
			return err
		}
		if taken {
			return r.kind.NameTaken(other)
		}
		res, err := r.checkWithFunction(ctx, tx, e)
		if err != nil {
			return err
		}
		if !res.Valid {
			return r.kind.FunctionError(res.Message)
		}
		if _, err := tx.Exec(ctx, `CALL `+r.kind.Insert+`($1::varchar, $2::varchar)`, e.Code, e.Name); err != nil {
			// The procedure RAISEs its own checks as SQLSTATE P0001.
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "P0001" {
				return r.kind.FunctionError(pgErr.Message)
			}
			if _, ok := pgutil.UniqueViolation(err); ok {
				return domain.Conflict(r.kind.Label+" already exists",
					domain.FieldError{Field: "name", Message: "is already taken"})
			}
			return domain.Internal("Could not save "+r.kind.Label, err)
		}
		return nil
	})
}
