package domain_institution

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
)

// Every message in pms.fn_mst_inst_validate and pms.sp_mst_inst_insert lands on its field.
func TestFunctionErrorMapsEveryUncleMessage(t *testing.T) {
	cases := map[string]string{
		"Institution code is mandatory":                                   "inst_code",
		"Institution code must be exactly 7 characters":                   "inst_code",
		"Full institution code must be exactly 14 characters":             "inst_code",
		"Institution registration code must contain exactly 6 characters": "inst_reg_code",
		"Registration code already exists":                                "inst_reg_code",
		"Invalid place code":                                              "place_code",
		"Invalid branch code: XYZ":                                        "branch_code",
		"Invalid trust code":                                              "trust_code",
		"Invalid institution email address":                               "inst_email",
		"Invalid institution head email address":                          "inst_head_email",
		"Invalid accountant email address":                                "inst_acnt_email",
		"Invalid liaison email address":                                   "inst_liason_email",
		"Institution head mobile must contain 10 digits":                  "inst_head_mobile",
		"Accountant mobile must contain 10 digits":                        "inst_acnt_mobile",
		"Liaison mobile must contain 10 digits":                           "inst_liason_mobile",
	}
	for msg, field := range cases {
		app, ok := domain.AsAppError(FunctionError(msg))
		if assert.True(t, ok, msg) {
			assert.Equal(t, field, app.Fields[0].Field, msg)
		}
	}
	app, _ := domain.AsAppError(FunctionError("Institution code already exists"))
	assert.Equal(t, domain.KindConflict, app.Kind, "a duplicate is a 409")
}

func TestCodesAndFullCode(t *testing.T) {
	assert.True(t, CodeRe.MatchString("ACU-009"))
	assert.False(t, CodeRe.MatchString("ACU-00009"), "Uncle's 9 characters cannot form a 14-character full code")
	assert.True(t, RegCodeRe.MatchString("BGN-09"))
	assert.False(t, RegCodeRe.MatchString("BGN-9"))
	full := FullCode("ACU-009", "BGN-09")
	assert.Equal(t, "ACU-009:BGN-09", full)
	assert.Len(t, full, 14, "the old system's \"ACM Code (14 Char.)\"")
}
