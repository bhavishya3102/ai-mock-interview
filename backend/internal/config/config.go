// Package config loads runtime configuration from environment variables.
// In non-production environments it also loads a .env file. Required values
// fail-fast so misconfigured deployments never silently start.
package config

import (
	"fmt"
	"strings"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

// Config is the resolved runtime configuration. All env vars are read from
// process environment via envconfig; secrets are never logged.
type Config struct {
	AppEnv             string `envconfig:"APP_ENV"              default:"development"`
	Port               string `envconfig:"PORT"                 default:"8080"`
	LogLevel           string `envconfig:"LOG_LEVEL"            default:"info"`
	DatabaseURL        string `envconfig:"DATABASE_URL"         required:"true"`
	DatabaseURLDirect  string `envconfig:"DATABASE_URL_DIRECT"`
	ClerkSecretKey     string `envconfig:"CLERK_SECRET_KEY"     required:"true"`
	GeminiAPIKey       string `envconfig:"GEMINI_API_KEY"       required:"true"`
	GeminiModel        string `envconfig:"GEMINI_MODEL"         default:"gemini-2.5-flash"`
	CORSAllowedOrigins string `envconfig:"CORS_ALLOWED_ORIGIN"  default:"http://localhost:5173"`
}

// Load reads env vars (after optionally loading .env in dev) and returns a
// validated Config. Production must NOT depend on a .env file — env is
// injected by the platform.
func Load() (*Config, error) {
	if !isProduction() {
		// Best-effort: missing .env in dev is fine when env vars are exported.
		_ = godotenv.Load()
	}

	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("envconfig process: %w", err)
	}

	return &cfg, nil
}

// AllowedOrigins splits CORSAllowedOrigins by comma into a clean slice.
func (c *Config) AllowedOrigins() []string {
	raw := strings.Split(c.CORSAllowedOrigins, ",")
	out := make([]string, 0, len(raw))
	for _, o := range raw {
		o = strings.TrimSpace(o)
		if o != "" {
			out = append(out, o)
		}
	}
	return out
}

// IsProduction reports whether the resolved environment is production.
func (c *Config) IsProduction() bool {
	return strings.EqualFold(c.AppEnv, "production")
}

// isProduction is the package-level early check used before envconfig has
// fully populated the struct (we only need APP_ENV for the .env decision).
func isProduction() bool {
	var early struct {
		AppEnv string `envconfig:"APP_ENV"`
	}
	_ = envconfig.Process("", &early)
	return strings.EqualFold(early.AppEnv, "production")
}
