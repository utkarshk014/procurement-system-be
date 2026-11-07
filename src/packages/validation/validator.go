package validation

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

// Violation is one failed rule on one field. Field uses the JSON name.
type Violation struct {
	Field   string
	Message string
}

var structValidator = newValidator()

func newValidator() *validator.Validate {
	v := validator.New(validator.WithRequiredStructEnabled())
	v.RegisterTagNameFunc(func(f reflect.StructField) string {
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name == "-" {
			return ""
		}
		return name
	})
	register := func(tag string, fn func(string) bool) {
		_ = v.RegisterValidation(tag, func(fl validator.FieldLevel) bool {
			s := fl.Field().String()
			return s == "" || fn(s) // emptiness is the job of "required"
		})
	}
	register("in_mobile", IsIndianMobile)
	register("in_pin", IsIndianPIN)
	register("in_pan", IsPAN)
	register("in_gstin", IsGSTIN)
	register("db_email", IsEmail)
	register("website", IsWebsite)
	return v
}

// Struct runs the `validate` tags on s and returns every violation, in field order.
func Struct(s any) []Violation {
	err := structValidator.Struct(s)
	if err == nil {
		return nil
	}
	var verrs validator.ValidationErrors
	if !errors.As(err, &verrs) {
		return []Violation{{Message: err.Error()}}
	}
	out := make([]Violation, 0, len(verrs))
	for _, fe := range verrs {
		out = append(out, Violation{Field: fe.Field(), Message: message(fe)})
	}
	return out
}

func message(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "is required"
	case "max":
		return fmt.Sprintf("must be at most %s characters", fe.Param())
	case "min":
		return fmt.Sprintf("must be at least %s characters", fe.Param())
	case "len":
		return fmt.Sprintf("must be exactly %s characters", fe.Param())
	case "email", "db_email":
		return "must be a valid email address"
	case "in_mobile":
		return "must be a 10-digit Indian mobile number"
	case "in_pin":
		return "must be a 6-digit PIN code"
	case "in_pan":
		return "must be a valid PAN, e.g. ABCDE1234F"
	case "in_gstin":
		return "must be a valid 15-character GSTIN"
	case "website":
		return "must be a valid website, e.g. example.com"
	case "datetime":
		return "must be a date in YYYY-MM-DD format"
	case "oneof":
		return "must be one of: " + strings.ReplaceAll(fe.Param(), " ", ", ")
	case "alphanum":
		return "may only contain letters and numbers"
	default:
		return "is invalid"
	}
}
