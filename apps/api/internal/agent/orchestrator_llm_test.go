package agent

import (
	"context"
	"testing"

	"mindimprint/api/internal/gateway"
)

// newStubProvider returns a StubProvider that replays a single canned model
// text response, followed by usage + done — the standard single-shot stub
// pattern used across internal/gateway tests.
func newStubProvider(text string) gateway.Provider {
	return gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: text},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 10, OutputTokens: 2}},
		{Kind: gateway.EventDone},
	})
}

func stubResolved() gateway.Resolved {
	return gateway.Resolved{Provider: "fake", Model: "m"}
}

func TestProposeOrchestratorTurn_ParsesToolTurn(t *testing.T) {
	prov := newStubProvider(`{"narrate":"先聊聊你的问题。","tools":[{"name":"set_status","args":{"stage":"proposal_forming"}}]}`)
	dec, usage, err := ProposeOrchestratorTurn(context.Background(), prov, stubResolved(), "SPINE", DefaultStudioState(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if dec.Narrate == "" || len(dec.Tools) != 1 {
		t.Fatalf("bad decision: %+v", dec)
	}
	if usage.InputTokens == 0 && usage.OutputTokens == 0 {
		t.Fatalf("expected usage to be populated from the stub, got %+v", usage)
	}
}

func TestProposeOrchestratorTurn_RetriesOnParseError(t *testing.T) {
	// SequenceStubProvider (internal/gateway/stub.go) replays a DIFFERENT
	// script per Stream call — exactly the retry shape this test needs: the
	// first attempt returns malformed JSON, the second returns a valid
	// {narrate,tools} payload.
	prov := gateway.NewSequenceStubProvider(
		[]gateway.StreamEvent{
			{Kind: gateway.EventTextDelta, TextDelta: `not json at all`},
			{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 5, OutputTokens: 1}},
			{Kind: gateway.EventDone},
		},
		[]gateway.StreamEvent{
			{Kind: gateway.EventTextDelta, TextDelta: `{"narrate":"再说说你的想法。","tools":[]}`},
			{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 7, OutputTokens: 3}},
			{Kind: gateway.EventDone},
		},
	)

	dec, usage, err := ProposeOrchestratorTurn(context.Background(), prov, stubResolved(), "SPINE", DefaultStudioState(), nil)
	if err != nil {
		t.Fatalf("expected the retry to succeed, got err: %v", err)
	}
	if dec.Narrate != "再说说你的想法。" {
		t.Fatalf("expected the second attempt's narration, got %q", dec.Narrate)
	}
	if prov.Calls != 2 {
		t.Fatalf("expected exactly 2 attempts, got %d", prov.Calls)
	}
	// Whole-branch review Fix 3: attempt 0 still cost real tokens even though
	// its output failed to parse — the returned usage must be the SUM across
	// both attempts (5+7, 1+3), not just the successful second attempt's, else
	// attempt 0's spend goes unmetered.
	if usage.InputTokens != 12 || usage.OutputTokens != 4 {
		t.Fatalf("expected usage summed across both attempts (12,4), got %+v", usage)
	}
}
