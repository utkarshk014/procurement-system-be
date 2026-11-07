package rest_utils

import (
	"net"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
	domain_auth "github.com/utkarshk014/procurement-system-be/src/internal/domain/auth"
)

// ClientMeta returns the caller's IP (proxy-aware when TRUST_PROXY is on) and user agent.
func ClientMeta(r *http.Request) domain_auth.ClientMeta {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	return domain_auth.ClientMeta{IP: ip, UserAgent: r.UserAgent()}
}

// IntParam reads a positive integer URL parameter, or returns NotFound.
func IntParam(r *http.Request, name, what string) (int64, error) {
	v, err := strconv.ParseInt(chi.URLParam(r, name), 10, 64)
	if err != nil || v <= 0 {
		return 0, domain.NotFound(what + " not found")
	}
	return v, nil
}
