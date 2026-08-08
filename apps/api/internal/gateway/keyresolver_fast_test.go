package gateway

import (
	"context"
	"testing"

	"mindimprint/api/internal/config"
)

func TestNewFastChaperoneResolver_UsesFlash(t *testing.T) {
	r := NewFastChaperoneResolver(config.Config{DeepSeekKey: "sk-x"})
	got, err := r(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Model != "deepseek-v4-flash" {
		t.Errorf("fast resolver model = %q, want deepseek-v4-flash", got.Model)
	}
	if got.Tier != "chaperone" {
		t.Errorf("fast resolver tier = %q, want chaperone", got.Tier)
	}
}

func TestNewFastChaperoneResolver_NoProvider(t *testing.T) {
	if _, err := NewFastChaperoneResolver(config.Config{})(context.Background()); err == nil {
		t.Error("expected errNoProvider with no keys")
	}
}
