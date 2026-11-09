package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeIndianMobile(t *testing.T) {
	cases := map[string]string{
		"9876543210":       "9876543210",
		"+91 98765-43210":  "9876543210",
		"919876543210":     "9876543210",
		"09876543210":      "9876543210",
		"(98765) 43210":    "9876543210",
		"12345":            "12345",
		"+1 (555) 0100123": "15550100123",
		" abc ":            "abc",
		"--":               "--",
	}
	for in, want := range cases {
		assert.Equal(t, want, NormalizeIndianMobile(in), in)
	}
}

func TestIsIndianMobile(t *testing.T) {
	assert.True(t, IsIndianMobile("9876543210"))
	assert.True(t, IsIndianMobile("6000000000"))
	assert.False(t, IsIndianMobile("5876543210"), "must start with 6-9")
	assert.False(t, IsIndianMobile("987654321"))
	assert.False(t, IsIndianMobile("98765432100"))
}

func TestIsIndianPIN(t *testing.T) {
	assert.True(t, IsIndianPIN("560001"))
	assert.False(t, IsIndianPIN("060001"), "cannot start with 0")
	assert.False(t, IsIndianPIN("56001"))
	assert.False(t, IsIndianPIN("56000A"))
}

func TestIsPAN(t *testing.T) {
	assert.True(t, IsPAN("ABCDE1234F"))
	assert.False(t, IsPAN("abcde1234f"))
	assert.False(t, IsPAN("ABCD1234F"))
}

func TestIsGSTIN(t *testing.T) {
	// Built from a valid PAN with the check digit computed by gstinCheckDigit.
	base := "29ABCDE1234F1Z"
	valid := base + string(gstinCheckDigit(base))
	assert.True(t, IsGSTIN(valid))

	wrong := base + "0"
	if wrong == valid {
		wrong = base + "1"
	}
	assert.False(t, IsGSTIN(wrong), "bad check digit")
	assert.False(t, IsGSTIN("29ABCDE1234F1Z"), "too short")
}

func TestGSTINKnownCheckDigit(t *testing.T) {
	// Published example GSTIN for the GSTN checksum algorithm.
	assert.True(t, IsGSTIN("27AAPFU0939F1ZV"))
}

func TestIsEmail(t *testing.T) {
	assert.True(t, IsEmail("a@b.in"))
	assert.True(t, IsEmail("first.last+tag@example.co.in"))
	assert.False(t, IsEmail("abc@"))
	assert.False(t, IsEmail("not-an-email"))
}

func TestIsWebsite(t *testing.T) {
	assert.True(t, IsWebsite("esis.in"))
	assert.True(t, IsWebsite("https://www.esis.in"))
	assert.True(t, IsWebsite("http://esis.org/about"))
	assert.False(t, IsWebsite("esis"))
	assert.False(t, IsWebsite("ftp://esis.in"))
}

func TestCanonicalIndianState(t *testing.T) {
	s, ok := CanonicalIndianState("  karnataka ")
	assert.True(t, ok)
	assert.Equal(t, "Karnataka", s)
	_, ok = CanonicalIndianState("Atlantis")
	assert.False(t, ok)
	assert.Len(t, IndianStates, 36)
}

func TestStructCollectsEveryViolation(t *testing.T) {
	type input struct {
		Name   string `json:"name" validate:"required,max=5"`
		Mobile string `json:"mobile" validate:"omitempty,in_mobile"`
		PIN    string `json:"pin" validate:"omitempty,in_pin"`
	}
	v := Struct(input{Name: "toolong", Mobile: "123", PIN: "1"})
	assert.Equal(t, []Violation{
		{Field: "name", Message: "must be at most 5 characters"},
		{Field: "mobile", Message: "must be a 10-digit Indian mobile number"},
		{Field: "pin", Message: "must be a 6-digit PIN code"},
	}, v)
	assert.Nil(t, Struct(input{Name: "ok"}))
}
