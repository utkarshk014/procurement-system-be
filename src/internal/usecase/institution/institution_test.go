package usecase_institution

import (
	"testing"

	"github.com/stretchr/testify/assert"
	domain_institution "github.com/utkarshk014/procurement-system-be/src/internal/domain/institution"
)

func fields(in domain_institution.Input) map[string]string {
	out := map[string]string{}
	for _, f := range Check(Normalize(in), "2026-11-09") {
		out[f.Field] = f.Message
	}
	return out
}

func valid() domain_institution.Input {
	return domain_institution.Input{
		TrustCode: "sat", Code: "sat-001", RegCode: "mys-01", Name: "BGS School",
		PlaceCode: "mys", BranchCode: "myb", FromFinYr: "2026-2027",
	}
}

func TestValidInputHasNoProblems(t *testing.T) {
	assert.Empty(t, fields(valid()))
}

func TestNormalizeTidiesEveryField(t *testing.T) {
	in := valid()
	in.Name, in.HeadMobile, in.AcntMobile, in.Email = "  BGS   School ", "+91 98450-12345", "", " Office@BGS.Test "
	n := Normalize(in)
	assert.Equal(t, "SAT", n.TrustCode)
	assert.Equal(t, "SAT-001", n.Code)
	assert.Equal(t, "MYS-01", n.RegCode)
	assert.Equal(t, "BGS School", n.Name)
	assert.Equal(t, "9845012345", n.HeadMobile)
	assert.Equal(t, "", n.AcntMobile, "an empty mobile stays empty")
	assert.Equal(t, "office@bgs.test", n.Email)
}

func TestCheckReportsEveryProblemAtOnce(t *testing.T) {
	in := valid()
	in.Code, in.RegCode, in.Name, in.PlaceCode = "ACU-001", "MYS-1", "", "M1S"
	in.RegDate, in.HeadMobile, in.LiasonEmail = "2026-11-10", "abc", "nope"
	f := fields(in)
	assert.Equal(t, "must start with the trust code SAT-", f["inst_code"])
	assert.Equal(t, "must be a region code, a dash and 2 digits, e.g. MYS-01", f["inst_reg_code"])
	assert.Equal(t, "is required", f["inst_name"])
	assert.Equal(t, "must be exactly 3 letters", f["place_code"])
	assert.Equal(t, "cannot be in the future", f["reg_date"])
	assert.Contains(t, f, "inst_head_mobile", "junk is rejected, not saved empty")
	assert.Contains(t, f, "inst_liason_email")
}

func TestRegDateTodayIsAllowed(t *testing.T) {
	in := valid()
	in.RegDate = "2026-11-09"
	assert.Empty(t, fields(in))
}
