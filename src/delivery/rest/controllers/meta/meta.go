package controllers_meta

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	rest_utils "github.com/utkarshk014/procurement-system-be/src/delivery/rest/utils"
	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
	"github.com/utkarshk014/procurement-system-be/src/packages/validation"
)

// Pinger checks a dependency is reachable.
type Pinger interface {
	Ping(ctx context.Context) error
}

// MetaController serves health checks and form reference data.
type MetaController interface {
	Health(w http.ResponseWriter, r *http.Request) error
	IndianStates(w http.ResponseWriter, r *http.Request) error
}

type metaController struct {
	db     Pinger
	logger *slog.Logger
}

// NewMetaController builds the meta controller.
func NewMetaController(db Pinger, logger *slog.Logger) MetaController {
	return &metaController{db: db, logger: logger}
}

// Health reports whether the API and its control database are up.
func (mc *metaController) Health(w http.ResponseWriter, r *http.Request) error {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	if err := mc.db.Ping(ctx); err != nil {
		return domain.Internal("Database is unreachable", err)
	}
	rest_utils.Respond(w, http.StatusOK, map[string]string{"status": "ok"})
	return nil
}

// IndianStates lists states and union territories for the state dropdown.
func (mc *metaController) IndianStates(w http.ResponseWriter, r *http.Request) error {
	rest_utils.Respond(w, http.StatusOK, validation.IndianStates)
	return nil
}
