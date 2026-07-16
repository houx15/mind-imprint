package agent

import (
	"context"
	"strings"
	"testing"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/skills"
)

// reviewProvider returns a canned model reply, reusing the same
// gateway.NewStubProvider fake pattern already used by coach_test.go /
// anchors_test.go (scriptedProvider) — no new provider interface. Returns the
// concrete *gateway.StubProvider (which still satisfies gateway.Provider) so
// callers can inspect .LastRequest after ProposeReview runs.
func reviewProvider(reply string) *gateway.StubProvider {
	return gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: reply},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 42, OutputTokens: 17}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
}

func TestProposeReview_ParsesWorkOrder(t *testing.T) {
	reply := `[
      {"criterion_code":"表E","band":"5–6 段","evidence":"第2段接住反方","missing":"变绿→可持续的跳步没补","fix":"补上可持续的定义"},
      {"criterion_code":"表H","band":"7–8 段","evidence":"结构清楚","missing":"","fix":""}
    ]`
	criteria := []skills.ReviewCriterion{{Code: "表E", Name: "分析"}, {Code: "表H", Name: "表达与组织"}}
	items, usage, err := ProposeReview(context.Background(), reviewProvider(reply), gateway.Resolved{Provider: "deepseek", Model: "x"}, criteria, []string{"p1", "p2"}, "claims:1 evidence:2", VoiceBoard, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[0].CriterionCode != "表E" || items[0].CriterionName != "分析" || items[0].Fix == "" {
		t.Fatalf("item0 = %+v", items[0])
	}
	_ = usage
}

func TestProposeReview_RejectsBannedPhrase(t *testing.T) {
	// A reply whose fix rewrites the student's sentence for her — must be
	// rejected by the enforcement stack (banned-phrasing), not returned.
	// Exercised under a generic voice too, to prove banned-phrasing rejection
	// is voice-independent.
	reply := `[{"criterion_code":"表E","band":"5–6 段","evidence":"e","missing":"m","fix":"你应该这样写：中国的转型是叠加式的。"}]`
	criteria := []skills.ReviewCriterion{{Code: "表E", Name: "分析"}}
	_, _, err := ProposeReview(context.Background(), reviewProvider(reply), gateway.Resolved{Provider: "deepseek", Model: "x"}, criteria, []string{"p1"}, "", VoiceSceptic, false)
	if err == nil {
		t.Fatal("expected banned-phrasing rejection, got nil")
	}
	_ = strings.TrimSpace
}

func TestParseVoice(t *testing.T) {
	cases := map[string]Voice{
		"board": VoiceBoard, "sceptic": VoiceSceptic, "layperson": VoiceLayperson,
		"executioner": VoiceExecutioner, "": VoiceBoard, "nonsense": VoiceBoard, "BOARD": VoiceBoard,
	}
	for in, want := range cases {
		if got := ParseVoice(in); got != want {
			t.Errorf("ParseVoice(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestReviewSystemPrompt_DistinctPerVoice(t *testing.T) {
	board := reviewSystemPrompt(VoiceBoard, false)
	// Slice 9 T2 appends a voice-invariant points instruction to every voice,
	// so board is no longer byte-identical to reviewPosturePrompt — but it
	// must still be built ON TOP OF the unmodified board posture (never
	// swapped for one of the three generic postures).
	if !strings.HasPrefix(board, reviewPosturePrompt) {
		t.Fatal("board voice must be built on the existing reviewPosturePrompt verbatim")
	}
	seen := map[string]bool{}
	for _, v := range []Voice{VoiceBoard, VoiceSceptic, VoiceLayperson, VoiceExecutioner} {
		p := reviewSystemPrompt(v, false)
		if seen[p] {
			t.Fatalf("voice %q produced a duplicate posture", v)
		}
		seen[p] = true
		// Every voice keeps the RL-1 iron rule (never rewrite / never a model sentence).
		if !strings.Contains(p, "绝不") {
			t.Fatalf("voice %q dropped the iron rule", v)
		}
	}
}

func TestReviewSystemPrompt_OverBudgetAppendsDeletionLens(t *testing.T) {
	base := reviewSystemPrompt(VoiceExecutioner, false)
	over := reviewSystemPrompt(VoiceExecutioner, true)
	if base == over {
		t.Fatal("overBudget must append a deletion-lens instruction")
	}
	if !strings.Contains(over, "删减") || !strings.Contains(over, "哪张表") {
		t.Fatalf("over-budget posture missing the deletion-lens frame: %s", over)
	}
}

func TestReviewSystemPromptAsksForPoints_AllVoices(t *testing.T) {
	for _, v := range []Voice{VoiceBoard, VoiceSceptic, VoiceLayperson, VoiceExecutioner} {
		p := reviewSystemPrompt(v, false)
		if !strings.Contains(p, "points") {
			t.Fatalf("voice %s: prompt missing points instruction", v)
		}
	}
}

func TestProposeReviewParsesPoints(t *testing.T) {
	prov := reviewProvider(`[{"criterion_code":"表D","band":"到达 identify","evidence":"有一手源","missing":"孤儿证据没接上","fix":"把它接到主张","points":3}]`)
	criteria := []skills.ReviewCriterion{{Code: "表D", Name: "来源与证据", Points: 4}}
	items, _, err := ProposeReview(context.Background(), prov, gateway.Resolved{}, criteria,
		[]string{"第一段"}, "摘要", VoiceBoard, false)
	if err != nil {
		t.Fatalf("propose: %v", err)
	}
	if len(items) != 1 || items[0].Points != 3 {
		t.Fatalf("want points 3, got %+v", items)
	}
}

func TestProposeReviewPromptCarriesCriterionTotal(t *testing.T) {
	prov := reviewProvider(`[{"criterion_code":"表D","band":"b","evidence":"e","missing":"m","fix":"f","points":2}]`)
	criteria := []skills.ReviewCriterion{{Code: "表D", Name: "来源与证据", Points: 4}}
	if _, _, err := ProposeReview(context.Background(), prov, gateway.Resolved{}, criteria,
		[]string{"第一段"}, "摘要", VoiceBoard, false); err != nil {
		t.Fatalf("propose: %v", err)
	}
	lastMsg := prov.LastRequest.Messages[len(prov.LastRequest.Messages)-1]
	if !strings.Contains(lastMsg.Content, "4") {
		t.Fatalf("user prompt should carry the table total 4; got %q", lastMsg.Content)
	}
}
