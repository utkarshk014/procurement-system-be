package usecase_institution

import (
	"testing"

	"github.com/stretchr/testify/assert"
	domain_codelist "github.com/utkarshk014/procurement-system-be/src/internal/domain/codelist"
)

func TestHeaderKeyIgnoresCaseSpacesAndPunctuation(t *testing.T) {
	for _, h := range []string{"inst_code", "Inst Code", "INSTCODE", " inst-code "} {
		assert.Equal(t, "instcode", headerKey(h), h)
	}
	assert.Equal(t, "acminstregshortcode", headerKey("ACM_INST_REG_SHORT_CODE"))
}

func TestImportDateFormats(t *testing.T) {
	for in, want := range map[string]string{
		"2025-06-01": "2025-06-01", "01-06-2025": "2025-06-01", "01/06/2025": "2025-06-01",
		"1/6/2025": "2025-06-01", "June 2025": "June 2025", "": "",
	} {
		assert.Equal(t, want, importDate(in), in)
	}
}

func TestResolverFindsByCodeOrName(t *testing.T) {
	r := newResolver([]domain_codelist.Entry{{Code: "BGN", Name: "B G Nagara"}, {Code: "MYS", Name: "Mysore"}})
	code, wasCode, ok := r.find(" bgn ")
	assert.Equal(t, "BGN", code)
	assert.True(t, wasCode && ok)
	code, wasCode, ok = r.find("b g  NAGARA")
	assert.Equal(t, "BGN", code)
	assert.True(t, ok && !wasCode)
	_, _, ok = r.find("BG Nagara")
	assert.False(t, ok, "a different spelling is not guessed")
}

func TestMissingNamesGroupBySpelling(t *testing.T) {
	m := missingNames{}
	for _, s := range []string{"Kunigal", "KUNIGAL", " kunigal ", "Kaggere, Kunigal"} {
		m.add(s)
	}
	got := m.sorted()
	assert.Equal(t, "Kunigal", got[0].Name)
	assert.Equal(t, 3, got[0].Count)
	assert.Equal(t, 1, got[1].Count)
}
