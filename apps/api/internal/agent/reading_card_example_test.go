package agent

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"

	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
)

func cardExampleSpec(t *testing.T) cards.Spec {
	t.Helper()
	spec, ok := cards.ByID("argument-map")
	if !ok {
		t.Fatalf("argument-map card not found in registry")
	}
	return spec
}

func TestProposeCardExample_ValidReplyReturnsAnchor(t *testing.T) {
	blocks := []MaterialBlock{
		{ID: "b0", Text: "全球变暖正在加速冰川融化，科学家在南极观测到前所未有的冰架断裂。"},
	}
	script := []gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: `{"block_id":"b0","quote":"科学家在南极观测到前所未有的冰架断裂","why":"这是文章给出的具体证据。"}`},
		{Kind: gateway.EventDone},
	}
	p := gateway.NewStubProvider(script)
	anchor, resolved, _, ok := ProposeCardExample(context.Background(), p, readingStubResolver(), cardExampleSpec(t), "mat-1", blocks)
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if resolved.Provider != "deepseek" {
		t.Fatalf("expected resolved provider, got %+v", resolved)
	}
	if anchor.MaterialID != "mat-1" || anchor.BlockID != "b0" || anchor.Quote != "科学家在南极观测到前所未有的冰架断裂" {
		t.Fatalf("unexpected anchor: %+v", anchor)
	}
	wantStart := utf8.RuneCountInString(strings.SplitN(blocks[0].Text, anchor.Quote, 2)[0])
	if anchor.Start != wantStart {
		t.Fatalf("unexpected start offset: got %d, want %d (rune offset into block text)", anchor.Start, wantStart)
	}
	if anchor.End-anchor.Start != utf8.RuneCountInString(anchor.Quote) {
		t.Fatalf("expected (start,end) to span exactly the quote, got (%d,%d) for quote len %d",
			anchor.Start, anchor.End, utf8.RuneCountInString(anchor.Quote))
	}
	if anchor.Dimension != "argument-map" || anchor.Author != "ai" {
		t.Fatalf("unexpected dimension/author: %+v", anchor)
	}
	if anchor.Question != "这是文章给出的具体证据。" {
		t.Fatalf("unexpected question/why: %q", anchor.Question)
	}
}

func TestProposeCardExample_NonVerbatimQuoteFails(t *testing.T) {
	blocks := []MaterialBlock{
		{ID: "b0", Text: "全球变暖正在加速冰川融化，科学家在南极观测到前所未有的冰架断裂。"},
	}
	script := []gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: `{"block_id":"b0","quote":"这句话不在原文里","why":"随便编的"}`},
		{Kind: gateway.EventDone},
	}
	p := gateway.NewStubProvider(script)
	_, resolved, _, ok := ProposeCardExample(context.Background(), p, readingStubResolver(), cardExampleSpec(t), "mat-1", blocks)
	if ok {
		t.Fatalf("expected ok=false for a non-verbatim quote")
	}
	// The call still cost money — resolved must still carry the provider so
	// the caller can record it.
	if resolved.Provider != "deepseek" {
		t.Fatalf("expected resolved provider to still be attached on a parse-level failure, got %+v", resolved)
	}
}

func TestProposeCardExample_GarbageReplyFails(t *testing.T) {
	blocks := []MaterialBlock{
		{ID: "b0", Text: "全球变暖正在加速冰川融化。"},
	}
	p := gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: "not json at all"},
		{Kind: gateway.EventDone},
	})
	anchor, _, _, ok := ProposeCardExample(context.Background(), p, readingStubResolver(), cardExampleSpec(t), "mat-1", blocks)
	if ok {
		t.Fatalf("expected ok=false for garbage, got anchor %+v", anchor)
	}
}

func TestProposeCardExample_ResolverErrorFails(t *testing.T) {
	blocks := []MaterialBlock{{ID: "b0", Text: "x"}}
	badResolver := gateway.KeyResolver(func(ctx context.Context) (gateway.Resolved, error) {
		return gateway.Resolved{}, context.DeadlineExceeded
	})
	_, resolved, _, ok := ProposeCardExample(context.Background(), gateway.NewStubProvider(nil), badResolver, cardExampleSpec(t), "mat-1", blocks)
	if ok {
		t.Fatalf("expected ok=false on resolver error")
	}
	if resolved.Provider != "" {
		t.Fatalf("expected no resolved on resolver error, got %+v", resolved)
	}
}
