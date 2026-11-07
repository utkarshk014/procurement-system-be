package middlewares

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
)

// RequestLogger logs one line per request with its id, route, status, duration and caller.
func RequestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			slot := &principalSlot{}
			next.ServeHTTP(ww, r.WithContext(context.WithValue(r.Context(), principalSlotKey{}, slot)))

			attrs := []any{
				"request_id", middleware.GetReqID(r.Context()),
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"duration_ms", time.Since(start).Milliseconds(),
			}
			if p := slot.principal; p != nil {
				attrs = append(attrs, "user_id", p.UserID, "tenant", p.TenantCode)
			}
			logger.InfoContext(r.Context(), "http request", attrs...)
		})
	}
}

// principalSlot lets VerifyToken, deeper in the chain, hand the caller back to RequestLogger.
type principalSlot struct{ principal *domain.Principal }

type principalSlotKey struct{}

func recordPrincipal(ctx context.Context, p *domain.Principal) {
	if slot, ok := ctx.Value(principalSlotKey{}).(*principalSlot); ok {
		slot.principal = p
	}
}
