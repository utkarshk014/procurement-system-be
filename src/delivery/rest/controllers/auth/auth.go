package controllers_auth

import (
	"log/slog"
	"net/http"

	rest_utils "github.com/utkarshk014/procurement-system-be/src/delivery/rest/utils"
	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
	domain_auth "github.com/utkarshk014/procurement-system-be/src/internal/domain/auth"
	"github.com/utkarshk014/procurement-system-be/src/internal/usecase"
)

// AuthController serves login, token refresh, logout and "who am I".
type AuthController interface {
	Login(w http.ResponseWriter, r *http.Request) error
	Refresh(w http.ResponseWriter, r *http.Request) error
	Logout(w http.ResponseWriter, r *http.Request) error
	Me(w http.ResponseWriter, r *http.Request) error
}

type authController struct {
	usecases *usecase.Usecases
	logger   *slog.Logger
}

// NewAuthController builds the auth controller.
func NewAuthController(usecases *usecase.Usecases, logger *slog.Logger) AuthController {
	return &authController{usecases: usecases, logger: logger}
}

// Login checks email and password and returns an access/refresh token pair.
func (ac *authController) Login(w http.ResponseWriter, r *http.Request) error {
	var in domain_auth.LoginInput
	if err := rest_utils.DecodeJSON(w, r, &in); err != nil {
		return err
	}
	pair, err := ac.usecases.Auth.Login(r.Context(), in, rest_utils.ClientMeta(r))
	if err != nil {
		return err
	}
	rest_utils.Respond(w, http.StatusOK, pair)
	return nil
}

// Refresh swaps a refresh token for a new token pair.
func (ac *authController) Refresh(w http.ResponseWriter, r *http.Request) error {
	var in domain_auth.RefreshInput
	if err := rest_utils.DecodeJSON(w, r, &in); err != nil {
		return err
	}
	pair, err := ac.usecases.Auth.Refresh(r.Context(), in, rest_utils.ClientMeta(r))
	if err != nil {
		return err
	}
	rest_utils.Respond(w, http.StatusOK, pair)
	return nil
}

// Logout revokes the refresh token. The access token simply expires.
func (ac *authController) Logout(w http.ResponseWriter, r *http.Request) error {
	var in domain_auth.RefreshInput
	if err := rest_utils.DecodeJSON(w, r, &in); err != nil {
		return err
	}
	if err := ac.usecases.Auth.Logout(r.Context(), in); err != nil {
		return err
	}
	rest_utils.NoContent(w)
	return nil
}

// Me returns the logged-in user.
func (ac *authController) Me(w http.ResponseWriter, r *http.Request) error {
	p := domain.PrincipalFrom(r.Context())
	if p == nil {
		return domain.Unauthorized("Please log in")
	}
	rest_utils.Respond(w, http.StatusOK, domain_auth.MeFromPrincipal(p))
	return nil
}
