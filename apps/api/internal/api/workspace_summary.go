package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// workspace_summary.go — S1 · summary-on-return. Re-opening a project composes
// (once, first-open-wins) a compact re-entry paragraph from the spine
// projection, exactly like the 你的思维印记 mirror: GET never spends and returns
// null until composed; POST is the only spender.

// summaryFallback is returned (NOT persisted) when composition fails, so a
// later open retries instead of caching canned text.
const summaryFallback = "欢迎回来。我们接着上次的思路继续。"

// getProjectSummary returns the stored re-entry summary, or JSON null when none
// has been composed. Never calls a model.
func (a *API) getProjectSummary(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	row, err := a.d.Queries.GetProjectSummaryProse(r.Context(), projectID)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteJSON(w, http.StatusOK, nil)
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"prose": row.Prose})
}

// postProjectSummary is the only summary endpoint that spends. First-open-wins:
// a stored row is returned without a model call; otherwise the flagship composer
// runs once. A SUCCESSFUL composition is persisted; a FAILED one returns the
// fallback WITHOUT persisting (retry on a later open). Cost is recorded on
// failure too (composeReturnSummary does it).
func (a *API) postProjectSummary(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
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
	ctx := r.Context()

	// Already composed → return it, no spend (first-open-wins).
	if row, gerr := a.d.Queries.GetProjectSummaryProse(ctx, projectID); gerr == nil {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"prose": row.Prose})
		return
	} else if !errors.Is(gerr, pgx.ErrNoRows) {
		httpx.WriteError(w, r, gerr)
		return
	}

	projection, perr := a.buildSpineProjection(ctx, projectID, "")
	if perr != nil {
		// No projection (e.g. project read failed) → graceful fallback, no persist.
		slog.Warn("summary: build projection failed", "err", perr, "request_id", httpx.RequestIDFromContext(ctx))
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"prose": summaryFallback})
		return
	}

	prose, model, tier, composed := a.composeReturnSummary(ctx, projectID, projection)
	if !composed {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"prose": summaryFallback})
		return
	}
	if err := a.d.Queries.InsertProjectSummaryProse(ctx, sqlc.InsertProjectSummaryProseParams{
		ProjectID: projectID, Prose: prose, Model: model, Tier: tier,
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// Re-read: a concurrent composer may have won (ON CONFLICT DO NOTHING).
	row, rerr := a.d.Queries.GetProjectSummaryProse(ctx, projectID)
	if rerr != nil {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"prose": prose})
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"prose": row.Prose})
}

// composeReturnSummary runs the flagship composer over the spine projection and
// records the call's cost (surface="studio", purpose="summary") BEFORE any bail.
// It NEVER returns an error: on a missing provider or a failed composition it
// returns composed=false (still recording cost when a call was made). Returns
// the prose plus the resolved model/tier and the composed flag.
func (a *API) composeReturnSummary(ctx context.Context, projectID uuid.UUID, projection string) (string, string, string, bool) {
	resolved, rerr := a.routeE(ctx, gateway.ClassDigest)
	if rerr != nil {
		slog.Warn("summary: no provider", "err", rerr)
		return "", "", "", false
	}
	prose, usage, cerr := agent.ComposeReturnSummary(ctx, a.d.Provider, resolved, projection)
	if u, ok := UserFromContext(ctx); ok && (usage.InputTokens > 0 || usage.OutputTokens > 0) {
		cost, priced := gateway.EstimateCost(resolved.Provider, resolved.Model, usage.InputTokens, usage.OutputTokens)
		if !priced {
			slog.Warn("summary llm_call: unpriced model — cost recorded as 0", "provider", resolved.Provider, "model", resolved.Model)
		}
		if _, err := a.d.Queries.RecordLLMCall(ctx, sqlc.RecordLLMCallParams{
			UserID: u.ID, ProjectID: pgtype.UUID{Bytes: projectID, Valid: true},
			Surface: "studio", Purpose: "summary",
			Provider: resolved.Provider, Model: resolved.Model, Tier: resolved.Tier,
			PromptTokens: int32(usage.InputTokens), CompletionTokens: int32(usage.OutputTokens),
			CostEstimate: gateway.CostNumeric(cost, true),
		}); err != nil {
			slog.Warn("summary: record llm call", "err", err)
		}
	}
	if cerr != nil {
		slog.Warn("summary: compose failed — not persisting (retry on a later open)", "err", cerr)
		return "", resolved.Model, resolved.Tier, false
	}
	return prose, resolved.Model, resolved.Tier, true
}
