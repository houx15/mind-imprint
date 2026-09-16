package pbl

import (
	"context"
	"fmt"
	"mindimprint/api/internal/config"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/materialize"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// Opt-in check of the actual recommended page and configured digest model.
func TestLiveSiteRefRecommendedPage(t *testing.T) {
	if os.Getenv("LIVE_LLM") != "1" {
		t.Skip("set LIVE_LLM=1")
	}
	wd, _ := os.Getwd()
	if err := os.Chdir("../.."); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	rs, err := gateway.NewResolvers(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	resolved, err := rs.For(gateway.ClassDigest)(ctx)
	if err != nil {
		t.Fatal(err)
	}
	title, body, _, err := materialize.NewFetcher().FetchReadable(ctx, "https://lilianweng.github.io/")
	if err != nil {
		t.Fatal(err)
	}
	if r := []rune(body); len(r) > maxSitePageRunes {
		body = string(r[:maxSitePageRunes])
	}
	p := gateway.NewMuxProvider(map[string]gateway.Provider{gateway.KindOpenAICompatible: gateway.NewCatalogProvider(&http.Client{})})
	res, err := gateway.Collect(ctx, p, resolved, gateway.ChatRequest{MaxTokens: 1024, Messages: []gateway.ChatMessage{{Role: gateway.RoleSystem, Content: siteRefSystem}, {Role: gateway.RoleUser, Content: fmt.Sprintf("页面标题：%s\n\n正文：\n%s", title, body)}}})
	if err != nil {
		t.Fatal(err)
	}
	card, err := ParseSiteRefCard(res.Text)
	if err != nil {
		t.Fatal(err)
	}
	if err = ValidateSiteRefEvidence(card, body); err != nil {
		for i, q := range card.Evidence {
			t.Logf("evidence[%d]=%q matched=%v", i, q, strings.Contains(strings.Join(strings.Fields(body), " "), strings.Join(strings.Fields(q), " ")))
		}
		t.Logf("fetched public text prefix: %.1500s", body)
		t.Logf("first extraction failed: %v; checking the production repair path", err)
		card, _, err = ReadSiteRef(ctx, p, resolved, title, body, true)
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("valid recommended page: %d source quotes", len(card.Evidence))
}
