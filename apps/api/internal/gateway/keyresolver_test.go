package gateway

import (
	"context"
	"testing"

	"mindimprint/api/internal/config"
)

func TestKeyResolverPrefersDeepSeek(t *testing.T) {
	r := NewKeyResolver(config.Config{DeepSeekKey: "sk-deepseek", AnthropicKey: "sk-anthropic"})
	got, err := r(context.Background())
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got.Provider != "deepseek" {
		t.Fatalf("provider = %q, want deepseek", got.Provider)
	}
	if got.BaseURL != "https://api.deepseek.com/v1" {
		t.Fatalf("baseURL = %q", got.BaseURL)
	}
	if got.Model != "deepseek-chat" {
		t.Fatalf("model = %q", got.Model)
	}
	if got.APIKey != "sk-deepseek" {
		t.Fatalf("apiKey not wired")
	}
	if got.Tier != "chaperone" {
		t.Fatalf("tier = %q, want chaperone", got.Tier)
	}
}

func TestKeyResolverFallsBackToAnthropic(t *testing.T) {
	r := NewKeyResolver(config.Config{AnthropicKey: "sk-anthropic"})
	got, err := r(context.Background())
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got.Provider != "anthropic" {
		t.Fatalf("provider = %q, want anthropic", got.Provider)
	}
	if got.BaseURL != "https://api.anthropic.com/v1" {
		t.Fatalf("baseURL = %q", got.BaseURL)
	}
	if got.Model != "claude-3-5-sonnet-latest" {
		t.Fatalf("model = %q", got.Model)
	}
}

func TestKeyResolverErrorsWhenNoKey(t *testing.T) {
	r := NewKeyResolver(config.Config{})
	_, err := r(context.Background())
	if err == nil {
		t.Fatal("want error when no provider key configured")
	}
}

func TestEvalResolverIsFlagship(t *testing.T) {
	r, err := NewEvalKeyResolver(config.Config{DeepSeekKey: "k"})(context.Background())
	if err != nil { t.Fatal(err) }
	if r.Tier != "flagship" || r.Model != "deepseek-reasoner" {
		t.Fatalf("want flagship deepseek-reasoner, got %s/%s", r.Tier, r.Model)
	}
	if _, err := NewEvalKeyResolver(config.Config{})(context.Background()); err == nil {
		t.Fatal("want error when no provider configured")
	}
}
