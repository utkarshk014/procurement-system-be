// Package controllers_stock serves the stock masters under /api/v1/stock.
package controllers_stock

import (
	"log/slog"
	"net/http"

	rest_utils "github.com/utkarshk014/procurement-system-be/src/delivery/rest/utils"
	domain_stock "github.com/utkarshk014/procurement-system-be/src/internal/domain/stock"
)

// Controller is the routes one stock master serves. Delete answers 405 where there is none.
type Controller interface {
	List(w http.ResponseWriter, r *http.Request) error
	Get(w http.ResponseWriter, r *http.Request) error
	Validate(w http.ResponseWriter, r *http.Request) error
	Create(w http.ResponseWriter, r *http.Request) error
}

// ItemController adds delete.
type ItemController interface {
	Controller
	Delete(w http.ResponseWriter, r *http.Request) error
}

// validateResponse is a dry-run result: the row as it would be saved.
type validateResponse[T any] struct {
	Valid bool `json:"valid"`
	Entry T    `json:"entry"`
}

// crud is the shared shape: list, get by id, validate and create with one input type.
type crud[In any, Out any] struct {
	what     string // "Item group", for the 404 of a bad id
	list     func(r *http.Request) ([]Out, error)
	get      func(r *http.Request, id int64) (*Out, error)
	validate func(r *http.Request, in In) (*Out, error)
	create   func(r *http.Request, in In) (*Out, error)
}

func (c crud[In, Out]) List(w http.ResponseWriter, r *http.Request) error {
	rows, err := c.list(r)
	if err != nil {
		return err
	}
	rest_utils.Respond(w, http.StatusOK, rows)
	return nil
}

func (c crud[In, Out]) Get(w http.ResponseWriter, r *http.Request) error {
	id, err := rest_utils.IntParam(r, "id", c.what)
	if err != nil {
		return err
	}
	row, err := c.get(r, id)
	if err != nil {
		return err
	}
	rest_utils.Respond(w, http.StatusOK, row)
	return nil
}

func (c crud[In, Out]) Validate(w http.ResponseWriter, r *http.Request) error {
	var in In
	if err := rest_utils.DecodeJSON(w, r, &in); err != nil {
		return err
	}
	row, err := c.validate(r, in)
	if err != nil {
		return err
	}
	rest_utils.Respond(w, http.StatusOK, validateResponse[*Out]{Valid: true, Entry: row})
	return nil
}

func (c crud[In, Out]) Create(w http.ResponseWriter, r *http.Request) error {
	var in In
	if err := rest_utils.DecodeJSON(w, r, &in); err != nil {
		return err
	}
	row, err := c.create(r, in)
	if err != nil {
		return err
	}
	rest_utils.Respond(w, http.StatusCreated, row)
	return nil
}

// NewListController serves item groups, categories or manufacturers.
func NewListController(list domain_stock.List, uc domain_stock.ListUsecase, _ *slog.Logger) Controller {
	return crud[domain_stock.EntryInput, domain_stock.Entry]{
		what: list.Label,
		list: func(r *http.Request) ([]domain_stock.Entry, error) { return uc.List(r.Context()) },
		get:  func(r *http.Request, id int64) (*domain_stock.Entry, error) { return uc.Get(r.Context(), id) },
		validate: func(r *http.Request, in domain_stock.EntryInput) (*domain_stock.Entry, error) {
			return uc.Validate(r.Context(), in)
		},
		create: func(r *http.Request, in domain_stock.EntryInput) (*domain_stock.Entry, error) {
			return uc.Create(r.Context(), in)
		},
	}
}

// NewUOMController serves the units of measurement.
func NewUOMController(uc domain_stock.UOMUsecase, _ *slog.Logger) Controller {
	return crud[domain_stock.UOMInput, domain_stock.UOM]{
		what: "Unit of measurement",
		list: func(r *http.Request) ([]domain_stock.UOM, error) { return uc.List(r.Context()) },
		get:  func(r *http.Request, id int64) (*domain_stock.UOM, error) { return uc.Get(r.Context(), id) },
		validate: func(r *http.Request, in domain_stock.UOMInput) (*domain_stock.UOM, error) {
			return uc.Validate(r.Context(), in)
		},
		create: func(r *http.Request, in domain_stock.UOMInput) (*domain_stock.UOM, error) {
			return uc.Create(r.Context(), in)
		},
	}
}

type itemController struct {
	crud[domain_stock.ItemInput, domain_stock.Item]
	uc domain_stock.ItemUsecase
}

// NewItemController serves the item master.
func NewItemController(uc domain_stock.ItemUsecase, _ *slog.Logger) ItemController {
	return itemController{
		crud: crud[domain_stock.ItemInput, domain_stock.Item]{
			what: "Item",
			list: func(r *http.Request) ([]domain_stock.Item, error) { return uc.List(r.Context()) },
			get:  func(r *http.Request, id int64) (*domain_stock.Item, error) { return uc.Get(r.Context(), id) },
			validate: func(r *http.Request, in domain_stock.ItemInput) (*domain_stock.Item, error) {
				return uc.Validate(r.Context(), in)
			},
			create: func(r *http.Request, in domain_stock.ItemInput) (*domain_stock.Item, error) {
				return uc.Create(r.Context(), in)
			},
		},
		uc: uc,
	}
}

// Delete removes an item (204).
func (c itemController) Delete(w http.ResponseWriter, r *http.Request) error {
	id, err := rest_utils.IntParam(r, "id", "Item")
	if err != nil {
		return err
	}
	if err := c.uc.Delete(r.Context(), id); err != nil {
		return err
	}
	rest_utils.NoContent(w)
	return nil
}
