package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"mindimprint/api/internal/gateway"
)

func stubResolver(_ context.Context) (gateway.Resolved, error) { return gateway.Resolved{Provider: "stub"}, nil }

func contains(raw json.RawMessage, sub string) bool { return strings.Contains(string(raw), sub) }

func TestRenderTeachingParsesGenerated(t *testing.T) {
	script := []gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: `{"title":"先别急着信","subtitle":"停一下。","body":["第一段。","第二段。"],"foreground_asset_id":"a0"}`},
		{Kind: gateway.EventDone},
	}
	in := CourseStepInput{Ordinal: 0, Kind: "teaching", Purpose: "建立停一下的习惯",
		Assets:          []CourseAsset{{ID: "a0", Kind: "text", Title: "开场", Value: "一张卫星图刷屏。"}},
		AuthoredContent: json.RawMessage(`{"title":"AUTH","subtitle":"a","body":["b"],"foreground_asset_id":null}`)}
	got := RenderCourseStep(context.Background(), in, gateway.NewStubProvider(script), stubResolver)
	if got.Source != "generated" || got.Template != "teaching" {
		t.Fatalf("want generated teaching, got %+v", got)
	}
	if !contains(got.Content, "先别急着信") {
		t.Fatalf("content not from model: %s", got.Content)
	}
}

func TestRenderTeachingFallsBackOnGarbage(t *testing.T) {
	script := []gateway.StreamEvent{{Kind: gateway.EventTextDelta, TextDelta: "not json"}, {Kind: gateway.EventDone}}
	in := CourseStepInput{Ordinal: 0, Kind: "teaching", Purpose: "p",
		Assets:          []CourseAsset{{ID: "a0", Kind: "text", Value: "x"}},
		AuthoredContent: json.RawMessage(`{"title":"AUTH","subtitle":"a","body":["b"]}`)}
	got := RenderCourseStep(context.Background(), in, gateway.NewStubProvider(script), stubResolver)
	if got.Source != "authored" || !contains(got.Content, "AUTH") {
		t.Fatalf("want authored fallback, got %+v", got)
	}
}

func TestRenderChallengeUsesAnchorsFromAsset(t *testing.T) {
	ct := "verify_claim"
	script := []gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: `[{"block_id":"b0","quote":"某科技博主综合整理","dimension":"权威性","question":"作者是谁？"}]`},
		{Kind: gateway.EventDone},
	}
	in := CourseStepInput{Ordinal: 2, Kind: "challenge", Purpose: "核查一处断言", ChallengeType: &ct,
		Assets:          []CourseAsset{{ID: "m0", Kind: "text", Title: "片段", Value: "某科技博主综合整理的文章称地球绿了 5%。"}},
		AuthoredContent: json.RawMessage(`{"title":"现在轮到你","prompt":"哪句是事实？","anchors":[],"reason_hint":"说说理由"}`)}
	got := RenderCourseStep(context.Background(), in, gateway.NewStubProvider(script), stubResolver)
	if got.Template != "challenge" || got.Source != "generated" {
		t.Fatalf("want generated challenge, got %+v", got)
	}
	if !contains(got.Content, "作者是谁？") || !contains(got.Content, "现在轮到你") {
		t.Fatalf("challenge content wrong: %s", got.Content)
	}
}
