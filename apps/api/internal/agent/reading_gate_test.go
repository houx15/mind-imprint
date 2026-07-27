package agent

import "testing"

func summon(card string) ReadingDecision {
	return ReadingDecision{Decision: "summon", CardID: card, ExampleBlockID: "b1", ExampleQuote: "因此它必然失败"}
}

func TestApplyReadingGate_OpenCardSuppressesNewSummon(t *testing.T) {
	got := ApplyReadingGate(summon("argument-map"), PacingState{OpenCard: true}, OrderingGuard{AllowCraap: true, AllowSift: true})
	if got.Decision != "respond" {
		t.Fatalf("open card must suppress summon, got %+v", got)
	}
}

func TestApplyReadingGate_BreathingRoomDowngradesSummonToHint(t *testing.T) {
	got := ApplyReadingGate(summon("framing"), PacingState{TurnsSinceLastPropose: 1}, OrderingGuard{})
	if got.Decision != "hint" {
		t.Fatalf("within breathing window summon→hint, got %+v", got)
	}
}

func TestApplyReadingGate_SkipCooldownSuppresses(t *testing.T) {
	got := ApplyReadingGate(summon("cda"), PacingState{RecentlySkipped: []string{"cda"}}, OrderingGuard{})
	if got.Decision != "respond" {
		t.Fatalf("recently-skipped card must be suppressed, got %+v", got)
	}
}

func TestApplyReadingGate_CompletedWithoutNewFocusSuppresses(t *testing.T) {
	got := ApplyReadingGate(summon("toulmin"), PacingState{CompletedCards: []string{"toulmin"}, HasNewFocus: false}, OrderingGuard{})
	if got.Decision != "respond" {
		t.Fatalf("completed card without new focus must be suppressed, got %+v", got)
	}
}

func TestApplyReadingGate_OrderingBlocksSiftBeforeCraap(t *testing.T) {
	got := ApplyReadingGate(summon("sift"), PacingState{}, OrderingGuard{AllowCraap: true, AllowSift: false})
	if got.Decision != "respond" {
		t.Fatalf("sift not allowed yet must be suppressed, got %+v", got)
	}
}

func TestApplyReadingGate_AllowsCleanSummon(t *testing.T) {
	got := ApplyReadingGate(summon("argument-map"), PacingState{TurnsSinceLastPropose: 5, HasNewFocus: true}, OrderingGuard{AllowCraap: true, AllowSift: true})
	if got.Decision != "summon" {
		t.Fatalf("clean summon must pass, got %+v", got)
	}
}

func TestResolveExampleAnchor_RejectsNonVerbatimQuote(t *testing.T) {
	blocks := []MaterialBlock{{ID: "b1", Text: "气候在变化。因此这项政策必然失败。"}}
	_, ok := ResolveExampleAnchor(ReadingDecision{Decision: "summon", ExampleBlockID: "b1", ExampleQuote: "这句不在原文里"}, "m1", blocks)
	if ok {
		t.Fatalf("non-verbatim quote must be rejected")
	}
}

func TestResolveExampleAnchor_ComputesRuneOffsets(t *testing.T) {
	blocks := []MaterialBlock{{ID: "b1", Text: "气候在变化。因此这项政策必然失败。"}}
	a, ok := ResolveExampleAnchor(ReadingDecision{Decision: "summon", CardID: "argument-map", ExampleBlockID: "b1", ExampleQuote: "因此这项政策必然失败", ExampleWhy: "用了必然"}, "m1", blocks)
	if !ok {
		t.Fatalf("verbatim quote must resolve")
	}
	if a.Start != 6 || a.End != 6+len([]rune("因此这项政策必然失败")) {
		t.Fatalf("rune offsets wrong: start=%d end=%d", a.Start, a.End)
	}
	if a.Author != "ai" || a.MaterialID != "m1" || a.BlockID != "b1" || a.Question == "" {
		t.Fatalf("anchor shape wrong: %+v", a)
	}
}
