package agent

import (
	"context"
	"strings"
	"testing"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/rubric"
)

func assessProvider(reply string) *gateway.StubProvider {
	return gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: reply},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 50, OutputTokens: 30}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
}

func TestAssessParsesScoresAndNarrative(t *testing.T) {
	reply := `{"dimensions":[{"code":"D2","level":"L4","evidence":"交叉验证两个一手源"}],"narrative":"你这次最大的跃迁在 S3。"}`
	in := BuildAssessmentInput(nil, []CardUse{{CardID: "sift", Dimension: "D3", Spont: "自发"}}, nil, nil, []int{1780}, nil, "claims:1", nil)
	a, usage, err := Assess(context.Background(), assessProvider(reply), gateway.Resolved{Provider: "deepseek", Model: "x"}, rubric.CT(), in, EmbeddedAnchors())
	if err != nil {
		t.Fatal(err)
	}
	if usage.OutputTokens != 30 {
		t.Errorf("usage not returned")
	}
	// Every rubric dimension is represented, in order; D2 lit, the rest NA.
	if len(a.Dimensions) != 10 || a.Dimensions[0].Code != "D1" {
		t.Fatalf("want 10 dims in order, got %d", len(a.Dimensions))
	}
	byCode := map[string]DimensionScore{}
	for _, d := range a.Dimensions {
		byCode[d.Code] = d
	}
	if byCode["D2"].Level != "L4" || byCode["D2"].Name != "信源辨识" {
		t.Errorf("D2 not scored/named: %+v", byCode["D2"])
	}
	if byCode["D5"].Level != "NA" {
		t.Errorf("unscored dim should be NA, got %q", byCode["D5"].Level)
	}
	if a.Narrative == "" {
		t.Errorf("narrative dropped")
	}
}

func TestAssessCoercesUnknownLevelToNA(t *testing.T) {
	reply := `{"dimensions":[{"code":"D1","level":"卓越","evidence":"x"}],"narrative":"n"}`
	a, _, err := Assess(context.Background(), assessProvider(reply), gateway.Resolved{}, rubric.CT(), AssessmentInput{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range a.Dimensions {
		if d.Code == "D1" {
			found = true
			if d.Level != "NA" {
				t.Errorf("unknown level should coerce to NA, got %q", d.Level)
			}
		}
	}
	if !found {
		t.Fatal("D1 not present in output")
	}
}

func TestAssessRejectsBannedPhrasingWholeReport(t *testing.T) {
	// A narrative containing a ghostwriting imperative must reject the WHOLE report.
	reply := `{"dimensions":[{"code":"D1","level":"L2","evidence":"e"}],"narrative":"你应该这样写：中国在可持续发展上……"}`
	_, _, err := Assess(context.Background(), assessProvider(reply), gateway.Resolved{}, rubric.CT(), AssessmentInput{}, nil)
	if err == nil {
		t.Fatal("want banned-phrasing rejection, got nil")
	}
}

func TestAssessPromptCarriesRubricAndAnchors(t *testing.T) {
	// Prove the prompt includes a rubric ladder anchor + an anchor-sample name.
	rb := rubric.CT()
	anchors := EmbeddedAnchors()
	if len(anchors) == 0 {
		t.Fatal("expected embedded anchor fixture, got none")
	}
	sys := assessSystemPrompt(rb, anchors)

	// A known D-anchor substring from the rubric ladder (D2 L4 anchor).
	if !strings.Contains(sys, "主动交叉验证，识别信源之间的利益关系与冲突") {
		t.Errorf("system prompt missing rubric ladder anchor text: %s", sys)
	}
	// An anchor sample name from the few-shot fixture.
	if !strings.Contains(sys, anchors[0].Name) {
		t.Errorf("system prompt missing anchor sample name %q", anchors[0].Name)
	}
	// RL-5: diagnostic posture, not a grade/rank.
	if !strings.Contains(sys, "不排名") {
		t.Errorf("system prompt missing RL-5 no-rank posture")
	}
}
