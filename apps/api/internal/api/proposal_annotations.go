package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// proposal_annotations.go — slice 3b · the 批注 endpoints. A flagship reviewer
// reads the proposal buffer and returns layered colored 批注, persisted (replace
// per doc) and surfaced VIEW-ONLY in the left panel. Never touches the draft.

type draftAnnotationDTO struct {
	ID      string `json:"id"`
	Level   string `json:"level"`
	Nature  string `json:"nature"`
	Quote   string `json:"quote"`
	Locator string `json:"locator"`
	Note    string `json:"note"`
}

// runDraftAnnotationReview reads the proposal buffer, runs the flagship 批注
// reviewer (focus empty = whole draft; else scoped to a step), replaces the
// doc's 批注 set, and returns the persisted rows as DTOs. Best-effort: nil
// resolver / model error → the prior set is left untouched and an empty list is
// returned (never breaks the surface).
func (a *API) runDraftAnnotationReview(ctx context.Context, projectID uuid.UUID, focus string) []draftAnnotationDTO {
	if a.d.Provider == nil || a.d.EvalResolver == nil {
		return a.listProposalAnnotationDTOs(ctx, projectID)
	}
	resolved, rerr := a.d.EvalResolver(ctx)
	if rerr != nil {
		return a.listProposalAnnotationDTOs(ctx, projectID)
	}
	title := ""
	if p, perr := a.d.Queries.GetProject(ctx, projectID); perr == nil {
		title = p.Title
	}
	draft, _ := a.d.Queries.GetEditBuffer(ctx, sqlc.GetEditBufferParams{ProjectID: projectID, DocKind: string(agent.DocProposal)})

	items, usage, err := agent.ReviewDraftAnnotations(ctx, a.d.Provider, resolved, agent.DraftAnnotationInput{
		Title: title, Draft: draft, Focus: focus,
	})
	a.meterCall(ctx, projectID, resolved, "proposal_annotation", usage)
	if err != nil {
		slog.Warn("proposal annotations: review failed — keeping prior set", "err", err, "request_id", httpx.RequestIDFromContext(ctx))
		return a.listProposalAnnotationDTOs(ctx, projectID)
	}

	rows := make([]agent.ProposalAnnotationRow, 0, len(items))
	for _, it := range items {
		rows = append(rows, agent.ProposalAnnotationRow{
			Level: it.Level, Nature: it.Nature, Quote: it.Quote, Locator: it.Locator, Note: it.Note,
		})
	}
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	if rerr := store.ReplaceProposalAnnotations(ctx, projectID, rows); rerr != nil {
		slog.Warn("proposal annotations: replace failed", "err", rerr, "request_id", httpx.RequestIDFromContext(ctx))
	}
	return a.listProposalAnnotationDTOs(ctx, projectID)
}

// listProposalAnnotationDTOs reads the persisted 批注 rows and maps them to DTOs
// (the structured facets ride the anchor jsonb).
func (a *API) listProposalAnnotationDTOs(ctx context.Context, projectID uuid.UUID) []draftAnnotationDTO {
	rows, err := a.d.Queries.ListProposalAnnotations(ctx, projectID)
	if err != nil {
		return []draftAnnotationDTO{}
	}
	out := make([]draftAnnotationDTO, 0, len(rows))
	for _, row := range rows {
		var anchor struct {
			Level   string `json:"level"`
			Nature  string `json:"nature"`
			Quote   string `json:"quote"`
			Locator string `json:"locator"`
		}
		if len(row.Anchor) > 0 {
			_ = json.Unmarshal(row.Anchor, &anchor)
		}
		out = append(out, draftAnnotationDTO{
			ID: row.ID.String(), Level: anchor.Level, Nature: anchor.Nature,
			Quote: anchor.Quote, Locator: anchor.Locator, Note: row.Body,
		})
	}
	return out
}

// GET /projects/{id}/proposal-annotations
func (a *API) getProposalAnnotations(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"annotations": a.listProposalAnnotationDTOs(r.Context(), projectID)})
}

// POST /projects/{id}/proposal-annotations/review — the whole-draft "AI check".
func (a *API) reviewProposalAnnotations(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	out := a.runDraftAnnotationReview(r.Context(), projectID, "")
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"annotations": out})
}
