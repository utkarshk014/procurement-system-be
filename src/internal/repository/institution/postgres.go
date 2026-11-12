package repository_institution

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
	domain_institution "github.com/utkarshk014/procurement-system-be/src/internal/domain/institution"
	"github.com/utkarshk014/procurement-system-be/src/internal/repository/pgutil"
)

type postgresInstitutionRepository struct {
	pools pgutil.TenantPools
}

// NewPostgresInstitutionRepository uses pms.mst_inst in the caller's tenant database.
func NewPostgresInstitutionRepository(pools pgutil.TenantPools) domain_institution.Repository {
	return &postgresInstitutionRepository{pools: pools}
}

// lockSQL serialises institution writes so the duplicate checks and the insert see the same rows.
const lockSQL = `SELECT pg_advisory_xact_lock(hashtext('pms.mst_inst'))`

const selectInst = `
	SELECT inst_code, inst_reg_code, inst_full_code, COALESCE(inst_name, ''), COALESCE(inst_add, ''),
	       COALESCE(inst_place, ''), COALESCE(place_code, ''), COALESCE(place_name, ''),
	       COALESCE(branch_code, ''), COALESCE(branch_name, ''), COALESCE(trust_code, ''), COALESCE(trust_name, ''),
	       COALESCE(to_char(reg_date, 'YYYY-MM-DD'), ''), COALESCE(from_fin_yr, ''),
	       COALESCE(inst_email, ''), COALESCE(inst_tel, ''),
	       COALESCE(inst_head_name, ''), COALESCE(inst_head_desig, ''), COALESCE(inst_head_mobile, ''), COALESCE(inst_head_email, ''),
	       COALESCE(inst_acnt_name, ''), COALESCE(inst_acnt_mobile, ''), COALESCE(inst_acnt_email, ''),
	       COALESCE(inst_liason_name, ''), COALESCE(inst_liason_desig, ''), COALESCE(inst_liason_mobile, ''), COALESCE(inst_liason_email, ''),
	       rec_created_at, last_update_at
	FROM pms.mst_inst`

func scanInst(row interface{ Scan(...any) error }, i *domain_institution.Institution) error {
	return row.Scan(&i.Code, &i.RegCode, &i.FullCode, &i.Name, &i.Address,
		&i.Locality, &i.PlaceCode, &i.PlaceName,
		&i.BranchCode, &i.BranchName, &i.TrustCode, &i.TrustName,
		&i.RegDate, &i.FromFinYr,
		&i.Email, &i.Tel,
		&i.HeadName, &i.HeadDesig, &i.HeadMobile, &i.HeadEmail,
		&i.AcntName, &i.AcntMobile, &i.AcntEmail,
		&i.LiasonName, &i.LiasonDesig, &i.LiasonMobile, &i.LiasonEmail,
		&i.CreatedAt, &i.UpdatedAt)
}

func (r *postgresInstitutionRepository) List(ctx context.Context, f domain_institution.Filter) (*domain_institution.Page, error) {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return nil, err
	}
	var where []string
	var args []any
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, strings.ReplaceAll(cond, "?", "$"+strconv.Itoa(len(args))))
	}
	if f.TrustCode != "" {
		add("trust_code = ?", f.TrustCode)
	}
	if f.BranchCode != "" {
		add("branch_code = ?", f.BranchCode)
	}
	if f.PlaceCode != "" {
		add("place_code = ?", f.PlaceCode)
	}
	if f.Q != "" {
		add("(inst_name ILIKE ? OR inst_full_code ILIKE ?)", "%"+escapeLike(f.Q)+"%")
	}
	cond := ""
	if len(where) > 0 {
		cond = " WHERE " + strings.Join(where, " AND ")
	}

	page := &domain_institution.Page{Items: []domain_institution.Institution{}, Page: f.Page, PageSize: f.PageSize}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM pms.mst_inst`+cond, args...).Scan(&page.Total); err != nil {
		return nil, domain.Internal("Could not count institutions", err)
	}
	args = append(args, f.PageSize, (f.Page-1)*f.PageSize)
	rows, err := pool.Query(ctx, selectInst+cond+` ORDER BY inst_full_code LIMIT $`+strconv.Itoa(len(args)-1)+` OFFSET $`+strconv.Itoa(len(args)), args...)
	if err != nil {
		return nil, domain.Internal("Could not list institutions", err)
	}
	defer rows.Close()
	for rows.Next() {
		var i domain_institution.Institution
		if err := scanInst(rows, &i); err != nil {
			return nil, domain.Internal("Could not read institution", err)
		}
		page.Items = append(page.Items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, domain.Internal("Could not list institutions", err)
	}
	return page, nil
}

// escapeLike makes % and _ in a search match themselves.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func (r *postgresInstitutionRepository) Get(ctx context.Context, code string) (*domain_institution.Institution, error) {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return nil, err
	}
	i := &domain_institution.Institution{}
	if err := scanInst(pool.QueryRow(ctx, selectInst+` WHERE inst_code = $1`, code), i); err != nil {
		if pgutil.IsNoRows(err) {
			return nil, domain_institution.ErrNotFound
		}
		return nil, domain.Internal("Could not load institution", err)
	}
	return i, nil
}

func (r *postgresInstitutionRepository) NextNumbers(ctx context.Context, trustCode, regionCode string) (int, int, error) {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return 0, 0, err
	}
	var code, reg int
	err = pool.QueryRow(ctx, `
		SELECT
		  COALESCE((SELECT MAX(SUBSTRING(inst_code FROM 5)::int) FROM pms.mst_inst
		            WHERE inst_code ~ ('^' || $1::text || '-[0-9]{3}$')), 0) + 1,
		  COALESCE((SELECT MAX(SUBSTRING(inst_reg_code FROM 5)::int) FROM pms.mst_inst
		            WHERE inst_reg_code ~ ('^' || $2::text || '-[0-9]{2}$')), 0) + 1`,
		trustCode, regionCode).Scan(&code, &reg)
	if err != nil {
		return 0, 0, domain.Internal("Could not work out the next institution codes", err)
	}
	return code, reg, nil
}

func (r *postgresInstitutionRepository) Taken(ctx context.Context, in *domain_institution.Institution) (string, string, error) {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return "", "", err
	}
	return taken(ctx, pool, in)
}

// taken checks the ACM code, then the reg code. The full code needs no check of its own: it is
// inst_code ':' inst_reg_code, so it is free whenever the ACM code is.
func taken(ctx context.Context, q pgutil.Querier, in *domain_institution.Institution) (string, string, error) {
	checks := []struct{ field, col, value string }{
		{"inst_code", "inst_code", in.Code},
		{"inst_reg_code", "inst_reg_code", in.RegCode},
	}
	for _, c := range checks {
		var by string
		err := q.QueryRow(ctx, `SELECT inst_code FROM pms.mst_inst WHERE `+c.col+` = $1`, c.value).Scan(&by)
		switch {
		case err == nil:
			return c.field, by, nil
		case !pgutil.IsNoRows(err):
			return "", "", domain.Internal("Could not check institution codes", err)
		}
	}
	return "", "", nil
}

func (r *postgresInstitutionRepository) CheckWithFunction(ctx context.Context, in *domain_institution.Institution) (*domain_institution.FunctionResult, error) {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return nil, err
	}
	return checkWithFunction(ctx, pool, in)
}

func checkWithFunction(ctx context.Context, q pgutil.Querier, i *domain_institution.Institution) (*domain_institution.FunctionResult, error) {
	res := &domain_institution.FunctionResult{}
	n := pgutil.NullString
	err := q.QueryRow(ctx, `
		SELECT is_valid, error_message FROM pms.fn_mst_inst_validate(
		  $1::varchar, $2::varchar, $3::varchar, $4::varchar, $5::varchar, $6::varchar, $7::varchar,
		  $8::varchar, $9::varchar, $10::varchar, $11::varchar, $12::varchar, $13::varchar)`,
		i.Code, i.RegCode, i.FullCode, n(i.PlaceCode), n(i.BranchCode), n(i.TrustCode), n(i.Email),
		n(i.HeadMobile), n(i.HeadEmail), n(i.AcntMobile), n(i.AcntEmail), n(i.LiasonMobile), n(i.LiasonEmail),
	).Scan(&res.Valid, &res.Message)
	if err != nil {
		return nil, domain.Internal("Could not run institution validation", err)
	}
	return res, nil
}

// Create runs one locked transaction: the codes must be free, the DBA's check is re-run, then
// pms.sp_mst_inst_insert saves the row. The four *_sl_idx columns are sent as NULL until the
// DBA defines how they are calculated.
func (r *postgresInstitutionRepository) Create(ctx context.Context, i *domain_institution.Institution) error {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, lockSQL); err != nil {
			return domain.Internal("Could not lock institutions", err)
		}
		field, by, err := taken(ctx, tx, i)
		if err != nil {
			return err
		}
		if field != "" {
			return domain_institution.CodeTaken(field, by)
		}
		res, err := checkWithFunction(ctx, tx, i)
		if err != nil {
			return err
		}
		if !res.Valid {
			return domain_institution.FunctionError(res.Message)
		}
		n := pgutil.NullString
		_, err = tx.Exec(ctx, `CALL pms.sp_mst_inst_insert(
		  $1::varchar, $2::varchar, $3::varchar, $4::varchar, $5::varchar, $6::varchar,
		  $7::varchar, $8::varchar, $9::varchar,
		  NULL::integer, NULL::integer, NULL::integer, NULL::integer,
		  $10::date, $11::varchar,
		  $12::varchar, $13::varchar,
		  $14::varchar, $15::varchar, $16::varchar, $17::varchar,
		  $18::varchar, $19::varchar, $20::varchar,
		  $21::varchar, $22::varchar, $23::varchar, $24::varchar)`,
			i.Code, i.RegCode, i.FullCode, i.Name, n(i.Address), n(i.Locality),
			i.PlaceCode, i.BranchCode, i.TrustCode,
			i.RegDate, i.FromFinYr,
			n(i.Email), n(i.Tel),
			n(i.HeadName), n(i.HeadDesig), n(i.HeadMobile), n(i.HeadEmail),
			n(i.AcntName), n(i.AcntMobile), n(i.AcntEmail),
			n(i.LiasonName), n(i.LiasonDesig), n(i.LiasonMobile), n(i.LiasonEmail))
		if err != nil {
			// The procedure RAISEs its own checks as SQLSTATE P0001.
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "P0001" {
				return domain_institution.FunctionError(pgErr.Message)
			}
			if constraint, ok := pgutil.UniqueViolation(err); ok {
				field := "inst_code"
				if constraint == "mst_inst_inst_reg_code_key" {
					field = "inst_reg_code"
				}
				return domain_institution.CodeTaken(field, "")
			}
			return domain.Internal("Could not save institution", err)
		}
		return nil
	})
}

func (r *postgresInstitutionRepository) ExistingCodes(ctx context.Context) (map[string]string, map[string]string, error) {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return nil, nil, err
	}
	return existingCodes(ctx, pool)
}

func existingCodes(ctx context.Context, q pgutil.Querier) (map[string]string, map[string]string, error) {
	rows, err := q.Query(ctx, `SELECT inst_code, inst_reg_code FROM pms.mst_inst`)
	if err != nil {
		return nil, nil, domain.Internal("Could not read institution codes", err)
	}
	defer rows.Close()
	codes, regs := map[string]string{}, map[string]string{}
	for rows.Next() {
		var code, reg string
		if err := rows.Scan(&code, &reg); err != nil {
			return nil, nil, domain.Internal("Could not read institution codes", err)
		}
		codes[code], regs[reg] = code, code
	}
	if err := rows.Err(); err != nil {
		return nil, nil, domain.Internal("Could not read institution codes", err)
	}
	return codes, regs, nil
}

func (r *postgresInstitutionRepository) CheckManyWithFunction(ctx context.Context, list []domain_institution.Institution) ([]domain_institution.FunctionResult, error) {
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return nil, err
	}
	return checkMany(ctx, pool, list)
}

// checkMany calls the DBA's validate function once per row, all in one query: the rows go in as
// parallel arrays and come back in the same order (WITH ORDINALITY).
func checkMany(ctx context.Context, q pgutil.Querier, list []domain_institution.Institution) ([]domain_institution.FunctionResult, error) {
	if len(list) == 0 {
		return nil, nil
	}
	cols := make([][]*string, 13)
	for i := range cols {
		cols[i] = make([]*string, len(list))
	}
	opt := func(s string) *string {
		if s == "" {
			return nil
		}
		return &s
	}
	for i, in := range list {
		vals := []string{in.Code, in.RegCode, in.FullCode, in.PlaceCode, in.BranchCode, in.TrustCode, in.Email,
			in.HeadMobile, in.HeadEmail, in.AcntMobile, in.AcntEmail, in.LiasonMobile, in.LiasonEmail}
		for c, v := range vals {
			if c < 3 {
				cols[c][i] = &vals[c] // the three codes are always sent, even if empty
			} else {
				cols[c][i] = opt(v)
			}
		}
	}
	args := make([]any, len(cols))
	for i := range cols {
		args[i] = cols[i]
	}
	rows, err := q.Query(ctx, `
		SELECT v.is_valid, v.error_message
		FROM unnest($1::text[], $2::text[], $3::text[], $4::text[], $5::text[], $6::text[], $7::text[],
		            $8::text[], $9::text[], $10::text[], $11::text[], $12::text[], $13::text[])
		     WITH ORDINALITY AS t(a1, a2, a3, a4, a5, a6, a7, a8, a9, a10, a11, a12, a13, n)
		CROSS JOIN LATERAL pms.fn_mst_inst_validate(
		     t.a1::varchar, t.a2::varchar, t.a3::varchar, t.a4::varchar, t.a5::varchar, t.a6::varchar, t.a7::varchar,
		     t.a8::varchar, t.a9::varchar, t.a10::varchar, t.a11::varchar, t.a12::varchar, t.a13::varchar) AS v
		ORDER BY t.n`, args...)
	if err != nil {
		return nil, domain.Internal("Could not run institution validation", err)
	}
	defer rows.Close()
	out := make([]domain_institution.FunctionResult, 0, len(list))
	for rows.Next() {
		var res domain_institution.FunctionResult
		if err := rows.Scan(&res.Valid, &res.Message); err != nil {
			return nil, domain.Internal("Could not read institution validation", err)
		}
		out = append(out, res)
	}
	if err := rows.Err(); err != nil {
		return nil, domain.Internal("Could not run institution validation", err)
	}
	if len(out) != len(list) {
		return nil, domain.Internal("Institution validation returned the wrong number of rows", nil)
	}
	return out, nil
}

// ErrImportChanged is returned when, between the check and the save, another user took one of
// the file's codes or the DBA's check started failing. Nothing was saved.
var ErrImportChanged = domain.Conflict("Something changed while importing (a code was taken or a check now fails). Nothing was saved; check the file again.")

func (r *postgresInstitutionRepository) CreateMany(ctx context.Context, list []domain_institution.Institution) error {
	if len(list) == 0 {
		return nil
	}
	pool, err := pgutil.TenantPool(ctx, r.pools)
	if err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, lockSQL); err != nil {
			return domain.Internal("Could not lock institutions", err)
		}
		codes, regs, err := existingCodes(ctx, tx)
		if err != nil {
			return err
		}
		for _, in := range list {
			if _, ok := codes[in.Code]; ok {
				return ErrImportChanged
			}
			if _, ok := regs[in.RegCode]; ok {
				return ErrImportChanged
			}
		}
		results, err := checkMany(ctx, tx, list)
		if err != nil {
			return err
		}
		for _, res := range results {
			if !res.Valid {
				return ErrImportChanged
			}
		}
		n := pgutil.NullString
		batch := &pgx.Batch{}
		for _, i := range list {
			batch.Queue(`CALL pms.sp_mst_inst_insert(
			  $1::varchar, $2::varchar, $3::varchar, $4::varchar, $5::varchar, $6::varchar,
			  $7::varchar, $8::varchar, $9::varchar,
			  NULL::integer, NULL::integer, NULL::integer, NULL::integer,
			  $10::date, $11::varchar,
			  $12::varchar, $13::varchar,
			  $14::varchar, $15::varchar, $16::varchar, $17::varchar,
			  $18::varchar, $19::varchar, $20::varchar,
			  $21::varchar, $22::varchar, $23::varchar, $24::varchar)`,
				i.Code, i.RegCode, i.FullCode, i.Name, n(i.Address), n(i.Locality),
				i.PlaceCode, i.BranchCode, i.TrustCode,
				i.RegDate, i.FromFinYr,
				n(i.Email), n(i.Tel),
				n(i.HeadName), n(i.HeadDesig), n(i.HeadMobile), n(i.HeadEmail),
				n(i.AcntName), n(i.AcntMobile), n(i.AcntEmail),
				n(i.LiasonName), n(i.LiasonDesig), n(i.LiasonMobile), n(i.LiasonEmail))
		}
		if err := tx.SendBatch(ctx, batch).Close(); err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "P0001" {
				return domain.Conflict("The database refused a row: " + pgErr.Message + ". Nothing was saved.")
			}
			if _, ok := pgutil.UniqueViolation(err); ok {
				return ErrImportChanged
			}
			return domain.Internal("Could not save the institutions", err)
		}
		return nil
	})
}
