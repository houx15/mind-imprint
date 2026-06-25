// Package config loads typed service configuration from the environment.
//
// 12-factor: configuration lives in env vars. A local .env.local is loaded as a
// developer convenience only and never committed. Missing required values are a
// fatal, fail-fast error at boot.
package config

import (
	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

// Config is the fully-resolved service configuration.
type Config struct {
	// Port is the HTTP listen port; defaults to 8080.
	Port string `env:"PORT" envDefault:"8080"`
	// DatabaseURL is the Postgres DSN. Required.
	DatabaseURL string `env:"DATABASE_URL,required"`
	// CORSOrigins is the comma-separated allowlist of SPA origins.
	CORSOrigins []string `env:"CORS_ORIGINS" envSeparator:","`
	// AnthropicKey is the Anthropic provider key (server-side only).
	AnthropicKey string `env:"ANTHROPIC_API_KEY"`
	// DeepSeekKey is the DeepSeek provider key (server-side only).
	DeepSeekKey string `env:"DEEPSEEK_API_KEY"`
	// CookieSecure sets the Secure flag on the session cookie. Default true;
	// set COOKIE_SECURE=false for local http dev so the browser sends it.
	CookieSecure bool `env:"COOKIE_SECURE" envDefault:"true"`
}

// Load reads .env.local if present (ignored if absent), then parses the
// environment into a Config. Returns an error if a required value is missing.
func Load() (Config, error) {
	// Best-effort local file; absence is not an error.
	_ = godotenv.Load(".env.local")
	return env.ParseAs[Config]()
}
