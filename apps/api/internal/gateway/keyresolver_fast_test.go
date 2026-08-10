package gateway

import (
	"context"
	"testing"

	"mindimprint/api/internal/config"
)

func TestNewFastChaperoneResolver_UsesProReasoningOff(t *testing.T) {
	// 2026-08-10 · the coach/guide side runs v4-pro with thinking disabled (per
	// buildBody's chaperone gate), chosen over v4-flash.
	r := NewFastChaperoneResolver(config.Config{DeepSeekKey: "sk-x"})
	got, err := r(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Model != "deepseek-v4-pro" {
		t.Errorf("fast resolver model = %q, want deepseek-v4-pro", got.Model)
	}
	if got.Tier != "chaperone" {
		t.Errorf("fast resolver tier = %q, want chaperone", got.Tier)
	}
}

func TestBuildBody_DisablesThinkingForChaperoneOnly(t *testing.T) {
	p := &DeepSeekProvider{}
	req := ChatRequest{Messages: []ChatMessage{{Role: RoleUser, Content: "hi"}}}
	chap := p.buildBody(Resolved{Model: "deepseek-v4-pro", Tier: "chaperone"}, req)
	if chap["thinking"] == nil {
		t.Error("chaperone tier should disable thinking")
	}
	flag := p.buildBody(Resolved{Model: "deepseek-v4-pro", Tier: "flagship"}, req)
	if flag["thinking"] != nil {
		t.Error("flagship tier should keep thinking (reasoning on)")
	}
}

func TestNewFastChaperoneResolver_NoProvider(t *testing.T) {
	if _, err := NewFastChaperoneResolver(config.Config{})(context.Background()); err == nil {
		t.Error("expected errNoProvider with no keys")
	}
}
