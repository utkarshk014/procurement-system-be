package controllers_admin

import (
	"log/slog"
	"net/http"

	rest_utils "github.com/utkarshk014/procurement-system-be/src/delivery/rest/utils"
	domain_tenant "github.com/utkarshk014/procurement-system-be/src/internal/domain/tenant"
	"github.com/utkarshk014/procurement-system-be/src/internal/usecase"
)

// AdminController serves platform-admin tenant onboarding.
type AdminController interface {
	CreateTenant(w http.ResponseWriter, r *http.Request) error
	ListTenants(w http.ResponseWriter, r *http.Request) error
	CreateTenantAdmin(w http.ResponseWriter, r *http.Request) error
}

type adminController struct {
	usecases *usecase.Usecases
	logger   *slog.Logger
}

// NewAdminController builds the admin controller.
func NewAdminController(usecases *usecase.Usecases, logger *slog.Logger) AdminController {
	return &adminController{usecases: usecases, logger: logger}
}

func (ac *adminController) CreateTenant(w http.ResponseWriter, r *http.Request) error {
	var in domain_tenant.CreateTenantInput
	if err := rest_utils.DecodeJSON(w, r, &in); err != nil {
		return err
	}
	t, err := ac.usecases.Tenant.Create(r.Context(), in)
	if err != nil {
		return err
	}
	rest_utils.Respond(w, http.StatusCreated, t)
	return nil
}

func (ac *adminController) ListTenants(w http.ResponseWriter, r *http.Request) error {
	tenants, err := ac.usecases.Tenant.List(r.Context())
	if err != nil {
		return err
	}
	rest_utils.Respond(w, http.StatusOK, tenants)
	return nil
}

func (ac *adminController) CreateTenantAdmin(w http.ResponseWriter, r *http.Request) error {
	tenantID, err := rest_utils.IntParam(r, "tenantID", "Tenant")
	if err != nil {
		return err
	}
	var in domain_tenant.CreateTenantAdminInput
	if err := rest_utils.DecodeJSON(w, r, &in); err != nil {
		return err
	}
	user, err := ac.usecases.Tenant.CreateAdmin(r.Context(), tenantID, in)
	if err != nil {
		return err
	}
	rest_utils.Respond(w, http.StatusCreated, user)
	return nil
}
