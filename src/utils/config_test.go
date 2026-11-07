package utils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func validConfig() Config {
	return Config{
		HTTPPort:        8080,
		RequestTimeout:  15 * time.Second,
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 720 * time.Hour,
		LoginRateLimit:  10,
		TrustedProxies:  []string{"10.0.0.0/8", "::1/128"},
	}
}

func TestCheckRanges(t *testing.T) {
	ok := validConfig()
	ok.TrustedProxies = []string{"10.0.0.0/8", " ::1/128"}
	assert.NoError(t, ok.checkRanges())
	assert.Equal(t, []string{"10.0.0.0/8", "::1/128"}, ok.TrustedProxies, "spaces after commas are trimmed")

	cases := map[string]func(*Config){
		"HTTP_PORT":                   func(c *Config) { c.HTTPPort = 0 },
		"REQUEST_TIMEOUT":             func(c *Config) { c.RequestTimeout = 0 },
		"ACCESS_TOKEN_TTL":            func(c *Config) { c.AccessTokenTTL = -5 * time.Minute },
		"REFRESH_TOKEN_TTL":           func(c *Config) { c.RefreshTokenTTL = c.AccessTokenTTL },
		"LOGIN_RATE_LIMIT_PER_MINUTE": func(c *Config) { c.LoginRateLimit = 0 },
		"TRUSTED_PROXIES":             func(c *Config) { c.TrustedProxies = []string{"10.0.0.1"} },
	}
	for name, breakIt := range cases {
		c := validConfig()
		breakIt(&c)
		assert.ErrorContains(t, c.checkRanges(), name)
	}
}
