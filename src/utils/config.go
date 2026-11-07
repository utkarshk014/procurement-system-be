package utils

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

// Config is read from the environment (or a git-ignored .env); secrets never live in the repo.
type Config struct {
	AppEnv   string `env:"APP_ENV" envDefault:"development"`
	HTTPPort int    `env:"HTTP_PORT" envDefault:"8080"`
	LogLevel string `env:"LOG_LEVEL" envDefault:"info"`

	// ControlDatabaseURL holds the platform tables (tenants, users, refresh tokens).
	ControlDatabaseURL string `env:"CONTROL_DATABASE_URL,required"`

	// TenantDatabasesJSON maps db_key to database URL as JSON, e.g. {"esis":"postgres://..."}.
	TenantDatabasesJSON string `env:"TENANT_DATABASES,required"`
	TenantDatabases     map[string]string

	// DefaultTimeZone decides "today" for financial years and stamps company_master.time_zone.
	DefaultTimeZone string `env:"DEFAULT_TIMEZONE" envDefault:"Asia/Kolkata"`

	// Access and refresh tokens use different secrets so one can never stand in for the other.
	JWTAccessSecret  string        `env:"JWT_ACCESS_SECRET,required"`
	JWTRefreshSecret string        `env:"JWT_REFRESH_SECRET,required"`
	JWTIssuer        string        `env:"JWT_ISSUER" envDefault:"pms-api"`
	AccessTokenTTL   time.Duration `env:"ACCESS_TOKEN_TTL" envDefault:"15m"`
	RefreshTokenTTL  time.Duration `env:"REFRESH_TOKEN_TTL" envDefault:"720h"`

	// TrustProxy reads the client IP from X-Forwarded-For; enable only behind a proxy that sets it.
	TrustProxy bool `env:"TRUST_PROXY" envDefault:"false"`

	CORSAllowedOrigins []string      `env:"CORS_ALLOWED_ORIGINS" envSeparator:"," envDefault:"http://localhost:3000"`
	RequestTimeout     time.Duration `env:"REQUEST_TIMEOUT" envDefault:"15s"`
	LoginRateLimit     int           `env:"LOGIN_RATE_LIMIT_PER_MINUTE" envDefault:"10"`
}

// NewConfig loads .env (if present) and parses the environment.
func NewConfig() (*Config, error) {
	_ = godotenv.Load() // a missing .env is normal outside local development

	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if err := json.Unmarshal([]byte(cfg.TenantDatabasesJSON), &cfg.TenantDatabases); err != nil {
		return nil, fmt.Errorf("TENANT_DATABASES must be a JSON object of db_key to URL: %w", err)
	}
	if len(cfg.TenantDatabases) == 0 {
		return nil, fmt.Errorf("TENANT_DATABASES must define at least one database")
	}
	if _, err := time.LoadLocation(cfg.DefaultTimeZone); err != nil {
		return nil, fmt.Errorf("DEFAULT_TIMEZONE %q: %w", cfg.DefaultTimeZone, err)
	}
	if len(cfg.JWTAccessSecret) < 32 || len(cfg.JWTRefreshSecret) < 32 {
		return nil, fmt.Errorf("JWT_ACCESS_SECRET and JWT_REFRESH_SECRET must each be at least 32 characters")
	}
	if cfg.JWTAccessSecret == cfg.JWTRefreshSecret {
		return nil, fmt.Errorf("JWT_ACCESS_SECRET and JWT_REFRESH_SECRET must differ")
	}
	return cfg, nil
}

// Location returns the parsed DefaultTimeZone.
func (c *Config) Location() *time.Location {
	loc, _ := time.LoadLocation(c.DefaultTimeZone) // validated in NewConfig
	return loc
}
