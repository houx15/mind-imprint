package gateway

import (
	"context"
	"strings"
	"testing"
)

// Thinking must never reach the reply body, because the reply body is what gets
// stored, refed into the next turn, and evaluated. A model that reasons out loud
// into Text would put chain-of-thought into the student's transcript, into the
// process tree, and into 过程评估's input — silently, and only for the models
// that happen to stream reasoning_content.
//
// This is a product rule from the owner (2026-09-03: "thinking content should
// not be added to context"), and it is invisible in a code read: both strings
// arrive on the same channel, one switch case apart.
func TestReasoningNeverReachesTheReplyBody(t *testing.T) {
	p := NewStubProvider([]StreamEvent{
		{Kind: EventReasoningDelta, TextDelta: "学生说的是装机量，"},
		{Kind: EventReasoningDelta, TextDelta: "但她没说衡量的是什么。"},
		{Kind: EventTextDelta, TextDelta: "你说的「装机量」，"},
		{Kind: EventTextDelta, TextDelta: "衡量的是能力还是结果？"},
		{Kind: EventDone, StopReason: StopStop},
	})

	res, err := Collect(context.Background(), p, Resolved{}, ChatRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(res.Text, "但她没说") {
		t.Fatalf("reasoning leaked into the reply body: %q", res.Text)
	}
	if res.Text != "你说的「装机量」，衡量的是能力还是结果？" {
		t.Errorf("reply body = %q", res.Text)
	}
	if !strings.Contains(res.Reasoning, "但她没说") {
		t.Errorf("reasoning must still be available for the fold, got %q", res.Reasoning)
	}
}
