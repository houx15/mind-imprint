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
