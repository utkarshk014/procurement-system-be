package validation

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGSTStatesAreSortedUniqueAndUseIndianStateSpelling(t *testing.T) {
	assert.Len(t, GSTStates, 37)
	assert.True(t, sort.SliceIsSorted(GSTStates, func(i, j int) bool { return GSTStates[i].Code < GSTStates[j].Code }))
	seen := map[string]bool{}
	for _, s := range GSTStates {
		assert.Len(t, s.Code, 2, s.Name)
		assert.False(t, seen[s.Code], "duplicate code %s", s.Code)
		seen[s.Code] = true
		if s.Code == "97" {
			continue
		}
		canonical, ok := CanonicalIndianState(s.Name)
		assert.True(t, ok, "%s is not in IndianStates", s.Name)
		assert.Equal(t, canonical, s.Name)
	}
	assert.Len(t, seen, len(IndianStates)+1, "every state and UT has a code, plus Other Territory")
}

func TestGSTStateByCode(t *testing.T) {
	name, ok := GSTStateByCode("29")
	assert.True(t, ok)
	assert.Equal(t, "Karnataka", name)
	name, ok = GSTStateByCode("97")
	assert.True(t, ok)
	assert.Equal(t, "Other Territory", name)
	for _, retired := range []string{"25", "28", "00", "99", "2", ""} {
		_, ok := GSTStateByCode(retired)
		assert.False(t, ok, retired)
	}
}

func TestCanonicalGSTState(t *testing.T) {
	name, ok := CanonicalGSTState("  karnataka ")
	assert.True(t, ok)
	assert.Equal(t, "Karnataka", name)
	_, ok = CanonicalGSTState("Atlantis")
	assert.False(t, ok)
}

func TestRealFormatGSTINsWithValidCheckDigit(t *testing.T) {
	for _, g := range []string{"29ABCDE1234F1ZW", "27AAPFU0939F1ZV", "29AAACE1234A1ZC"} {
		assert.True(t, IsGSTIN(g), g)
	}
	assert.False(t, IsGSTIN("29ABCDE1234F1ZX"), "wrong check digit")
}

func TestGSTINMatchesPAN(t *testing.T) {
	assert.True(t, GSTINMatchesPAN("29ABCDE1234F1ZW", "ABCDE1234F"))
	assert.False(t, GSTINMatchesPAN("29ABCDE1234F1ZW", "ABCDE1234G"))
	assert.False(t, GSTINMatchesPAN("29ABCDE1234F1ZW", ""))
	assert.False(t, GSTINMatchesPAN("", "ABCDE1234F"))
}
