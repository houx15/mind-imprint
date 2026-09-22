package api

import (
	"strings"
	"testing"

	"mindimprint/api/internal/gateway"
)

func TestNormalizeShowcaseAboutRequestBoundsAndLatestStudent(t *testing.T) {
	in := showcaseAboutRequest{
		Messages: []showcaseChatMessage{{Role: "assistant", Content: "你关注什么？"}, {Role: "user", Content: "  城市里的植物。  "}},
		Profile:  showcaseAboutProfile{Name: " 小雨 ", Interests: []string{"植物", "植物"}, AboutLayout: "orbit"},
	}
	got, err := normalizeShowcaseAboutRequest(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Messages[1].Content != "城市里的植物。" || got.Profile.Name != "小雨" || len(got.Profile.Interests) != 1 {
		t.Fatalf("unexpected normalized request: %#v", got)
	}

	in.Messages = []showcaseChatMessage{{Role: "user", Content: strings.Repeat("界", 2001)}}
	if _, err := normalizeShowcaseAboutRequest(in); err == nil {
		t.Fatal("message over 2,000 runes should be rejected")
	}
	in.Messages = []showcaseChatMessage{{Role: "assistant", Content: "结束"}}
	if _, err := normalizeShowcaseAboutRequest(in); err == nil {
		t.Fatal("latest message must come from the student")
	}
}

func TestShowcaseAboutMessagesContainOnlyAuthorizedInput(t *testing.T) {
	in := showcaseAboutRequest{
		Messages: []showcaseChatMessage{{Role: "user", Content: "我喜欢植物。"}},
		Profile:  showcaseAboutProfile{Name: "小雨", Bio: "观察生活", Interests: []string{"植物"}, AboutLayout: "classic"},
	}
	messages := showcaseAboutMessages(in)
	if len(messages) != 3 || messages[0].Role != gateway.RoleSystem || messages[2].Content != "我喜欢植物。" {
		t.Fatalf("unexpected model messages: %#v", messages)
	}
	joined := messages[1].Content + messages[2].Content
	for _, forbidden := range []string{"source", "evidence", "interestTree", "note"} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("unauthorized private field %q entered model context", forbidden)
		}
	}
}

func TestParseShowcaseAboutReplyValidatesProposal(t *testing.T) {
	got, err := parseShowcaseAboutReply(`{"reply":"你最常观察哪一种植物？","proposal":null}`)
	if err != nil || got.Proposal != nil {
		t.Fatalf("question reply: got=%#v err=%v", got, err)
	}
	got, err = parseShowcaseAboutReply(`{"reply":"已经可以整理一版。","proposal":{"name":"小雨","bio":"我关注城市中的植物。","interests":["植物观察"],"aboutLayout":"orbit","reason":"多个明确兴趣适合环绕展示。"}}`)
	if err != nil || got.Proposal == nil || got.Proposal.AboutLayout != "orbit" {
		t.Fatalf("proposal reply: got=%#v err=%v", got, err)
	}
	if _, err := parseShowcaseAboutReply(`{"reply":"建议","proposal":{"name":"小雨","bio":"简介","interests":[],"aboutLayout":"unknown","reason":"理由"}}`); err == nil {
		t.Fatal("invalid layout should be rejected")
	}
}
