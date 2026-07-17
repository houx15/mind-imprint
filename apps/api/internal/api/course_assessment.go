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

// getCourseAssessment returns the session's most recently generated report, or
// an explicit JSON null when none exists — the report view's own empty state,
// never a 404 ("not yet assessed" is a normal state for a session). No model
// call, ever. Mirrors getAssessment (assessment.go) at session scope.
func (a *API) getCourseAssessment(w http.ResponseWriter, r *http.Request) {
	sess, ok := a.loadOwnedSession(w, r)
	if !ok {
		return
	}
	row, err := a.d.Queries.GetLatestSessionEvaluation(r.Context(), pgtype.UUID{Bytes: sess.ID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteJSON(w, http.StatusOK, nil)
			return
		}
		httpx.WriteError(w, r, err)
		return
	}
	dto, derr := dtoFromEvaluationRow(row)
	if derr != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dto)
}

// generateCourseAssessment runs the isolated flagship assessor over one course
// session's evidence: load the session's events + cards, digest, ONE flagship
// call (never downgraded), record the cost regardless of outcome, and — only on
// success — persist at session scope and return. Never in the coach loop, and
// never inside the course's SSE turn (DEC-A1.4).
func (a *API) generateCourseAssessment(w http.ResponseWriter, r *http.Request) {
	sess, ok := a.loadOwnedSession(w, r)
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

	sid := pgtype.UUID{Bytes: sess.ID, Valid: true}
	eventRows, err := a.d.Queries.ListEventsBySession(r.Context(), sid)
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
	cards, err := a.d.Queries.ListCardInstancesBySession(r.Context(), sid)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	in := buildAssessmentInputFromSession(events, cards)

	resolved, rerr := a.d.ChatResolver(r.Context())
	if rerr != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	assessment, usage, aerr := agent.Assess(r.Context(), a.d.Provider, resolved, rubric.CT(), in, agent.EmbeddedAnchors())

	// Record the call's cost even if enforcement then rejects the output — a
	// rejected call still cost money (mirrors generateAssessment/orderReview).
	// The project-side RecordLLMCall cannot be reused: it resolves the owning
	// user via GetProject, and a course session has no project.
	store := agent.NewSqlcCourseStore(a.d.Queries)
	if resolved.Provider != "" {
		if err := store.RecordCourseLLMCall(r.Context(), sess.UserID, "assessment", resolved,
			int32(usage.InputTokens), int32(usage.OutputTokens)); err != nil {
			slog.Warn("generate_course_assessment: record llm call", "err", err)
		}
	}
	if aerr != nil {
		slog.Warn("generate_course_assessment: rejected", "err", aerr)
		httpx.WriteError(w, r, &httpx.APIError{
			Status: http.StatusUnprocessableEntity, Code: "assessment_rejected",
			Message: "这次评估没通过内部校验，请再试一次",
		})
		return
	}

	scoresJSON, merr := json.Marshal(assessment.Dimensions)
	if merr != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	cost, priced := gateway.EstimateCost(resolved.Provider, resolved.Model, usage.InputTokens, usage.OutputTokens)
	promptTokens := int32(usage.InputTokens)
	completionTokens := int32(usage.OutputTokens)
	row, err := a.d.Queries.InsertSessionEvaluation(r.Context(), sqlc.InsertSessionEvaluationParams{
		SessionID:        sid,
		Scores:           scoresJSON,
		Narrative:        assessment.Narrative,
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
	dto, derr := dtoFromEvaluationRow(row)
	if derr != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dto)
}
