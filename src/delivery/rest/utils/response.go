// Package rest_utils is shared HTTP plumbing: JSON in/out and the one place errors become status codes.
package rest_utils

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
)

const maxBodyBytes = 1 << 20 // 1 MiB

// ErrorBody is the JSON shape of every error response.
type ErrorBody struct {
	Code      domain.ErrorKind    `json:"code"`
	Message   string              `json:"message"`
	Fields    []domain.FieldError `json:"fields,omitempty"`
	RequestID string              `json:"request_id,omitempty"`
}

// HandlerFunc is an http handler that returns an error instead of writing it.
type HandlerFunc func(w http.ResponseWriter, r *http.Request) error

// Wrap adapts a HandlerFunc and writes any returned error with WriteError.
func Wrap(logger *slog.Logger, h HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := h(w, r); err != nil {
			WriteError(w, r, logger, err)
		}
	}
}

// Respond writes {"data": data} with the given status.
func Respond(w http.ResponseWriter, status int, data any) {
	writeJSON(w, status, map[string]any{"data": data})
}

// NoContent writes a 204.
func NoContent(w http.ResponseWriter) { w.WriteHeader(http.StatusNoContent) }

var statusByKind = map[domain.ErrorKind]int{
	domain.KindInvalidParams: http.StatusBadRequest,
	domain.KindValidation:    http.StatusUnprocessableEntity,
	domain.KindUnauthorized:  http.StatusUnauthorized,
	domain.KindForbidden:     http.StatusForbidden,
	domain.KindNotFound:      http.StatusNotFound,
	domain.KindConflict:      http.StatusConflict,
	domain.KindRateLimited:   http.StatusTooManyRequests,
	domain.KindMethod:        http.StatusMethodNotAllowed,
	domain.KindInternal:      http.StatusInternalServerError,
	domain.KindUnavailable:   http.StatusServiceUnavailable,
	domain.KindTimeout:       http.StatusGatewayTimeout,
}

// serverErrorMessage is the only text a caller sees for each 5xx kind.
var serverErrorMessage = map[domain.ErrorKind]string{
	domain.KindInternal:    "Something went wrong, please try again",
	domain.KindUnavailable: "Service is temporarily unavailable, please try again",
	domain.KindTimeout:     "The request took too long, please try again",
}

// WriteError writes {"error": …}; 5xx causes are logged and hidden from the caller.
func WriteError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, err error) {
	reqID := middleware.GetReqID(r.Context())
	ae, ok := domain.AsAppError(err)
	if !ok {
		ae = &domain.AppError{Kind: domain.KindInternal, Err: err}
	}
	kind := ae.Kind
	if kind == domain.KindInternal {
		kind = classifyServerError(err)
	}
	status, known := statusByKind[kind]
	if !known {
		status = http.StatusInternalServerError
	}

	body := ErrorBody{Code: kind, Message: ae.Message, Fields: ae.Fields, RequestID: reqID}
	if status >= 500 {
		logger.ErrorContext(r.Context(), "request failed",
			"request_id", reqID, "method", r.Method, "path", r.URL.Path, "status", status, "error", err)
		body = ErrorBody{Code: kind, Message: serverErrorMessage[kind], RequestID: reqID}
	}
	if status == http.StatusUnauthorized {
		w.Header().Set("WWW-Authenticate", `Bearer realm="pms-api"`)
	}
	writeJSON(w, status, map[string]any{"error": body})
}

// classifyServerError tells a timeout (504) and an unreachable dependency (503) apart from a bug (500).
func classifyServerError(err error) domain.ErrorKind {
	var netErr *net.OpError
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return domain.KindTimeout
	case errors.As(err, &netErr):
		return domain.KindUnavailable
	default:
		return domain.KindInternal
	}
}

// DecodeJSON strictly decodes one JSON object (no unknown fields, max 1 MiB) into dst.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	if mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mediaType != "application/json" {
		return domain.InvalidParams("Content-Type must be application/json")
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		var syntaxErr *json.SyntaxError
		var typeErr *json.UnmarshalTypeError
		switch {
		case errors.As(err, &maxErr):
			return domain.InvalidParams("Request body is too large")
		case errors.Is(err, io.EOF):
			return domain.InvalidParams("Request body is empty")
		case errors.As(err, &syntaxErr), errors.Is(err, io.ErrUnexpectedEOF):
			return domain.InvalidParams("Request body is not valid JSON")
		case errors.As(err, &typeErr):
			return domain.Validation(domain.FieldError{Field: typeErr.Field, Message: "has the wrong type"})
		case strings.HasPrefix(err.Error(), "json: unknown field "):
			field := strings.Trim(strings.TrimPrefix(err.Error(), "json: unknown field "), `"`)
			return domain.Validation(domain.FieldError{Field: field, Message: "is not an accepted field"})
		default:
			return domain.InvalidParams("Request body could not be read")
		}
	}
	// Anything after the object, even a stray "}", makes the body invalid.
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return domain.InvalidParams("Request body must contain a single JSON object")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
