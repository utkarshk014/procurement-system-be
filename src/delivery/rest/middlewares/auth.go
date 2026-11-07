package middlewares

import (
	"log/slog"
	"net/http"
	"strings"

	rest_utils "github.com/utkarshk014/procurement-system-be/src/delivery/rest/utils"
	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
	domain_auth "github.com/utkarshk014/procurement-system-be/src/internal/domain/auth"
)

// AuthMiddleware guards routes with the access token.
type AuthMiddleware interface {
	// VerifyToken requires "Authorization: Bearer <access token>" and puts the caller in the context.
	VerifyToken(next http.Handler) http.Handler
	// RequireRole lets the request through only for the given roles; use after VerifyToken.
	RequireRole(roles ...domain.Role) func(http.Handler) http.Handler
}

type authMiddleware struct {
	authUsecase domain_auth.Usecase
	logger      *slog.Logger
}

// NewAuthMiddleware builds the auth middleware.
func NewAuthMiddleware(authUsecase domain_auth.Usecase, logger *slog.Logger) AuthMiddleware {
	return &authMiddleware{authUsecase: authUsecase, logger: logger}
}

func (am *authMiddleware) VerifyToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scheme, token, found := strings.Cut(r.Header.Get("Authorization"), " ")
		token = strings.TrimSpace(token)
		if !found || !strings.EqualFold(scheme, "Bearer") || token == "" {
			rest_utils.WriteError(w, r, am.logger, domain.Unauthorized("Please log in"))
			return
		}
		principal, err := am.authUsecase.Authenticate(r.Context(), token)
		if err != nil {
			rest_utils.WriteError(w, r, am.logger, err)
			return
		}
		recordPrincipal(r.Context(), principal)
		next.ServeHTTP(w, r.WithContext(domain.WithPrincipal(r.Context(), principal)))
	})
}

func (am *authMiddleware) RequireRole(roles ...domain.Role) func(http.Handler) http.Handler {
	allowed := map[domain.Role]bool{}
	for _, role := range roles {
		allowed[role] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p := domain.PrincipalFrom(r.Context())
			if p == nil {
				rest_utils.WriteError(w, r, am.logger, domain.Unauthorized("Please log in"))
				return
			}
			if !allowed[p.Role] {
				rest_utils.WriteError(w, r, am.logger, domain.Forbidden("You do not have access to this"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
