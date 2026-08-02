package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/rubric"
	"mindimprint/api/internal/store/sqlc"
	"mindimprint/api/internal/studio"
)

// getChatAssessment returns the thread's most recently generated report, or an
// explicit JSON null when none exists — the report view's own empty state,
// never a 404. No model call, ever. Thread-scoped sibling of getCourseAssessment.
func (a *API) getChatAssessment(w http.ResponseWriter, r *http.Request) {
	threadID, ok := a.loadOwnedThread(w, r)
	if !ok {
		return
	}
	row, err := a.d.Queries.GetLatestThreadEvaluation(r.Context(), pgtype.UUID{Bytes: threadID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteJSON(w, http.StatusOK, nil)
			return
		}
		httpx.WriteError(w, r, err)
		return
	}
	dto, derr := reportDTOFromEvaluationRow(row)
	if derr != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dto)
}

// generateChatAssessment runs the isolated flagship assessor over one chat
// thread's evidence: load the thread's events + cards, digest, ONE flagship
// call (never downgraded), record the cost regardless of outcome, and — only
// on success — persist at thread scope and return. Student-opt-in (铁律 2):
// only ever reached by an explicit POST, never auto-run.
func (a *API) generateChatAssessment(w http.ResponseWriter, r *http.Request) {
	threadID, ok := a.loadOwnedThread(w, r)
	if !ok {
		return
	}
	u, _ := UserFromContext(r.Context())
	entitled, eerr := HasEntitlement(r.Context(), u)
	if eerr != nil {
		httpx.WriteError(w, r, eerr)
		return
	}
	if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}

	tid := pgtype.UUID{Bytes: threadID, Valid: true}
	eventRows, err := a.d.Queries.ListEventsByThread(r.Context(), tid)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	events := make([]studio.Event, len(eventRows))
	for i, ev := range eventRows {
		events[i] = studio.Event{
			Type: ev.Type, Surface: ev.Surface,
			Payload: json.RawMessage(ev.Payload), CreatedAt: ev.CreatedAt,
		}
	}
	cardRows, err := a.d.Queries.ListCardInstancesByThread(r.Context(), tid)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	in := buildAssessmentInputFromEvidence(events, cardRows)

	resolved, rerr := a.d.EvalResolver(r.Context())
	if rerr != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	report, usage, aerr := agent.AssessReport(r.Context(), a.d.Provider, resolved, rubric.Model(), in)

	// Record cost even if enforcement then rejects — a rejected call still cost
	// money (mirrors generateCourseAssessment).
	store := agent.NewSqlcChatStore(a.d.Queries)
	if resolved.Provider != "" {
		if err := store.RecordChatLLMCall(r.Context(), u.ID, "assessment", resolved,
			int32(usage.InputTokens), int32(usage.OutputTokens)); err != nil {
			slog.Warn("generate_chat_assessment: record llm call", "err", err)
		}
	}
	if aerr != nil {
		slog.Warn("generate_chat_assessment: rejected", "err", aerr)
		httpx.WriteError(w, r, &httpx.APIError{
			Status: http.StatusUnprocessableEntity, Code: "assessment_rejected",
			Message: "这次评估没通过内部校验，请再试一次",
		})
		return
	}

	scoresJSON, merr := json.Marshal(report)
	if merr != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	cost, priced := gateway.EstimateCost(resolved.Provider, resolved.Model, usage.InputTokens, usage.OutputTokens)
	promptTokens := int32(usage.InputTokens)
	completionTokens := int32(usage.OutputTokens)
	row, err := a.d.Queries.InsertThreadEvaluation(r.Context(), sqlc.InsertThreadEvaluationParams{
		ThreadID:         tid,
		Scores:           scoresJSON,
		Narrative:        report.Narrative,
		Model:            resolved.Model,
		Tier:             resolved.Tier,
		PromptTokens:     &promptTokens,
		CompletionTokens: &completionTokens,
		CostEstimate:     gateway.CostNumeric(cost, priced),
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	dto, derr := reportDTOFromEvaluationRow(row)
	if derr != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dto)
}

// buildAssessmentInputFromEvidence maps an event stream + cards onto
// agent.BuildAssessmentInput's primitive slices. Used to live in
// course_assessment_input.go, shared with the course session (A1) report;
// migration 0050 (course v2) retired course_session and Task 5 deleted that
// file along with the rest of the phase-gated runtime, so this is now
// chat-only — relocated here rather than deleted, since chat's report still
// needs it.
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
		false,      // ProjectProjection — chat/course are both non-project surfaces
		[]string{}, // WorkSamples — no draft snapshots on either surface
	)
}

// roundsFromEvidence pairs student-message events into ordered rounds. Chat
// has no separate AI-context projection, so AiContext is left empty; the
// student prompt alone still drives SOLO + prompt-lens. A "student turn" is a
// chat prompt_sent event (the course_message half of this rule died with
// course_session; kept only for chat now) — read back via promptText, the
// same payload-to-real-text reader roundsFromProject trusts. A turn whose
// payload lacks text yields an honest empty prompt — never a fabricated one
// (see reportPosture's anti-fabrication guard).
func roundsFromEvidence(events []studio.Event) []agent.Round {
	rounds := make([]agent.Round, 0)
	n := 0
	for _, e := range events {
		if e.Type != "prompt_sent" {
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
