// Package controllers_stock serves the stock masters under /api/v1/stock.
package controllers_stock

import (
	"context"
	"log/slog"
	"net/http"

	rest_utils "github.com/utkarshk014/procurement-system-be/src/delivery/rest/utils"
	domain_stock "github.com/utkarshk014/procurement-system-be/src/internal/domain/stock"
)

// Controller is the routes every stock master serves.
type Controller interface {
	List(w http.ResponseWriter, r *http.Request) error
	Get(w http.ResponseWriter, r *http.Request) error
	Validate(w http.ResponseWriter, r *http.Request) error
	Create(w http.ResponseWriter, r *http.Request) error
	Update(w http.ResponseWriter, r *http.Request) error
	Delete(w http.ResponseWriter, r *http.Request) error
}

// usecase is the shape all five stock use cases share: In is the body, Out the stored row.
type usecase[In, Out any] interface {
	List(ctx context.Context) ([]Out, error)
	Get(ctx context.Context, id int64) (*Out, error)
	Validate(ctx context.Context, in In) (*Out, error)
	Create(ctx context.Context, in In) (*Out, error)
	Update(ctx context.Context, id int64, in In) (*Out, error)
	Delete(ctx context.Context, id int64) error
}

// validateResponse is a dry-run result: the row as it would be saved.
type validateResponse[T any] struct {
	Valid bool `json:"valid"`
	Entry T    `json:"entry"`
}

type controller[In, Out any] struct {
	what string // "Item group", for the 404 of a bad id
	uc   usecase[In, Out]
}

// NewListController serves item groups, categories or manufacturers.
func NewListController(list domain_stock.List, uc domain_stock.ListUsecase, _ *slog.Logger) Controller {
	return controller[domain_stock.EntryInput, domain_stock.Entry]{what: list.Label, uc: uc}
}

// NewUOMController serves the units of measurement.
func NewUOMController(uc domain_stock.UOMUsecase, _ *slog.Logger) Controller {
	return controller[domain_stock.UOMInput, domain_stock.UOM]{what: "Unit of measurement", uc: uc}
}

// NewItemController serves the item master.
func NewItemController(uc domain_stock.ItemUsecase, _ *slog.Logger) Controller {
	return controller[domain_stock.ItemInput, domain_stock.Item]{what: "Item", uc: uc}
}

func (c controller[In, Out]) List(w http.ResponseWriter, r *http.Request) error {
	rows, err := c.uc.List(r.Context())
	if err != nil {
		return err
	}
	rest_utils.Respond(w, http.StatusOK, rows)
	return nil
}

func (c controller[In, Out]) Get(w http.ResponseWriter, r *http.Request) error {
	id, err := rest_utils.IntParam(r, "id", c.what)
	if err != nil {
		return err
	}
	row, err := c.uc.Get(r.Context(), id)
	if err != nil {
		return err
	}
	rest_utils.Respond(w, http.StatusOK, row)
	return nil
}

// Validate dry-runs a create without writing.
func (c controller[In, Out]) Validate(w http.ResponseWriter, r *http.Request) error {
	var in In
	if err := rest_utils.DecodeJSON(w, r, &in); err != nil {
		return err
	}
	row, err := c.uc.Validate(r.Context(), in)
	if err != nil {
		return err
	}
	rest_utils.Respond(w, http.StatusOK, validateResponse[*Out]{Valid: true, Entry: row})
	return nil
}

// Create adds a row (201) and returns it as stored.
func (c controller[In, Out]) Create(w http.ResponseWriter, r *http.Request) error {
	var in In
	if err := rest_utils.DecodeJSON(w, r, &in); err != nil {
		return err
	}
	row, err := c.uc.Create(r.Context(), in)
	if err != nil {
		return err
	}
	rest_utils.Respond(w, http.StatusCreated, row)
	return nil
}

// Update saves an edit (the whole body, as for create) and returns the row as stored.
func (c controller[In, Out]) Update(w http.ResponseWriter, r *http.Request) error {
	id, err := rest_utils.IntParam(r, "id", c.what)
	if err != nil {
		return err
	}
	var in In
	if err := rest_utils.DecodeJSON(w, r, &in); err != nil {
		return err
	}
	row, err := c.uc.Update(r.Context(), id, in)
	if err != nil {
		return err
	}
	rest_utils.Respond(w, http.StatusOK, row)
	return nil
}

// Delete removes a row (204); the DBA's procedure refuses one that is still used.
func (c controller[In, Out]) Delete(w http.ResponseWriter, r *http.Request) error {
	id, err := rest_utils.IntParam(r, "id", c.what)
	if err != nil {
		return err
	}
	if err := c.uc.Delete(r.Context(), id); err != nil {
		return err
	}
	rest_utils.NoContent(w)
	return nil
}
