package agent

import (
	"context"
	"strings"
	"testing"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/rubric"
)

// reportProvider stubs a flagship reply (mirrors assessProvider in assess_test.go).
func reportProvider(reply string) *gateway.StubProvider {
	return gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: reply},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 80, OutputTokens: 40}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
}

const goodReport = `{
  "depthAxis":{"dims":[
    {"code":"D1","score":3,"evidence":"限定判断","promptEvidence":"R4"},
    {"code":"D3","score":2,"evidence":"NASA","promptEvidence":""},
    {"code":"D4","score":3,"evidence":"warrant","promptEvidence":""},
    {"code":"D5","score":3,"evidence":"理由","promptEvidence":""}]},
  "autonomyAxis":{"observation":"设边界","anchoredSignals":["R1"],"promptedSignals":["R3"],"adversaryInvites":0,"promptEvidence":""},
  "crossAxis":{"depthLevel":"L3","initiative":"引导后","prose":"能反思","promptEvidence":""},
  "solo":[{"round":4,"excerpt":"限定","level":"L3","rationale":"组织者","initiative":"自发"}],
  "promptLens":{"directiveRounds":3,"totalRounds":10,"boundarySettings":3,"adversaryInvites":0,
    "questions":[{"title":"一问","body":"…"}],
    "bestPrompt":{"round":8,"quote":"检查回扣","annotation":"齐备"},
    "takeaway":{"round":0,"quote":"苛刻审稿人","annotation":"P4"},
    "perRound":[{"round":1,"tier":"P3","label":"要过程·设边界"}]},
  "timeline":[{"round":1,"task":"上传","prompt":"不要重写","pTag":"P3","dimTags":["D1=2"]}],
  "keyEvidence":[{"label":"任务理解","quote":"改 thesis"}],
  "guidance":{"anchored":"限定 thesis","prompted":"SIFT","risk":"D3","nextSteps":[{"title":"强化 D3","body":"SIFT 记录"}]},
  "narrative":"深度 L3 稳定复现。"
}`

func TestAssessReportParsesAxes(t *testing.T) {
	rep, _, err := AssessReport(context.Background(), reportProvider(goodReport),
		gateway.Resolved{Tier: "flagship"}, rubric.Model(), AssessmentInput{})
	if err != nil {
		t.Fatalf("AssessReport: %v", err)
	}
	if len(rep.DepthAxis.Dims) != 4 {
		t.Fatalf("depth dims = %d, want 4", len(rep.DepthAxis.Dims))
	}
	if rep.DepthAxis.Subtotal != 11 {
		t.Fatalf("subtotal = %d, want 11 (3+2+3+3)", rep.DepthAxis.Subtotal)
	}
	if rep.DepthAxis.Dims[0].Name != "任务理解与问题表述" {
		t.Fatalf("D1 name not filled from rubric: %q", rep.DepthAxis.Dims[0].Name)
	}
	if rep.Axiom != rubric.Model().Axiom {
		t.Fatalf("axiom not set from config: %q", rep.Axiom)
	}
	if rep.AutonomyAxis.Code != "D2" || rep.CrossAxis.Code != "D6" {
		t.Fatalf("axis codes = %q/%q, want D2/D6", rep.AutonomyAxis.Code, rep.CrossAxis.Code)
	}
}

func TestAssessReportSubtotalEqualsSumAndClamps(t *testing.T) {
	// score 5 clamps to 3; missing D5 → 0. subtotal = 3+0+3+0 = 6.
	reply := `{"depthAxis":{"dims":[
		{"code":"D1","score":5,"evidence":"x","promptEvidence":""},
		{"code":"D4","score":3,"evidence":"x","promptEvidence":""}]},
		"autonomyAxis":{"observation":"o"},"crossAxis":{"depthLevel":"NA"},
		"narrative":"n"}`
	rep, _, err := AssessReport(context.Background(), reportProvider(reply),
		gateway.Resolved{Tier: "flagship"}, rubric.Model(), AssessmentInput{})
	if err != nil {
		t.Fatalf("AssessReport: %v", err)
	}
	sum := 0
	for _, d := range rep.DepthAxis.Dims {
		if d.Score < 0 || d.Score > 3 {
			t.Fatalf("dim %s score %d out of range", d.Code, d.Score)
		}
		sum += d.Score
	}
	if rep.DepthAxis.Subtotal != sum {
		t.Fatalf("subtotal %d != Σ scores %d", rep.DepthAxis.Subtotal, sum)
	}
	if rep.DepthAxis.Subtotal != 6 {
		t.Fatalf("subtotal = %d, want 6", rep.DepthAxis.Subtotal)
	}
}

func TestAssessReportBannedPhrasingRejects(t *testing.T) {
	reply := `{"depthAxis":{"dims":[{"code":"D1","score":2,"evidence":"你应该这样写：先摆结论"}]},
		"autonomyAxis":{"observation":"o"},"crossAxis":{"depthLevel":"L2"},"narrative":"n"}`
	_, _, err := AssessReport(context.Background(), reportProvider(reply),
		gateway.Resolved{Tier: "flagship"}, rubric.Model(), AssessmentInput{})
	if err == nil {
		t.Fatalf("expected banned-phrasing rejection")
	}
}

func TestAssessReportPromptCarriesLaddersAndAxiom(t *testing.T) {
	sys := assessReportSystemPrompt(rubric.Model())
	for _, want := range []string{"认知深度", "智识自主", "两轴永不合成总分", "不排名", "SOLO"} {
		if !strings.Contains(sys, want) {
			t.Fatalf("system prompt missing %q", want)
		}
	}
}
