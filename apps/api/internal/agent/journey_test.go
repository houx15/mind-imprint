package agent

import (
	"context"
	"testing"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/skills"
)

// fakeProviderReturning builds a scripted gateway.Provider (modelled on
// scriptedProvider in coach_test.go) plus a gateway.KeyResolver that returns a
// non-empty gateway.Resolved, for tests that need to drive ComposeJourney
// through a resolver rather than a fixed Resolved value.
func fakeProviderReturning(text string) (gateway.Provider, gateway.KeyResolver) {
	provider := gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: text},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 42, OutputTokens: 17}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
	resolver := func(ctx context.Context) (gateway.Resolved, error) {
		return gateway.Resolved{Provider: "fake", Model: "fake-model", Tier: "coach"}, nil
	}
	return provider, resolver
}

func TestComposeJourneyWaivesUnkeptContracts(t *testing.T) {
	sk, _ := skills.ByID("writing-project")
	// Fake provider returns a decision list keeping only S3..S6.
	reply := `[
	  {"id":"decode_task","keep":false,"reason":"她已解码任务"},
	  {"id":"frame_question","keep":false,"reason":"题目已定"},
	  {"id":"evaluate_perspectives","keep":false,"reason":"视角已想过"},
	  {"id":"evaluate_sources","keep":true,"reason":""},
	  {"id":"build_argument","keep":true,"reason":""},
	  {"id":"draft_polish","keep":true,"reason":""},
	  {"id":"reflect_archive","keep":true,"reason":""}]`
	provider, resolver := fakeProviderReturning(reply) // package helper
	res := ComposeJourney(context.Background(), provider, resolver, sk, "（她贴了一篇写了一半的草稿）")
	got := map[string]bool{}
	for _, id := range res.Waived {
		got[id] = true
	}
	for _, id := range []string{"decode_task", "frame_question", "evaluate_perspectives"} {
		if !got[id] {
			t.Fatalf("expected %s waived, got %v", id, res.Waived)
		}
	}
	if len(res.Waived) != 3 {
		t.Fatalf("expected exactly 3 waived, got %v", res.Waived)
	}
	if res.Resolved.Provider == "" {
		t.Fatalf("a real call happened; Resolved must be populated for metering")
	}
}

func TestComposeJourneyFailSafeToFullJourney(t *testing.T) {
	sk, _ := skills.ByID("writing-project")
	for name, reply := range map[string]string{
		"malformed":     "not json at all",
		"empty":         "[]",
		"unknown_id":    `[{"id":"not_a_contract","keep":false,"reason":"x"}]`,
		"partial_cover": `[{"id":"decode_task","keep":false,"reason":"x"}]`, // must cover ALL contracts
	} {
		t.Run(name, func(t *testing.T) {
			provider, resolver := fakeProviderReturning(reply)
			res := ComposeJourney(context.Background(), provider, resolver, sk, "题目")
			if len(res.Waived) != 0 {
				t.Fatalf("%s: fail-safe must waive nothing, got %v", name, res.Waived)
			}
			if res.Resolved.Provider == "" {
				t.Fatalf("%s: a real call happened; must still be meterable", name)
			}
		})
	}
}

func TestComposeJourneyAllKeepIsFullJourneyNotFailure(t *testing.T) {
	sk, _ := skills.ByID("writing-project")
	// Well-formed, covers all, keeps all → empty waived-set, still a success.
	reply := `[
	  {"id":"decode_task","keep":true,"reason":""},
	  {"id":"frame_question","keep":true,"reason":""},
	  {"id":"evaluate_perspectives","keep":true,"reason":""},
	  {"id":"evaluate_sources","keep":true,"reason":""},
	  {"id":"build_argument","keep":true,"reason":""},
	  {"id":"draft_polish","keep":true,"reason":""},
	  {"id":"reflect_archive","keep":true,"reason":""}]`
	provider, resolver := fakeProviderReturning(reply)
	res := ComposeJourney(context.Background(), provider, resolver, sk, "题目")
	if len(res.Waived) != 0 {
		t.Fatalf("all-keep should yield empty waived-set, got %v", res.Waived)
	}
	if len(res.Decisions) != 7 {
		t.Fatalf("expected 7 decisions recorded, got %d", len(res.Decisions))
	}
}

func TestComposeJourneyAllWaivedDegradesToFullJourney(t *testing.T) {
	sk, _ := skills.ByID("writing-project")
	// Well-formed, covers all, but claims every contract is already done — not
	// a credible verdict, so it must degrade to the full journey.
	reply := `[
	  {"id":"decode_task","keep":false,"reason":""},
	  {"id":"frame_question","keep":false,"reason":""},
	  {"id":"evaluate_perspectives","keep":false,"reason":""},
	  {"id":"evaluate_sources","keep":false,"reason":""},
	  {"id":"build_argument","keep":false,"reason":""},
	  {"id":"draft_polish","keep":false,"reason":""},
	  {"id":"reflect_archive","keep":false,"reason":""}]`
	provider, resolver := fakeProviderReturning(reply)
	res := ComposeJourney(context.Background(), provider, resolver, sk, "题目")
	if len(res.Waived) != 0 {
		t.Fatalf("all-waived must degrade to full journey, got %v", res.Waived)
	}
	if res.Resolved.Provider == "" {
		t.Fatalf("a real call happened; Resolved must still be populated for metering")
	}
}
