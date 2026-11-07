// Package usecase_common holds helpers shared by the use cases.
package usecase_common

import (
	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
	"github.com/utkarshk014/procurement-system-be/src/packages/validation"
)

// ValidateStruct runs a DTO's validate tags and returns all violations as one error.
func ValidateStruct(v any) error {
	violations := validation.Struct(v)
	if len(violations) == 0 {
		return nil
	}
	return domain.Validation(ToFieldErrors(violations)...)
}

// ToFieldErrors converts validation violations to domain field errors.
func ToFieldErrors(violations []validation.Violation) []domain.FieldError {
	fields := make([]domain.FieldError, 0, len(violations))
	for _, v := range violations {
		fields = append(fields, domain.FieldError{Field: v.Field, Message: v.Message})
	}
	return fields
}
