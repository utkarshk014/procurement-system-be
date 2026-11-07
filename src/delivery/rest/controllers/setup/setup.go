package controllers_setup

import (
	"log/slog"
	"net/http"

	rest_utils "github.com/utkarshk014/procurement-system-be/src/delivery/rest/utils"
	"github.com/utkarshk014/procurement-system-be/src/internal/usecase"
)

// SetupController reports where the tenant is in the setup wizard.
type SetupController interface {
	Status(w http.ResponseWriter, r *http.Request) error
}

type setupController struct {
	usecases *usecase.Usecases
	logger   *slog.Logger
}

// NewSetupController builds the setup controller.
func NewSetupController(usecases *usecase.Usecases, logger *slog.Logger) SetupController {
	return &setupController{usecases: usecases, logger: logger}
}

func (sc *setupController) Status(w http.ResponseWriter, r *http.Request) error {
	st, err := sc.usecases.Setup.Status(r.Context())
	if err != nil {
		return err
	}
	rest_utils.Respond(w, http.StatusOK, st)
	return nil
}
