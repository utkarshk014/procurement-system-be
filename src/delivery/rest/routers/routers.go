// Package routers declares every route and the guards in front of it.
package routers

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httprate"
	controllers_admin "github.com/utkarshk014/procurement-system-be/src/delivery/rest/controllers/admin"
	controllers_auth "github.com/utkarshk014/procurement-system-be/src/delivery/rest/controllers/auth"
	controllers_codelist "github.com/utkarshk014/procurement-system-be/src/delivery/rest/controllers/codelist"
	controllers_company "github.com/utkarshk014/procurement-system-be/src/delivery/rest/controllers/company"
	controllers_finyear "github.com/utkarshk014/procurement-system-be/src/delivery/rest/controllers/finyear"
	controllers_institution "github.com/utkarshk014/procurement-system-be/src/delivery/rest/controllers/institution"
	controllers_meta "github.com/utkarshk014/procurement-system-be/src/delivery/rest/controllers/meta"
	controllers_setup "github.com/utkarshk014/procurement-system-be/src/delivery/rest/controllers/setup"
	controllers_taxdetails "github.com/utkarshk014/procurement-system-be/src/delivery/rest/controllers/taxdetails"
	controllers_trust "github.com/utkarshk014/procurement-system-be/src/delivery/rest/controllers/trust"
	"github.com/utkarshk014/procurement-system-be/src/delivery/rest/middlewares"
	rest_utils "github.com/utkarshk014/procurement-system-be/src/delivery/rest/utils"
	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
)

// SetMetaRoutes: public health check, and reference data for logged-in users.
func SetMetaRoutes(r chi.Router, auth middlewares.AuthMiddleware, c controllers_meta.MetaController, logger *slog.Logger) {
	r.Get("/healthz", rest_utils.Wrap(logger, c.Health))
	r.Route("/api/v1/meta", func(r chi.Router) {
		r.Use(auth.VerifyToken)
		r.Get("/indian-states", rest_utils.Wrap(logger, c.IndianStates))
		r.Get("/gst-states", rest_utils.Wrap(logger, c.GSTStates))
	})
}

// SetAuthRoutes: login is public and rate limited per IP; refresh and logout need a refresh token; /me an access token.
func SetAuthRoutes(r chi.Router, auth middlewares.AuthMiddleware, c controllers_auth.AuthController, loginPerMinute int, logger *slog.Logger) {
	r.Route("/api/v1/auth", func(r chi.Router) {
		loginLimit := httprate.Limit(loginPerMinute, time.Minute,
			httprate.WithKeyFuncs(func(r *http.Request) (string, error) { return rest_utils.ClientIP(r), nil }),
			httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
				rest_utils.WriteError(w, r, logger, domain.RateLimited("Too many login attempts, try again in a minute"))
			}))
		r.With(loginLimit).Post("/login", rest_utils.Wrap(logger, c.Login))
		r.Post("/refresh", rest_utils.Wrap(logger, c.Refresh))
		r.Post("/logout", rest_utils.Wrap(logger, c.Logout))
		r.With(auth.VerifyToken).Get("/me", rest_utils.Wrap(logger, c.Me))
	})
}

// SetAdminRoutes: platform admins only.
func SetAdminRoutes(r chi.Router, auth middlewares.AuthMiddleware, c controllers_admin.AdminController, logger *slog.Logger) {
	r.Route("/api/v1/admin", func(r chi.Router) {
		r.Use(auth.VerifyToken, auth.RequireRole(domain.RolePlatformAdmin))
		r.Post("/tenants", rest_utils.Wrap(logger, c.CreateTenant))
		r.Get("/tenants", rest_utils.Wrap(logger, c.ListTenants))
		r.Post("/tenants/{tenantID}/admins", rest_utils.Wrap(logger, c.CreateTenantAdmin))
	})
}

// SetSetupRoutes: wizard progress, tenant admins only.
func SetSetupRoutes(r chi.Router, auth middlewares.AuthMiddleware, c controllers_setup.SetupController, logger *slog.Logger) {
	r.Route("/api/v1/setup", func(r chi.Router) {
		r.Use(auth.VerifyToken, auth.RequireRole(domain.RoleTenantAdmin))
		r.Get("/status", rest_utils.Wrap(logger, c.Status))
	})
}

// SetFinYearRoutes: wizard step 1, tenant admins only.
func SetFinYearRoutes(r chi.Router, auth middlewares.AuthMiddleware, c controllers_finyear.FinYearController, logger *slog.Logger) {
	r.Route("/api/v1/financial-years", func(r chi.Router) {
		r.Use(auth.VerifyToken, auth.RequireRole(domain.RoleTenantAdmin))
		r.Get("/", rest_utils.Wrap(logger, c.List))
		r.Get("/options", rest_utils.Wrap(logger, c.Options))
		r.Get("/current", rest_utils.Wrap(logger, c.Current))
		r.Put("/current", rest_utils.Wrap(logger, c.SetCurrent))
		r.Delete("/{finyr}", rest_utils.Wrap(logger, c.Delete))
		r.Post("/validate", rest_utils.Wrap(logger, c.Validate))
		r.Post("/", rest_utils.Wrap(logger, c.Create))
	})
}

// SetCompanyRoutes: the single company record (wizard step 2) and its tax and contact details (Form 3), tenant admins only.
func SetCompanyRoutes(r chi.Router, auth middlewares.AuthMiddleware, c controllers_company.CompanyController, tax controllers_taxdetails.TaxDetailsController, logger *slog.Logger) {
	r.Route("/api/v1/company", func(r chi.Router) {
		r.Use(auth.VerifyToken, auth.RequireRole(domain.RoleTenantAdmin))
		r.Get("/", rest_utils.Wrap(logger, c.Get))
		r.Put("/", rest_utils.Wrap(logger, c.Save))
		r.Post("/validate", rest_utils.Wrap(logger, c.Validate))
		r.Get("/tax-details", rest_utils.Wrap(logger, tax.Get))
		r.Put("/tax-details", rest_utils.Wrap(logger, tax.Save))
		r.Post("/tax-details/validate", rest_utils.Wrap(logger, tax.Validate))
	})
}

// SetTrustRoutes: the trust master (Form 4), tenant admins only. A trust code never changes; a trust
// can be deleted while no institution uses it.
func SetTrustRoutes(r chi.Router, auth middlewares.AuthMiddleware, c controllers_trust.TrustController, logger *slog.Logger) {
	r.Route("/api/v1/trusts", func(r chi.Router) {
		r.Use(auth.VerifyToken, auth.RequireRole(domain.RoleTenantAdmin))
		r.Get("/", rest_utils.Wrap(logger, c.List))
		r.Post("/", rest_utils.Wrap(logger, c.Create))
		r.Post("/validate", rest_utils.Wrap(logger, c.Validate))
		r.Get("/{code}", rest_utils.Wrap(logger, c.Get))
		r.Put("/{code}", rest_utils.Wrap(logger, c.Update))
		r.Delete("/{code}", rest_utils.Wrap(logger, c.Delete))
	})
}

// SetCodeListRoute: one code list (/places, /branches, /regions, /item-groups, /item-categories,
// /manufacturers), tenant admins only.
// There is no update: the DBA has given insert and delete procedures only.
func SetCodeListRoute(r chi.Router, path string, auth middlewares.AuthMiddleware, c controllers_codelist.CodeListController, logger *slog.Logger) {
	r.Route("/api/v1/"+path, func(r chi.Router) {
		r.Use(auth.VerifyToken, auth.RequireRole(domain.RoleTenantAdmin))
		r.Get("/", rest_utils.Wrap(logger, c.List))
		r.Post("/", rest_utils.Wrap(logger, c.Create))
		r.Post("/validate", rest_utils.Wrap(logger, c.Validate))
		r.Get("/{code}", rest_utils.Wrap(logger, c.Get))
		r.Delete("/{code}", rest_utils.Wrap(logger, c.Delete))
	})
}

// SetInstitutionRoutes: the institution master (Form 5), tenant admins only, including the .xlsx
// import. No update or delete yet.
func SetInstitutionRoutes(r chi.Router, auth middlewares.AuthMiddleware, c controllers_institution.InstitutionController, logger *slog.Logger) {
	r.Route("/api/v1/institutions", func(r chi.Router) {
		r.Use(auth.VerifyToken, auth.RequireRole(domain.RoleTenantAdmin))
		r.Get("/", rest_utils.Wrap(logger, c.List))
		r.Post("/", rest_utils.Wrap(logger, c.Create))
		r.Post("/validate", rest_utils.Wrap(logger, c.Validate))
		r.Get("/next-code", rest_utils.Wrap(logger, c.NextCodes))
		r.Get("/import/template", rest_utils.Wrap(logger, c.ImportTemplate))
		r.Post("/import", rest_utils.Wrap(logger, c.Import))
		r.Get("/{code}", rest_utils.Wrap(logger, c.Get))
	})
}
