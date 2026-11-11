// Package rest is the HTTP delivery layer: router, middleware chain and server.
package rest

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
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
	"github.com/utkarshk014/procurement-system-be/src/delivery/rest/routers"
	rest_utils "github.com/utkarshk014/procurement-system-be/src/delivery/rest/utils"
	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
	"github.com/utkarshk014/procurement-system-be/src/internal/usecase"
	"github.com/utkarshk014/procurement-system-be/src/utils"
)

// RestDelivery owns the HTTP server.
type RestDelivery struct {
	server *http.Server
	logger *slog.Logger
}

// NewRouter builds the full handler; tests drive it with httptest.
func NewRouter(cfg *utils.Config, usecases *usecase.Usecases, db controllers_meta.Pinger, logger *slog.Logger) http.Handler {
	r := chi.NewRouter()

	r.Use(middlewares.RequestID)
	r.Use(clientIP(cfg.TrustedProxies))
	r.Use(middlewares.RequestLogger(logger))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(cfg.RequestTimeout))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: cfg.CORSAllowedOrigins,
		AllowedMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowedHeaders: []string{"Authorization", "Content-Type", "X-Request-Id"},
		ExposedHeaders: []string{"X-Request-Id"},
		MaxAge:         300,
	}))
	r.Use(securityHeaders)

	auth := middlewares.NewAuthMiddleware(usecases.Auth, logger)

	routers.SetMetaRoutes(r, auth, controllers_meta.NewMetaController(db, logger), logger)
	routers.SetAuthRoutes(r, auth, controllers_auth.NewAuthController(usecases, logger), cfg.LoginRateLimit, logger)
	routers.SetAdminRoutes(r, auth, controllers_admin.NewAdminController(usecases, logger), logger)
	routers.SetSetupRoutes(r, auth, controllers_setup.NewSetupController(usecases, logger), logger)
	routers.SetFinYearRoutes(r, auth, controllers_finyear.NewFinYearController(usecases, logger), logger)
	routers.SetCompanyRoutes(r, auth, controllers_company.NewCompanyController(usecases, logger),
		controllers_taxdetails.NewTaxDetailsController(usecases, logger), logger)
	routers.SetTrustRoutes(r, auth, controllers_trust.NewTrustController(usecases, logger), logger)
	routers.SetCodeListRoute(r, "places", auth, controllers_codelist.NewCodeListController(usecases.Places, logger), logger)
	routers.SetCodeListRoute(r, "branches", auth, controllers_codelist.NewCodeListController(usecases.Branches, logger), logger)
	routers.SetCodeListRoute(r, "regions", auth, controllers_codelist.NewCodeListController(usecases.Regions, logger), logger)
	routers.SetInstitutionRoutes(r, auth, controllers_institution.NewInstitutionController(usecases, logger), logger)

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		rest_utils.WriteError(w, r, logger, domain.NotFound("Route not found"))
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		rest_utils.WriteError(w, r, logger, domain.MethodNotAllowed("Method not allowed"))
	})
	return r
}

// clientIP trusts X-Forwarded-For only when the TCP peer is a listed proxy, so clients cannot spoof their IP.
func clientIP(trustedProxies []string) func(http.Handler) http.Handler {
	if len(trustedProxies) == 0 {
		return middleware.ClientIPFromRemoteAddr
	}
	return middleware.ClientIPFromXFF(trustedProxies...)
}

// NewRestDelivery builds the HTTP server.
func NewRestDelivery(cfg *utils.Config, usecases *usecase.Usecases, db controllers_meta.Pinger, logger *slog.Logger) *RestDelivery {
	return &RestDelivery{
		logger: logger,
		server: &http.Server{
			Addr:              fmt.Sprintf(":%d", cfg.HTTPPort),
			Handler:           NewRouter(cfg, usecases, db, logger),
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       15 * time.Second,
			WriteTimeout:      cfg.RequestTimeout + 5*time.Second,
			IdleTimeout:       60 * time.Second,
		},
	}
}

// Start serves until Shutdown; it returns nil on a clean shutdown.
func (d *RestDelivery) Start() error {
	d.logger.Info("http server listening", "addr", d.server.Addr)
	if err := d.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// Shutdown stops accepting requests and waits for in-flight ones.
func (d *RestDelivery) Shutdown(ctx context.Context) error {
	return d.server.Shutdown(ctx)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
