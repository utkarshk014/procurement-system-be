package routers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/utkarshk014/procurement-system-be/src/delivery/rest/middlewares"
	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
	domain_auth "github.com/utkarshk014/procurement-system-be/src/internal/domain/auth"
)

type stubAuthController struct{}

func (stubAuthController) Login(w http.ResponseWriter, _ *http.Request) error {
	w.WriteHeader(http.StatusOK)
	return nil
}
func (stubAuthController) Refresh(w http.ResponseWriter, _ *http.Request) error {
	w.WriteHeader(http.StatusOK)
	return nil
}
func (stubAuthController) Logout(w http.ResponseWriter, _ *http.Request) error { return nil }
func (stubAuthController) Me(w http.ResponseWriter, _ *http.Request) error     { return nil }

type stubAuthUsecase struct{ domain_auth.Usecase }

func (stubAuthUsecase) Authenticate(context.Context, string) (*domain.Principal, error) {
	return nil, domain.Unauthorized("no")
}

func TestLoginRateLimitIgnoresSpoofedIPsAndRefreshIsNotLimited(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	r := chi.NewRouter()
	SetAuthRoutes(r, middlewares.NewAuthMiddleware(stubAuthUsecase{}, logger), stubAuthController{}, 2, logger)

	n := 0
	post := func(path string) *httptest.ResponseRecorder {
		n++
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader("{}"))
		req.RemoteAddr = "203.0.113.7:5000"
		// A fresh spoofed client IP on every request must not buy a fresh limit bucket.
		req.Header.Set("True-Client-IP", fmt.Sprintf("198.51.100.%d", n))
		req.Header.Set("X-Forwarded-For", fmt.Sprintf("192.0.2.%d", n))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}

	assert.Equal(t, 200, post("/api/v1/auth/login").Code)
	assert.Equal(t, 200, post("/api/v1/auth/login").Code)
	limited := post("/api/v1/auth/login")
	require.Equal(t, http.StatusTooManyRequests, limited.Code)
	var body struct {
		Error struct{ Code string } `json:"error"`
	}
	require.NoError(t, json.Unmarshal(limited.Body.Bytes(), &body))
	assert.Equal(t, "rate_limited", body.Error.Code)

	for i := 0; i < 5; i++ {
		assert.Equal(t, 200, post("/api/v1/auth/refresh").Code, "refresh shares no limit with login")
	}
}
