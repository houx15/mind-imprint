package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

func (a *API) renderCourseStep(w http.ResponseWriter, r *http.Request) {
	c, ok := a.loadCourse(w, r)
	if !ok {
		return
	}
	u, _ := UserFromContext(r.Context())
	entitled, err := HasEntitlement(r.Context(), u)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}
	ord, err := strconv.Atoi(r.PathValue("ordinal"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	step, err := a.d.Queries.GetCourseStepByOrdinal(r.Context(), sqlc.GetCourseStepByOrdinalParams{CourseID: c.ID, Ordinal: int32(ord)})
	if err != nil {
		httpx.WriteError(w, r, err) // ErrNoRows → 404
		return
	}

	// Record the view server-side (Slice-12 whole-branch C1+C3 fix): this
	// handler fires exactly once per page the student actually opens, which
	// is precisely the semantic the `steps_viewed` floor wants — and, unlike
	// PUT /progress, it can never be spoofed by the client. Idempotent
	// (RecordCourseStepViewed dedupes); best-effort like the metering write
	// below it — a failure here must not block the render the student asked
	// for.
	if _, verr := a.d.Queries.RecordCourseStepViewed(r.Context(), sqlc.RecordCourseStepViewedParams{
		UserID: u.ID, CourseID: c.ID, Ordinal: step.Ordinal,
	}); verr != nil {
		slog.Warn("course render: record step viewed failed", "err", verr, "request_id", httpx.RequestIDFromContext(r.Context()))
	}

	// Cache hit → return it.
	if cached, err := a.d.Queries.GetCourseStepRender(r.Context(), step.ID); err == nil {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"rendered": map[string]any{
			"ordinal": step.Ordinal, "kind": step.Kind,
			"template": step.Kind, "content": json.RawMessage(cached.Content), "source": cached.Source,
		}})
		return
	} else if !errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, err)
		return
	}

	// Miss → generate.
	assets := []agent.CourseAsset{}
	_ = json.Unmarshal(step.Assets, &assets) // best-effort; empty on error
	in := agent.CourseStepInput{
		Ordinal: step.Ordinal, Kind: step.Kind, Purpose: step.Purpose,
		Assets: assets, ChallengeType: step.ChallengeType, AuthoredContent: json.RawMessage(step.AuthoredContent),
	}
	rendered := agent.RenderCourseStep(r.Context(), in, a.d.Provider, a.d.EvalResolver)

	// A real call happened whenever RenderCourseStep attached a Resolved
	// (populated regardless of whether it then fell back to the authored
	// content — the call still cost money). No project owns a course
	// render (surface="course", purpose="course_render", project_id NULL);
	// the acting user is the metering row's owner. A metering failure must
	// never fail the render, same policy as TouchProject.
	if rendered.Resolved.Provider != "" {
		// llm_call.cost_estimate is NOT NULL DEFAULT 0 (migration 0019) — an
		// unpriced model (EstimateCost's ok=false, cost=0) is recorded as an
		// explicit $0.00, never the ok-derived NULL Numeric CostNumeric(cost,
		// ok) would otherwise produce.
		cost, priced := gateway.EstimateCost(rendered.Resolved.Provider, rendered.Resolved.Model, rendered.Usage.InputTokens, rendered.Usage.OutputTokens)
		if !priced {
			slog.Warn("llm_call: unpriced model — cost recorded as 0",
				"provider", rendered.Resolved.Provider, "model", rendered.Resolved.Model)
		}
		if _, rerr := a.d.Queries.RecordLLMCall(r.Context(), sqlc.RecordLLMCallParams{
			UserID: u.ID, Surface: "course", Purpose: "course_render",
			Provider: rendered.Resolved.Provider, Model: rendered.Resolved.Model, Tier: rendered.Resolved.Tier,
			PromptTokens: int32(rendered.Usage.InputTokens), CompletionTokens: int32(rendered.Usage.OutputTokens),
			CostEstimate: gateway.CostNumeric(cost, true),
		}); rerr != nil {
			slog.Warn("course render: record llm usage failed", "err", rerr, "request_id", httpx.RequestIDFromContext(r.Context()))
		}
	}

	// Cache only successful generations (never pin the authored fallback).
	if rendered.Source == "generated" {
		_, _ = a.d.Queries.UpsertCourseStepRender(r.Context(), sqlc.UpsertCourseStepRenderParams{
			CourseStepID: step.ID, Content: []byte(rendered.Content), Source: rendered.Source,
		})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"rendered": rendered})
}
