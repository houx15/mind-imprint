package api

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNormalizeShowcaseBoundsPrivateAboutConversation(t *testing.T) {
	c := defaultShowcase("小雨")
	c.AboutConversation = []showcaseChatMessage{
		{Role: "user", Content: "  我喜欢观察城市里的植物。  "},
		{Role: "assistant", Content: "你最常观察哪一种植物？"},
	}
	got, err := normalizeShowcase(c)
	if err != nil {
		t.Fatal(err)
	}
	if got.AboutConversation[0].Content != "我喜欢观察城市里的植物。" {
		t.Fatalf("conversation not trimmed: %#v", got.AboutConversation)
	}

	c.AboutConversation = make([]showcaseChatMessage, 21)
	for i := range c.AboutConversation {
		c.AboutConversation[i] = showcaseChatMessage{Role: "user", Content: "内容"}
	}
	if _, err := normalizeShowcase(c); err == nil {
		t.Fatal("21 private messages should be rejected")
	}
}

func TestPublishedShowcaseStripsPrivateDraftingInputs(t *testing.T) {
	c := redactShowcaseForPublic(showcaseConfig{
		Name:              "小雨",
		AboutConversation: []showcaseChatMessage{{Role: "user", Content: "这句话不能公开"}},
		HeroImagePrompt:   "私有封面提示词",
		AvatarImagePrompt: "私有头像提示词",
	})
	publicConfig, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	for _, privateValue := range []string{"aboutConversation", "这句话不能公开", "私有封面提示词", "私有头像提示词"} {
		if strings.Contains(string(publicConfig), privateValue) {
			t.Fatalf("private drafting input escaped public config: %s", publicConfig)
		}
	}
}

func TestNormalizeShowcaseCustomWorksAndComponents(t *testing.T) {
	c := defaultShowcase("小雨")
	c.CustomWorks = []showcaseCustomWork{{ID: "debate-final", Title: "辩论作品", Summary: "校际展示", URL: "https://example.org/work", Date: "2026-09-22"}}
	c.SelectedWorkIDs = []string{"external:debate-final"}
	c.HomeWorkLimit = 3
	c.Components = []showcaseComponent{{ID: "orbit", Title: "兴趣轨道", Format: "svg", Source: `<svg xmlns="http://www.w3.org/2000/svg"><circle r="4"/></svg>`, Height: 320, Placement: "after-about", Enabled: true}}
	if _, err := normalizeShowcase(c); err != nil {
		t.Fatal(err)
	}
	bad := c
	bad.CustomWorks[0].URL = "http://example.org/work"
	if _, err := normalizeShowcase(bad); err == nil {
		t.Fatal("non-HTTPS custom work accepted")
	}
	bad = c
	bad.Components[0].Source = `<svg><foreignObject/></svg>`
	if _, err := normalizeShowcase(bad); err == nil {
		t.Fatal("embedded HTML in SVG accepted")
	}
}

func TestShowcaseSemanticConfigPublishesOnlyEnabledAndSelected(t *testing.T) {
	c := defaultShowcase("小雨")
	c.CustomWorks = []showcaseCustomWork{{ID: "selected", Title: "A", URL: "https://example.org/a"}, {ID: "private", Title: "B", URL: "https://example.org/b"}}
	c.SelectedWorkIDs = []string{"external:selected"}
	c.Components = []showcaseComponent{{ID: "on", Format: "html", Source: "<canvas></canvas>", Height: 200, Placement: "after-works", Enabled: true}, {ID: "off", Format: "html", Source: "<p>draft</p>", Height: 200, Placement: "after-works", Enabled: false}}
	got := showcaseSemanticConfig(c)
	if len(got.CustomWorks) != 1 || got.CustomWorks[0].ID != "selected" || len(got.Components) != 1 || got.Components[0].ID != "on" {
		t.Fatalf("semantic snapshot = %#v", got)
	}
}
