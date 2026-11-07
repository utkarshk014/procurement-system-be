package rest_utils

import (
	"net"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
	domain_auth "github.com/utkarshk014/procurement-system-be/src/internal/domain/auth"
)

// ClientIP is the verified client IP set by the client-IP middleware, else the TCP peer.
func ClientIP(r *http.Request) string {
	if ip := middleware.GetClientIP(r.Context()); ip != "" {
		return ip
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// ClientMeta returns the caller's verified IP and user agent.
func ClientMeta(r *http.Request) domain_auth.ClientMeta {
	return domain_auth.ClientMeta{IP: ClientIP(r), UserAgent: r.UserAgent()}
}

// IntParam reads a positive integer URL parameter, or returns NotFound.
func IntParam(r *http.Request, name, what string) (int64, error) {
	v, err := strconv.ParseInt(chi.URLParam(r, name), 10, 64)
	if err != nil || v <= 0 {
		return 0, domain.NotFound(what + " not found")
	}
	return v, nil
}
