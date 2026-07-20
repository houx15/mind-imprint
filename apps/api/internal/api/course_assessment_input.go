package api

import (
	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/store/sqlc"
	"mindimprint/api/internal/studio"
)

// buildAssessmentInputFromEvidence maps an event stream + cards onto
// agent.BuildAssessmentInput's primitive slices — shared by the course session
// (A1) and chat thread (A2) reports, whose evidence is exactly events + cards,
// no gates/graph.
//
// This evidence has no gates, no draft snapshots, and no argument graph, so
// those arguments are empty: Assess's own NA defaults then report those
// dimensions as unevidenced, which is the honest answer (spec §4) — not a bug
// to paper over.
//
// Pure — no I/O. The handler does the loading.
func buildAssessmentInputFromEvidence(events []studio.Event, cards []sqlc.CardInstance) agent.AssessmentInput {
	return agent.BuildAssessmentInput(
		eventDigestsFromProject(events),
		cardUsesFromEvidence(cards),
		dispositionUsesFromEvidence(cards),
		nil, // GateProgress — no gates
		nil, // WordCounts   — no draft snapshots
		nil, // ReviewBands  — no whole-draft review
		"",  // GraphSummary — no argument graph
		roundsFromEvidence(events),
	)
}

// roundsFromEvidence pairs student-message events into ordered rounds. Course/chat
// have no separate AI-context projection, so AiContext is left empty; the student
// prompt alone still drives SOLO + prompt-lens. A "student turn" is a chat
// prompt_sent event OR a course course_message event — the two surfaces name their
// student turn differently but both carry the real text in a {"text":…} payload,
// read back via promptText (the same payload-to-real-text reader roundsFromProject
// trusts) so rounds and the timeline agree on what a turn is and how it reads. The
// two event types are disjoint by surface, so a chat stream still yields only its
// prompt_sent rounds and a course stream only its course_message rounds. A turn
// whose payload lacks text yields an honest empty prompt — never a fabricated one
// (see reportPosture's anti-fabrication guard).
func roundsFromEvidence(events []studio.Event) []agent.Round {
	rounds := make([]agent.Round, 0)
	n := 0
	for _, e := range events {
		if e.Type != "prompt_sent" && e.Type != "course_message" {
			continue
		}
		n++
		rounds = append(rounds, agent.Round{N: n, StudentPrompt: promptText(e), AiContext: ""})
	}
	return rounds
}

// cardUsesFromEvidence carries each card. Unlike the project side there is no
// Equipment projection to zip against, so Spont is left empty rather than
// guessed — the 自发/提示后 signal is a studio derivation and inventing one here
// would be a fabricated fact.
func cardUsesFromEvidence(cards []sqlc.CardInstance) []agent.CardUse {
	out := make([]agent.CardUse, 0, len(cards))
	for _, ci := range cards {
		out = append(out, agent.CardUse{CardID: ci.CardID})
	}
	return out
}

// dispositionUsesFromEvidence reports what the student did with each offered
// card. A skip is evidence, not an absence — Slice 12 made card offers
// skippable by design, and the skip is exactly the signal 过程即数据 wants.
func dispositionUsesFromEvidence(cards []sqlc.CardInstance) []agent.DispositionUse {
	out := make([]agent.DispositionUse, 0, len(cards))
	for _, ci := range cards {
		out = append(out, agent.DispositionUse{Kind: ci.Status})
	}
	return out
}
