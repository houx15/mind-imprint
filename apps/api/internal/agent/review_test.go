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

// TestProposeReview_FencedReplyOnSingleParagraph_item10 — item #10's root-cause
// reproduction. review.go's json.Unmarshal ran on a bare strings.TrimSpace,
// unlike ~10 other agent/*.go call sites (anchors.go, course.go, formingdim.go,
// reading_router.go, reading_takeaway.go, ai_use.go, exploration_guide.go,
// reading_eval.go, reading_card_example.go, journey.go) which already call the
// shared stripFences() first. A reasoning model asked to output "只输出 JSON
// 数组" still occasionally wraps the array in ```json fences — most plausible
// on an unusual, tiny scope like a single selected paragraph checked against
// the FULL essay rubric, where the model has little to work with and is more
// prone to hedge/wrap instead of emitting the bare array. Before this fix that
// reply failed json.Unmarshal outright → ProposeReview returned an error →
// order_review's perr branch → client saw the blanket "review_rejected" /
// "体检没跑完，稍后再试一次" — for ANY 体检 (not exclusively paragraph-scoped,
// but most likely to bite there). This reply now parses cleanly.
func TestProposeReview_FencedReplyOnSingleParagraph_item10(t *testing.T) {
	fenced := "```json\n" + `[{"criterion_code":"表E","band":"5–6 段","evidence":"e","missing":"m","fix":"f"}]` + "\n```"
	criteria := []skills.ReviewCriterion{{Code: "表E", Name: "分析"}}
	// A single short paragraph — the 体检这段 scope, not a whole multi-paragraph draft.
	items, _, err := ProposeReview(context.Background(), reviewProvider(fenced), gateway.Resolved{Provider: "deepseek", Model: "x"}, criteria, []string{"我的草稿第一段。"}, "", VoiceBoard, false)
	if err != nil {
		t.Fatalf("a ```json-fenced reply must parse via stripFences, got: %v", err)
	}
	if len(items) != 1 || items[0].CriterionCode != "表E" {
		t.Fatalf("items = %+v", items)
	}
}

func TestProposeReview_RetriesOnUnparseableReply(t *testing.T) {
	garbage := "抱歉，这段内容太短，我无法给出完整的评分表。" // no JSON array at all — a plausible short-paragraph hedge
	good := `[{"criterion_code":"表E","band":"5–6 段","evidence":"e","missing":"m","fix":"f"}]`
	p := gateway.NewSequenceStubProvider(
		[]gateway.StreamEvent{
			{Kind: gateway.EventTextDelta, TextDelta: garbage},
			{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 10, OutputTokens: 5}},
			{Kind: gateway.EventDone, StopReason: gateway.StopStop},
		},
		[]gateway.StreamEvent{
			{Kind: gateway.EventTextDelta, TextDelta: good},
			{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 12, OutputTokens: 6}},
			{Kind: gateway.EventDone, StopReason: gateway.StopStop},
		},
	)
	criteria := []skills.ReviewCriterion{{Code: "表E", Name: "分析"}}
	items, usage, err := ProposeReview(context.Background(), p, gateway.Resolved{}, criteria, []string{"我的草稿第一段。"}, "", VoiceBoard, false)
	if err != nil {
		t.Fatalf("propose: %v", err)
	}
	if p.Calls != 2 {
		t.Fatalf("an unparseable reply must retry exactly once (2 calls), got %d", p.Calls)
	}
	if len(items) != 1 {
		t.Fatalf("items = %+v", items)
	}
	// Usage accumulates across attempts — a rejected/retried call still cost money.
	if usage.InputTokens != 22 || usage.OutputTokens != 11 {
		t.Fatalf("usage must accumulate across attempts, got %+v", usage)
	}
}

func TestProposeReview_ExhaustsRetriesAndReturnsError(t *testing.T) {
	// Every attempt is garbage — the stub returns the same script every call.
	garbage := reviewProvider("not json at all")
	criteria := []skills.ReviewCriterion{{Code: "表E", Name: "分析"}}
	_, _, err := ProposeReview(context.Background(), garbage, gateway.Resolved{}, criteria, []string{"我的草稿第一段。"}, "", VoiceBoard, false)
	if err == nil {
		t.Fatal("expected an error once every attempt fails to parse")
	}
}

func TestProposeReview_BannedPhrasingDoesNotRetry(t *testing.T) {
	// A content decision, not a transient — must reject on the FIRST attempt,
	// never re-ask the model for the same (already-banned) content.
	bad := `[{"criterion_code":"表E","band":"5–6 段","evidence":"e","missing":"m","fix":"你应该这样写：中国的转型是叠加式的。"}]`
	good := `[{"criterion_code":"表E","band":"5–6 段","evidence":"e","missing":"m","fix":"f"}]`
	p := gateway.NewSequenceStubProvider(
		[]gateway.StreamEvent{
			{Kind: gateway.EventTextDelta, TextDelta: bad},
			{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 1, OutputTokens: 1}},
			{Kind: gateway.EventDone, StopReason: gateway.StopStop},
		},
		[]gateway.StreamEvent{
			{Kind: gateway.EventTextDelta, TextDelta: good},
			{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 1, OutputTokens: 1}},
			{Kind: gateway.EventDone, StopReason: gateway.StopStop},
		},
	)
	criteria := []skills.ReviewCriterion{{Code: "表E", Name: "分析"}}
	_, _, err := ProposeReview(context.Background(), p, gateway.Resolved{}, criteria, []string{"p1"}, "", VoiceBoard, false)
	if err == nil {
		t.Fatal("expected banned-phrasing rejection")
	}
	if p.Calls != 1 {
		t.Fatalf("a banned-phrasing rejection must NOT retry, got %d calls", p.Calls)
	}
}

func TestProposeReview_RetriesWhenNoCriterionCodeResolves(t *testing.T) {
	// Every item's criterion_code is unrecognized on attempt 1 (dropped), so
	// zero usable items survive — the same "transient, try again" class as an
	// unparseable reply, not a content decision.
	unresolvable := `[{"criterion_code":"完全不存在的表","band":"b","evidence":"e","missing":"m","fix":"f"}]`
	good := `[{"criterion_code":"表E","band":"b","evidence":"e","missing":"m","fix":"f"}]`
	p := gateway.NewSequenceStubProvider(
		[]gateway.StreamEvent{
			{Kind: gateway.EventTextDelta, TextDelta: unresolvable},
			{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 1, OutputTokens: 1}},
			{Kind: gateway.EventDone, StopReason: gateway.StopStop},
		},
		[]gateway.StreamEvent{
			{Kind: gateway.EventTextDelta, TextDelta: good},
			{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 1, OutputTokens: 1}},
			{Kind: gateway.EventDone, StopReason: gateway.StopStop},
		},
	)
	criteria := []skills.ReviewCriterion{{Code: "表E", Name: "分析"}}
	items, _, err := ProposeReview(context.Background(), p, gateway.Resolved{}, criteria, []string{"p1"}, "", VoiceBoard, false)
	if err != nil {
		t.Fatalf("propose: %v", err)
	}
	if p.Calls != 2 {
		t.Fatalf("zero usable items must retry once, got %d calls", p.Calls)
	}
	if len(items) != 1 || items[0].CriterionCode != "表E" {
		t.Fatalf("items = %+v", items)
	}
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
	if !strings.Contains(over, "删减") || !strings.Contains(over, "由学生决定") {
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

func TestReviewSystemPromptNamesAllFields_AllVoices(t *testing.T) {
	// Regression: the prompt must name band/evidence/fix explicitly or the model
	// silently omits them (observed live 2026-07-30 — only missing+points came back).
	for _, v := range []Voice{VoiceBoard, VoiceSceptic, VoiceLayperson, VoiceExecutioner} {
		p := reviewSystemPrompt(v, false)
		for _, key := range []string{"band", "evidence", "fix"} {
			if !strings.Contains(p, key) {
				t.Fatalf("voice %s: prompt must name required key %q", v, key)
			}
		}
	}
}

func TestProposeReviewFillsBandWhenModelOmits(t *testing.T) {
	// The model dropped band (empty) but gave points — the pill must not be blank.
	prov := reviewProvider(`[{"criterion_code":"表D","band":"","evidence":"e","missing":"m","fix":"f","points":3},{"criterion_code":"表D","band":"","evidence":"","missing":"m","fix":"f","points":0}]`)
	criteria := []skills.ReviewCriterion{{Code: "表D", Name: "来源与证据", Points: 4}}
	items, _, err := ProposeReview(context.Background(), prov, gateway.Resolved{}, criteria,
		[]string{"第一段"}, "摘要", VoiceBoard, false)
	if err != nil {
		t.Fatalf("propose: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("want 2 items, got %d", len(items))
	}
	if items[0].Band != "到第 3 分点" {
		t.Fatalf("band should fall back to the points cell, got %q", items[0].Band)
	}
	if items[1].Band != "尚未落点" {
		t.Fatalf("band for points=0 should be 尚未落点, got %q", items[1].Band)
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
	if !strings.Contains(lastMsg.Content, "共 4 分点") {
		t.Fatalf("user prompt should carry the table total as '共 4 分点'; got %q", lastMsg.Content)
	}
}
