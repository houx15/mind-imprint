package config

import (
	"os"
	"reflect"
	"testing"
)

// allEnvKeys are every variable Load reads. Each subtest starts from a clean
// slate by unsetting all of them, then setting only what the case needs.
var allEnvKeys = []string{
	"PORT", "DATABASE_URL", "CORS_ORIGINS", "ANTHROPIC_API_KEY", "DEEPSEEK_API_KEY", "COOKIE_SECURE",
}

func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range allEnvKeys {
		if err := os.Unsetenv(k); err != nil {
			t.Fatalf("unsetenv %s: %v", k, err)
		}
	}
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr bool
		check   func(t *testing.T, c Config)
	}{
		{
			name:    "missing required DATABASE_URL is an error",
			env:     map[string]string{},
			wantErr: true,
		},
		{
			name: "all set parses values and splits CORS origins",
			env: map[string]string{
				"PORT":              "9090",
				"DATABASE_URL":      "postgres://localhost/db",
				"CORS_ORIGINS":      "http://localhost:5173,https://app.example.com",
				"ANTHROPIC_API_KEY": "ak-test",
				"DEEPSEEK_API_KEY":  "dk-test",
				"COOKIE_SECURE":     "false",
			},
			wantErr: false,
			check: func(t *testing.T, c Config) {
				if c.Port != "9090" {
					t.Fatalf("Port = %q, want 9090", c.Port)
				}
				if c.DatabaseURL != "postgres://localhost/db" {
					t.Fatalf("DatabaseURL = %q", c.DatabaseURL)
				}
				want := []string{"http://localhost:5173", "https://app.example.com"}
				if !reflect.DeepEqual(c.CORSOrigins, want) {
					t.Fatalf("CORSOrigins = %#v, want %#v", c.CORSOrigins, want)
				}
				if c.AnthropicKey != "ak-test" || c.DeepSeekKey != "dk-test" {
					t.Fatalf("keys = %q/%q", c.AnthropicKey, c.DeepSeekKey)
				}
				if c.CookieSecure {
					t.Fatalf("CookieSecure = true, want false")
				}
			},
		},
		{
			name: "PORT defaults to 8080 when unset",
			env: map[string]string{
				"DATABASE_URL": "postgres://localhost/db",
			},
			wantErr: false,
			check: func(t *testing.T, c Config) {
				if c.Port != "8080" {
					t.Fatalf("Port = %q, want default 8080", c.Port)
				}
				if !c.CookieSecure {
					t.Fatalf("CookieSecure default = false, want true")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Start clean, then set only this case's vars. t.Setenv restores the
			// process environment automatically after the subtest.
			clearEnv(t)
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			got, err := Load()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Load() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() unexpected error: %v", err)
			}
			if tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}
