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
// annotationDocParam reads the 批注 document, defaulting to PROPOSAL (3b
// back-compat — the proposal path calls these without ?doc). ?doc=essay selects
// the essay.
func annotationDocParam(r *http.Request) string {
	if r.URL.Query().Get("doc") == string(agent.DocEssay) {
		return string(agent.DocEssay)
	}
	return string(agent.DocProposal)
}

func (a *API) runDraftAnnotationReview(ctx context.Context, projectID uuid.UUID, doc, focus string) []draftAnnotationDTO {
	if a.d.Provider == nil || a.d.EvalResolver == nil {
		return a.listAnnotationDTOs(ctx, projectID, doc)
	}
	resolved, rerr := a.d.EvalResolver(ctx)
	if rerr != nil {
		return a.listAnnotationDTOs(ctx, projectID, doc)
	}
	title := ""
	if p, perr := a.d.Queries.GetProject(ctx, projectID); perr == nil {
		title = p.Title
	}
	draft, _ := a.d.Queries.GetEditBuffer(ctx, sqlc.GetEditBufferParams{ProjectID: projectID, DocKind: doc})

	items, usage, err := agent.ReviewDraftAnnotations(ctx, a.d.Provider, resolved, agent.DraftAnnotationInput{
		Title: title, Draft: draft, Focus: focus, Doc: doc,
	})
	a.meterCall(ctx, projectID, resolved, doc+"_annotation", usage)
	if err != nil {
		slog.Warn("annotations: review failed — keeping prior set", "err", err, "doc", doc, "request_id", httpx.RequestIDFromContext(ctx))
		return a.listAnnotationDTOs(ctx, projectID, doc)
	}

	rows := make([]agent.ProposalAnnotationRow, 0, len(items))
	for _, it := range items {
		rows = append(rows, agent.ProposalAnnotationRow{
			Level: it.Level, Nature: it.Nature, Quote: it.Quote, Locator: it.Locator, Note: it.Note,
		})
	}
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	if rerr := store.ReplaceAnnotations(ctx, projectID, doc, rows); rerr != nil {
		slog.Warn("annotations: replace failed", "err", rerr, "doc", doc, "request_id", httpx.RequestIDFromContext(ctx))
	}
	return a.listAnnotationDTOs(ctx, projectID, doc)
}

// listAnnotationDTOs reads a doc's persisted 批注 rows and maps them to DTOs (the
// structured facets ride the anchor jsonb).
func (a *API) listAnnotationDTOs(ctx context.Context, projectID uuid.UUID, doc string) []draftAnnotationDTO {
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	rows, err := store.ListAnnotations(ctx, projectID, doc)
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
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"annotations": a.listAnnotationDTOs(r.Context(), projectID, annotationDocParam(r))})
}

// POST /projects/{id}/proposal-annotations/review[?doc=essay] — the whole-draft "AI check".
func (a *API) reviewProposalAnnotations(w http.ResponseWriter, r *http.Request) {
	row, ok := a.loadOwnedProjectRow(w, r)
	if !ok {
		return
	}
	projectID := row.ID
	if row.IsDemo {
		httpx.WriteJSON(w, http.StatusOK, cannedAnnotationReview())
		return
	}
	out := a.runDraftAnnotationReview(r.Context(), projectID, annotationDocParam(r), "")
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"annotations": out})
}

// postAnnotationOpen (G2) records that the student OPENED / clicked a specific
// AI writing 批注 — the "did she engage with AI feedback" signal for the
// (deferred) report generator (D5 · 反馈处理与修订). The annotations themselves
// are already durably stored (intervention rows); this appends one append-only
// `annotation_opened` event per open, keyed by the annotation id. An event
// (not an `opened_at` column) so it survives ReplaceAnnotations wiping+reinserting
// the set on each re-review. Best-effort recording — 204 regardless.
func (a *API) postAnnotationOpen(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	var body struct {
		AnnotationID string `json:"annotationId"`
		Doc          string `json:"doc"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if body.AnnotationID == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_request", "annotationId 不能为空", nil))
		return
	}
	payload, _ := json.Marshal(map[string]any{"annotation_id": body.AnnotationID, "doc": body.Doc})
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	if err := store.AppendEvent(r.Context(), agent.EventRow{
		ProjectID: projectID, Surface: "studio", Type: "annotation_opened", Payload: payload,
	}); err != nil {
		slog.Warn("annotation open: append event failed",
			"err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}
	w.WriteHeader(http.StatusNoContent)
}
