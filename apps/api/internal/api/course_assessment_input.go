package api

import (
	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/store/sqlc"
	"mindimprint/api/internal/studio"
)

// buildAssessmentInputFromSession maps one course session's evidence onto
// agent.BuildAssessmentInput's primitive slices — the session-scoped sibling of
// buildAssessmentInputFromProject (assessment.go).
//
// A course session's evidence is its event stream and its cards. It has no
// gates, no draft snapshots, and no argument graph, so those arguments are
// empty: Assess's own NA defaults then report those dimensions as unevidenced,
// which is the honest answer for a course (spec §4) — not a bug to paper over.
//
// Pure — no I/O. The handler does the loading.
func buildAssessmentInputFromSession(events []studio.Event, cards []sqlc.CardInstance) agent.AssessmentInput {
	return agent.BuildAssessmentInput(
		eventDigestsFromProject(events),
		cardUsesFromSession(cards),
		dispositionUsesFromSession(cards),
		nil, // GateProgress — a course session has no gates
		nil, // WordCounts   — no draft snapshots
		nil, // ReviewBands  — no whole-draft review
		"",  // GraphSummary — no argument graph
	)
}

// cardUsesFromSession carries each session card. Unlike the project side there
// is no Equipment projection to zip against, so Spont is left empty rather than
// guessed — the 自发/提示后 signal is a studio derivation and inventing one here
// would be a fabricated fact.
func cardUsesFromSession(cards []sqlc.CardInstance) []agent.CardUse {
	out := make([]agent.CardUse, 0, len(cards))
	for _, ci := range cards {
		out = append(out, agent.CardUse{CardID: ci.CardID})
	}
	return out
}

// dispositionUsesFromSession reports what the student did with each offered
// card. A skip is evidence, not an absence — Slice 12 made card offers
// skippable by design, and the skip is exactly the signal 过程即数据 wants.
func dispositionUsesFromSession(cards []sqlc.CardInstance) []agent.DispositionUse {
	out := make([]agent.DispositionUse, 0, len(cards))
	for _, ci := range cards {
		out = append(out, agent.DispositionUse{Kind: ci.Status})
	}
	return out
}
