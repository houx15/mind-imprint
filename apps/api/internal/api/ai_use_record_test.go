package api

import "testing"

import "mindimprint/api/internal/store/sqlc"

func TestBuildAIUseRecord_CountsAndAbsences(t *testing.T) {
	rec := buildAIUseRecord(
		[]sqlc.Event{
			{Type: "coach_turn"}, {Type: "coach_turn"},
			{Type: "coach_proposed"},
			{Type: "card_activated"},     // reading-loop open
			{Type: "card_logged"},        // coach/writing persist — the path CRITICAL missed
			{Type: "rabbit_hole_logged"}, // 兔子洞 card
			{Type: "coach_proposal_skipped"},
			{Type: "source_added"},  // a source brought into the library (counted)
			{Type: "source_opened"}, // reader open — NOT double-counted
			{Type: "gate_checked"},  // ignored
		},
		[]sqlc.LlmCall{{Purpose: "coach"}, {Purpose: "coach"}, {Purpose: "classify"}},
	)
	if rec.CoachTurns != 2 {
		t.Fatalf("CoachTurns = %d, want 2", rec.CoachTurns)
	}
	if rec.CardsProposed != 1 || rec.CardsAccepted != 3 || rec.CardsDismissed != 1 {
		t.Fatalf("cards = proposed %d accepted %d dismissed %d, want 1/3/1", rec.CardsProposed, rec.CardsAccepted, rec.CardsDismissed)
	}
	if rec.SourcesOpened != 1 {
		t.Fatalf("SourcesOpened = %d, want 1", rec.SourcesOpened)
	}
	if rec.LLMCallsByPurpose["coach"] != 2 || rec.LLMCallsByPurpose["classify"] != 1 {
		t.Fatalf("llm by purpose = %+v", rec.LLMCallsByPurpose)
	}
	if rec.GhostwroteEssay || rec.PredictedScore {
		t.Fatalf("absences must be false: ghostwrote=%v predicted=%v", rec.GhostwroteEssay, rec.PredictedScore)
	}
	if !hasInteractionRecord(rec) {
		t.Fatalf("a populated record must report hasInteractionRecord")
	}
}

func TestBuildAIUseRecord_EmptyHasNoRecord(t *testing.T) {
	if hasInteractionRecord(buildAIUseRecord(nil, nil)) {
		t.Fatalf("empty must have no interaction record")
	}
}
