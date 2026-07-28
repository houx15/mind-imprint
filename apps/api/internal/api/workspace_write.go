package api

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// workspace_write.go — Slice 4 (Write room) read/write handlers: the outline
// (bullets ⇄ mind-map over the SAME depth-indexed flat list) and the current
// draft (the silent edit buffer, read back). None of these make a model call
// (that is /coach's job), so none gate on HasEntitlement — plain owned-project
// REST. The draft's WRITE path is the pre-existing PUT /buffer (putEditBuffer);
// GET /draft here is its read side.

// outlineNodeDTO is the wire shape (contracts.OutlineNode): id/text/depth/
// position, camelCase JSON. position orders siblings; depth drives indent.
type outlineNodeDTO struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	Depth    int32  `json:"depth"`
	Position int32  `json:"position"`
}

func toOutlineNodeDTO(row sqlc.OutlineNode) outlineNodeDTO {
	return outlineNodeDTO{
		ID:       row.ID.String(),
		Text:     row.Text,
		Depth:    row.Depth,
		Position: row.Position,
	}
}

// listOutline returns the project's outline nodes ordered by position.
func (a *API) listOutline(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	rows, err := a.d.Queries.ListOutlineNodes(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	nodes := make([]outlineNodeDTO, 0, len(rows))
	for _, row := range rows {
		nodes = append(nodes, toOutlineNodeDTO(row))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"nodes": nodes})
}

// putOutline replaces the whole outline set: in ONE transaction it deletes all
// of the project's outline nodes then re-inserts the posted array in order,
// with position = array index. Incoming ids are ignored and fresh ones minted
// (the client re-reads the response), and depth is clamped to 0..2. Returns the
// freshly-read nodes so the client can reconcile ids.
func (a *API) putOutline(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	var body struct {
		Nodes []struct {
			Text  string `json:"text"`
			Depth int32  `json:"depth"`
		} `json:"nodes"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// "First non-empty save" = no outline row yet AND the incoming set carries
	// real text (a single blank editable row is the empty-outline UI state, not
	// the outline taking shape). Read before the replace so the auto-log fires
	// exactly once, on the transition from absent to present (mirrors putProposal).
	hasText := false
	for _, n := range body.Nodes {
		if strings.TrimSpace(n.Text) != "" {
			hasText = true
			break
		}
	}
	existing, existErr := a.d.Queries.ListOutlineNodes(r.Context(), projectID)
	firstNonEmpty := existErr == nil && len(existing) == 0 && hasText

	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	qtx := a.d.Queries.WithTx(tx)

	if err := qtx.DeleteAllOutlineNodes(r.Context(), projectID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	for i, n := range body.Nodes {
		if _, err := qtx.CreateOutlineNode(r.Context(), sqlc.CreateOutlineNodeParams{
			ProjectID: projectID,
			Text:      n.Text,
			Depth:     clampDepth(n.Depth),
			Position:  int32(i),
		}); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	if firstNonEmpty {
		if err := a.appendAutoLog(r.Context(), a.d.Queries, projectID, "写作提纲初次落定"); err != nil {
			slog.Warn("outline: append auto-log failed", "err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
		}
	}

	// Re-read outside the transaction so the response is exactly what a
	// subsequent GET /outline would return (fresh ids, position = index).
	rows, err := a.d.Queries.ListOutlineNodes(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	nodes := make([]outlineNodeDTO, 0, len(rows))
	for _, row := range rows {
		nodes = append(nodes, toOutlineNodeDTO(row))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"nodes": nodes})
}

// getDraft returns the project's current edit buffer content (the silent
// student scratch), or "" when no buffer row exists yet.
func (a *API) getDraft(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	content, err := a.d.Queries.GetEditBuffer(r.Context(), projectID)
	if errors.Is(err, pgx.ErrNoRows) {
		content = ""
	} else if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"content": content})
}

// clampDepth keeps outline depth in the proto's 0..2 range (bullet nesting is
// three levels deep; anything the client sends outside that is clamped, never
// rejected).
func clampDepth(d int32) int32 {
	if d < 0 {
		return 0
	}
	if d > 2 {
		return 2
	}
	return d
}
