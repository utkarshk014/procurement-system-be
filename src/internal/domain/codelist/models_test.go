package domain_codelist

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
)

func TestFunctionErrorMapsUnclesMessagesToFields(t *testing.T) {
	cases := []struct {
		kind    Kind
		message string
		field   string
		kindErr domain.ErrorKind
	}{
		{Place, "Place code must contain exactly 3 characters", "code", domain.KindValidation},
		{Place, "Place code MYS already exists", "code", domain.KindConflict},
		{Branch, "Branch name already exists", "name", domain.KindConflict},
		{Region, "Region name cannot be empty", "name", domain.KindValidation},
		{Region, "Something new", "", domain.KindValidation},
	}
	for _, c := range cases {
		err := c.kind.FunctionError(c.message)
		app, ok := domain.AsAppError(err)
		if assert.True(t, ok, c.message) {
			assert.Equal(t, c.kindErr, app.Kind, c.message)
			assert.Equal(t, c.field, app.Fields[0].Field, c.message)
			assert.Equal(t, c.message, app.Fields[0].Message)
		}
	}
}

func TestCodeRe(t *testing.T) {
	for _, ok := range []string{"MYS", "BGN"} {
		assert.True(t, CodeRe.MatchString(ok), ok)
	}
	for _, bad := range []string{"1 A", "MY", "MYSO", "my1", ""} {
		assert.False(t, CodeRe.MatchString(bad), bad)
	}
}
