package api

import "mindimprint/api/internal/store/sqlc"

// ai_use_record.go — S5 · the OBJECTIVE half of the AI-interaction retrospective.
// Assembled deterministically from the project's event stream + llm_call rows —
// the un-forgeable record of how the student and 印记 actually interacted. No LLM
// call: this is fact, not a model output, so it can seed the student's self-audit
// without the AI shaping it (克制). Crucially it also asserts the ABSENCES that
// matter (no essay ghostwriting, no score prediction) — we have no such event,
// so they are always false, stated as fact rather than guessed.

// aiUseRecord is the objective interaction record. Wire DTO mirrors this
// (camelCase) in the handler.
type aiUseRecord struct {
	CoachTurns        int
	CardsProposed     int
	CardsAccepted     int
	CardsDismissed    int
	SourcesOpened     int
	LLMCallsByPurpose map[string]int
	GhostwroteEssay   bool // always false — no such event exists
	PredictedScore    bool // always false — no such event exists
}

// buildAIUseRecord counts the interaction from the raw event + llm_call rows.
// Pure — no DB, no spend — so it is unit-testable and always current.
func buildAIUseRecord(events []sqlc.Event, llmCalls []sqlc.LlmCall) aiUseRecord {
	rec := aiUseRecord{LLMCallsByPurpose: map[string]int{}}
	for _, e := range events {
		switch e.Type {
		case "coach_turn":
			rec.CoachTurns++
		case "coach_proposed":
			rec.CardsProposed++
		case "card_activated", "card_logged", "rabbit_hole_logged":
			// A card the student ENGAGED — across all summon paths, which each emit
			// a DISJOINT event (reading loop → card_activated; coach/writing persist
			// → card_logged; 兔子洞 → rabbit_hole_logged), so counting all three is
			// exactly-once. card_activated ALONE misses every coach-proposed card
			// (whole-branch review CRITICAL): those complete via /cards/persist and
			// emit only card_logged, so an accepted proposal was reported as 0.
			rec.CardsAccepted++
		case "coach_proposal_skipped", "card_skipped":
			rec.CardsDismissed++
		case "source_opened":
			rec.SourcesOpened++
		}
	}
	for _, c := range llmCalls {
		rec.LLMCallsByPurpose[c.Purpose]++
	}
	return rec
}

// hasInteractionRecord reports whether there is any interaction to reflect on —
// the gate before the seed compose spends anything (克制 / no-spend on empty).
func hasInteractionRecord(r aiUseRecord) bool {
	return r.CoachTurns > 0 || r.CardsProposed > 0 || r.SourcesOpened > 0 || len(r.LLMCallsByPurpose) > 0
}
