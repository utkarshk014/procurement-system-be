package domain_trust

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
)

func fieldsOf(err error) map[string]string {
	out := map[string]string{}
	if ae, ok := domain.AsAppError(err); ok {
		for _, f := range ae.Fields {
			out[f.Field] = f.Message
		}
	}
	return out
}

// One case per sentence pms.mst_trust_validate can return.
func TestFunctionMessageField(t *testing.T) {
	cases := map[string]string{
		"Trust code must contain exactly 3 uppercase alphabetic characters": "trust_code",
		"Trust description cannot be empty":                                 "trust_desc",
		"Trust description already exists":                                  "trust_desc",
		"Working financial year is required":                                "wrk_st_finyr",
		"Invalid financial year":                                            "wrk_st_finyr",
		"Working start date is required":                                    "wrk_st_date",
		"Something unexpected":                                              "",
	}
	for sentence, want := range cases {
		assert.Equal(t, want, FunctionMessageField(sentence), sentence)
	}
}

func TestFunctionErrorsSplitsEverySentence(t *testing.T) {
	msg := "Trust code must contain exactly 3 uppercase alphabetic characters. Trust description cannot be empty. " +
		"Invalid financial year. Please select from mst_fin_year. Working start date is required."
	err := FunctionErrors(msg)
	assert.True(t, domain.IsKind(err, domain.KindValidation))
	f := fieldsOf(err)
	assert.Equal(t, "Trust code must contain exactly 3 uppercase alphabetic characters", f["trust_code"])
	assert.Equal(t, "Trust description cannot be empty", f["trust_desc"])
	assert.Equal(t, "Invalid financial year. Please select from mst_fin_year", f["wrk_st_finyr"], "the hint stays with its sentence")
	assert.Equal(t, "Working start date is required", f["wrk_st_date"])
	assert.NotContains(t, f, "")
}

func TestDuplicateDescriptionFromTheFunctionIsAConflict(t *testing.T) {
	err := FunctionErrors("Trust description already exists.")
	assert.True(t, domain.IsKind(err, domain.KindConflict))
	assert.Equal(t, "Trust description already exists", fieldsOf(err)["trust_desc"])
}

func TestUnknownMessageHasNoField(t *testing.T) {
	err := FunctionErrors("Something unexpected")
	assert.True(t, domain.IsKind(err, domain.KindValidation))
	assert.Equal(t, "Something unexpected", fieldsOf(err)[""])
}

func TestNormalizeCode(t *testing.T) {
	assert.Equal(t, "SAT", NormalizeCode(" sat "))
	assert.True(t, CodeRe.MatchString("SAT"))
	for _, bad := range []string{"SA", "SATT", "S4T", "sat", ""} {
		assert.False(t, CodeRe.MatchString(bad), bad)
	}
}
