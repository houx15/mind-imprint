package api

import (
	"strings"
	"testing"
)

func TestShowcaseGuideRequestAndProposalStayWithinCurrentStep(t *testing.T) {
	in := showcaseGuideRequest{Stage: "hero", Messages: []showcaseChatMessage{{Role: "user", Content: "  我想要蓝色星空  "}}}
	normalized, err := normalizeShowcaseGuideRequest(in)
	if err != nil || normalized.Messages[0].Content != "我想要蓝色星空" {
		t.Fatalf("normalize: %#v %v", normalized, err)
	}
	if _, err := normalizeShowcaseGuideRequest(showcaseGuideRequest{Stage: "hero", Messages: []showcaseChatMessage{{Role: "assistant", Content: "结束"}}}); err == nil {
		t.Fatal("latest message must come from student")
	}
	if _, err := parseShowcaseGuideReply(`{"reply":"可以让标题和画面呼应。","proposal":{"heroTitle":"欢迎来到星空","reason":"对应学生说的星空"}}`, "hero"); err != nil {
		t.Fatal(err)
	}
	if _, err := parseShowcaseGuideReply(`{"reply":"建议","proposal":{"bio":"我得过奖","reason":"猜测"}}`, "hero"); err == nil {
		t.Fatal("a hero turn must not alter the student's biography")
	}
	if _, err := parseShowcaseGuideReply(`{"reply":"建议","proposal":{"style":"unsupported","reason":"理由"}}`, "design"); err == nil {
		t.Fatal("unsupported visual enum must fail")
	}
}

func TestShowcaseGeneratedComponentIsValidatedBeforeDraft(t *testing.T) {
	if _, err := normalizeShowcaseComponentRequest(showcaseComponentRequest{Prompt: "画一颗星球", Format: "svg", Style: "cute", Palette: "ocean"}); err != nil {
		t.Fatal(err)
	}
	valid := `{"title":"星球","source":"<svg xmlns=\"http://www.w3.org/2000/svg\" viewBox=\"0 0 100 100\"><circle cx=\"50\" cy=\"50\" r=\"20\"/></svg>","height":280,"placement":"after-about","explanation":"一个圆形星球"}`
	if _, err := parseShowcaseGeneratedComponent(valid, "svg"); err != nil {
		t.Fatal(err)
	}
	if _, err := parseShowcaseGeneratedComponent(strings.Replace(valid, "<circle", "<foreignObject", 1), "svg"); err == nil {
		t.Fatal("invalid SVG must fail before student preview")
	}
	if _, err := parseShowcaseGeneratedComponent(`{"title":"互动","source":"<div>没有画布</div>","height":280,"placement":"after-about","explanation":"说明"}`, "html"); err == nil {
		t.Fatal("canvas generation must contain a canvas")
	}
}

func TestShowcasePrivateGuideDataNeverEntersPublicConfig(t *testing.T) {
	c := defaultShowcase("学生")
	c.GuideConversation = []showcaseChatMessage{{Role: "user", Content: "我的私人想法"}}
	c.ComponentPrompt = "私有生成提示"
	public := redactShowcaseForPublic(c)
	if public.GuideConversation != nil || public.ComponentPrompt != "" {
		t.Fatalf("private design data leaked: %#v", public)
	}
	semantic := showcaseSemanticConfig(c)
	if semantic.GuideConversation != nil || semantic.ComponentPrompt != "" {
		t.Fatalf("private design data affected published comparison: %#v", semantic)
	}
}
