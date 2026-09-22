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
