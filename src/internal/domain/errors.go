// Package domain holds types every layer shares (errors, principal, roles) and imports nothing else.
package domain

import (
	"errors"
	"fmt"
)

// ErrorKind classifies an error; the REST layer maps each kind to a status code.
type ErrorKind string

const (
	KindInvalidParams ErrorKind = "invalid_params"
	KindValidation    ErrorKind = "validation_failed"
	KindUnauthorized  ErrorKind = "unauthorized"
	KindForbidden     ErrorKind = "forbidden"
	KindNotFound      ErrorKind = "not_found"
	KindConflict      ErrorKind = "conflict"
	KindRateLimited   ErrorKind = "rate_limited"
	KindMethod        ErrorKind = "method_not_allowed"
	KindUnavailable   ErrorKind = "service_unavailable"
	KindTimeout       ErrorKind = "timeout"
	KindInternal      ErrorKind = "internal_error"
)

// FieldError is one problem with one input field; an empty Field means the whole request.
type FieldError struct {
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

// AppError is what use cases return: Message is shown to callers, Err is only logged.
type AppError struct {
	Kind    ErrorKind
	Message string
	Fields  []FieldError
	Err     error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Kind, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Kind, e.Message)
}

func (e *AppError) Unwrap() error { return e.Err }

// AsAppError unwraps err into an *AppError if it is one.
func AsAppError(err error) (*AppError, bool) {
	var ae *AppError
	ok := errors.As(err, &ae)
	return ae, ok
}

// IsKind reports whether err is an AppError of the given kind.
func IsKind(err error, kind ErrorKind) bool {
	ae, ok := AsAppError(err)
	return ok && ae.Kind == kind
}

func InvalidParams(msg string) error {
	return &AppError{Kind: KindInvalidParams, Message: msg}
}

func Validation(fields ...FieldError) error {
	return &AppError{Kind: KindValidation, Message: "Some fields need attention", Fields: fields}
}

func Unauthorized(msg string) error {
	return &AppError{Kind: KindUnauthorized, Message: msg}
}

func Forbidden(msg string) error {
	return &AppError{Kind: KindForbidden, Message: msg}
}

func NotFound(msg string) error {
	return &AppError{Kind: KindNotFound, Message: msg}
}

func Conflict(msg string, fields ...FieldError) error {
	return &AppError{Kind: KindConflict, Message: msg, Fields: fields}
}

func RateLimited(msg string) error {
	return &AppError{Kind: KindRateLimited, Message: msg}
}

func MethodNotAllowed(msg string) error {
	return &AppError{Kind: KindMethod, Message: msg}
}

// Internal wraps an unexpected failure. The caller only ever sees msg.
func Internal(msg string, err error) error {
	return &AppError{Kind: KindInternal, Message: msg, Err: err}
}
