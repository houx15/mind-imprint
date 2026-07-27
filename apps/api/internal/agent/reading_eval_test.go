package agent

import (
	"context"
	"testing"

	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
)

func TestVerdictFromChecks_Rules(t *testing.T) {
	miss := []SelectionCheck{{Key: "target", Status: "miss"}, {Key: "evidence", Status: "pass"}, {Key: "centrality", Status: "pass"}}
	if v, l := verdictFromChecks(miss); v != "rethink" || l != "暂不匹配" {
		t.Fatalf("target miss → rethink, got %s/%s", v, l)
	}
	allPass := []SelectionCheck{{Key: "target", Status: "pass"}, {Key: "evidence", Status: "pass"}, {Key: "centrality", Status: "pass"}}
	if v, l := verdictFromChecks(allPass); v != "strong" || l != "高度匹配" {
		t.Fatalf("all pass → strong, got %s/%s", v, l)
	}
	mixed := []SelectionCheck{{Key: "target", Status: "pass"}, {Key: "evidence", Status: "partial"}, {Key: "centrality", Status: "pass"}}
	if v, l := verdictFromChecks(mixed); v != "partial" || l != "部分匹配" {
		t.Fatalf("mixed → partial, got %s/%s", v, l)
	}
}

func TestEvaluateSelection_VerdictIsProgramOwned_NotModel(t *testing.T) {
	// Model lies: claims "strong" while target is a miss. Program must override.
	script := []gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: `{"verdict":"strong",` +
			`"checks":[{"key":"target","status":"miss","evidence":"","explanation":"选错了对象"},` +
			`{"key":"evidence","status":"pass","evidence":"必然失败","explanation":"有强词"},` +
			`{"key":"centrality","status":"pass","evidence":"必然失败","explanation":"是关键"}],` +
			`"finding":"这是一个没给证据的结论","judgment":"论证跳跃","support":"用了必然","caveat":"","next_step":"找找它的证据"}`},
		{Kind: gateway.EventDone},
	}
	p := gateway.NewStubProvider(script)
	span := Anchor{ID: "s0", Quote: "因此这项政策必然失败", Dimension: "logic"}
	ev, _, _, err := EvaluateSelection(context.Background(), p, readingStubResolver(), cards.Spec{ID: "argument-map", Name: "论证地图"}, "logic", span)
	if err != nil {
		t.Fatalf("EvaluateSelection error: %v", err)
	}
	if ev.Verdict != "rethink" {
		t.Fatalf("program must override model's lie to rethink, got %q", ev.Verdict)
	}
	if ev.NextStep != "找找它的证据" {
		t.Fatalf("next_step must be parsed from the model reply, got %q", ev.NextStep)
	}
	if len(ev.SpanIDs) != 1 || ev.SpanIDs[0] != "s0" {
		t.Fatalf("finding must cite the student's span only, got %+v", ev.SpanIDs)
	}
}

func TestEvaluateSelection_DropsNonVerbatimEvidenceSnippet(t *testing.T) {
	script := []gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: `{"checks":[` +
			`{"key":"target","status":"pass","evidence":"因此这项政策必然失败","explanation":"对"},` +
			`{"key":"evidence","status":"pass","evidence":"这段文字并不在学生选的句子里","explanation":"x"},` +
			`{"key":"centrality","status":"pass","evidence":"必然失败","explanation":"关键"}],` +
			`"finding":"f","judgment":"j","support":"s","caveat":"","next_step":"n"}`},
		{Kind: gateway.EventDone},
	}
	p := gateway.NewStubProvider(script)
	span := Anchor{ID: "s0", Quote: "因此这项政策必然失败"}
	ev, _, _, _ := EvaluateSelection(context.Background(), p, readingStubResolver(), cards.Spec{ID: "x"}, "d", span)
	for _, c := range ev.Checks {
		if c.Key == "evidence" && c.Evidence != "" {
			t.Fatalf("non-verbatim evidence snippet must be dropped, got %q", c.Evidence)
		}
	}
}

func TestEvaluateSelection_DeterministicFallbackOnGarbage(t *testing.T) {
	p := gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: "garbage"},
		{Kind: gateway.EventDone},
	})
	span := Anchor{ID: "s0", Quote: "因此这项政策必然失败"}
	ev, _, _, err := EvaluateSelection(context.Background(), p, readingStubResolver(), cards.Spec{ID: "x"}, "d", span)
	if err != nil {
		t.Fatalf("garbage must not error (deterministic fallback keeps the loop alive): %v", err)
	}
	if len(ev.Checks) != 3 || ev.Verdict == "" {
		t.Fatalf("fallback must still produce 3 checks + a verdict, got %+v", ev)
	}
	if len(ev.SpanIDs) != 1 || ev.SpanIDs[0] != "s0" {
		t.Fatalf("fallback must still cite the student's span, got %+v", ev.SpanIDs)
	}
}
