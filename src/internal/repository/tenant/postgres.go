package repository_tenant

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
	domain_tenant "github.com/utkarshk014/procurement-system-be/src/internal/domain/tenant"
	"github.com/utkarshk014/procurement-system-be/src/internal/repository/pgutil"
)

type postgresTenantRepository struct {
	db *pgxpool.Pool
}

// NewPostgresTenantRepository stores tenants in the control database.
func NewPostgresTenantRepository(control *pgxpool.Pool) domain_tenant.Repository {
	return &postgresTenantRepository{db: control}
}

const tenantColumns = `id, code, name, db_key, status, created_at, updated_at`

func scanTenant(row interface{ Scan(...any) error }, t *domain_tenant.Tenant) error {
	return row.Scan(&t.ID, &t.Code, &t.Name, &t.DBKey, &t.Status, &t.CreatedAt, &t.UpdatedAt)
}

func (r *postgresTenantRepository) Create(ctx context.Context, t *domain_tenant.Tenant) error {
	err := scanTenant(r.db.QueryRow(ctx, `
		INSERT INTO pms.sys_tenant (code, name, db_key, status)
		VALUES ($1, $2, $3, $4)
		RETURNING `+tenantColumns,
		t.Code, t.Name, t.DBKey, t.Status), t)
	if err != nil {
		if constraint, ok := pgutil.UniqueViolation(err); ok {
			if constraint == "sys_tenant_db_key_uq" {
				return domain.Conflict("Another tenant already uses this database",
					domain.FieldError{Field: "db_key", Message: "is already assigned to another tenant"})
			}
			return domain.Conflict("A tenant with this code already exists",
				domain.FieldError{Field: "code", Message: "is already taken"})
		}
		return domain.Internal("Could not create tenant", err)
	}
	return nil
}

func (r *postgresTenantRepository) List(ctx context.Context) ([]domain_tenant.Tenant, error) {
	rows, err := r.db.Query(ctx, `SELECT `+tenantColumns+` FROM pms.sys_tenant ORDER BY id`)
	if err != nil {
		return nil, domain.Internal("Could not list tenants", err)
	}
	defer rows.Close()

	tenants := []domain_tenant.Tenant{}
	for rows.Next() {
		var t domain_tenant.Tenant
		if err := scanTenant(rows, &t); err != nil {
			return nil, domain.Internal("Could not read tenant", err)
		}
		tenants = append(tenants, t)
	}
	if err := rows.Err(); err != nil {
		return nil, domain.Internal("Could not list tenants", err)
	}
	return tenants, nil
}

func (r *postgresTenantRepository) GetByID(ctx context.Context, id int64) (*domain_tenant.Tenant, error) {
	t := &domain_tenant.Tenant{}
	err := scanTenant(r.db.QueryRow(ctx,
		`SELECT `+tenantColumns+` FROM pms.sys_tenant WHERE id = $1`, id), t)
	if err != nil {
		if pgutil.IsNoRows(err) {
			return nil, domain.NotFound("Tenant not found")
		}
		return nil, domain.Internal("Could not load tenant", err)
	}
	return t, nil
}
