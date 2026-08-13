package api

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"

	"mindimprint/api/internal/agent"
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

// proposalOutlineDTOs projects the studio_state-held 提案 outline onto the same
// wire shape as the essay's table-backed outline (synthesized ids/positions —
// the client uses ids only as local React keys and re-reads on every save).
func proposalOutlineDTOs(nodes []agent.ProposalOutlineNode) []outlineNodeDTO {
	out := make([]outlineNodeDTO, 0, len(nodes))
	for i, n := range nodes {
		out = append(out, outlineNodeDTO{ID: fmt.Sprintf("po-%d", i), Text: n.Text, Depth: n.Depth, Position: int32(i)})
	}
	return out
}

// listOutline returns the project's outline nodes ordered by position. The 提案
// (?doc=proposal) has its OWN outline in studio_state, kept separate from the
// essay outline (the table, which review/assessment/digest read).
func (a *API) listOutline(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	if docKindParam(r) == string(agent.DocProposal) {
		state, err := a.loadStudioStateForNeeds(r.Context(), projectID)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"nodes": proposalOutlineDTOs(state.ProposalOutline)})
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

	// 提案 outline → studio_state (kept separate from the essay outline table).
	if docKindParam(r) == string(agent.DocProposal) {
		state, err := a.loadStudioStateForNeeds(r.Context(), projectID)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		nodes := make([]agent.ProposalOutlineNode, 0, len(body.Nodes))
		for _, n := range body.Nodes {
			nodes = append(nodes, agent.ProposalOutlineNode{Text: n.Text, Depth: clampDepth(n.Depth)})
		}
		state.ProposalOutline = nodes
		if serr := a.saveTrackState(r.Context(), projectID, state); serr != nil {
			httpx.WriteError(w, r, serr)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"nodes": proposalOutlineDTOs(nodes)})
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
	content, err := a.d.Queries.GetEditBuffer(r.Context(), sqlc.GetEditBufferParams{ProjectID: projectID, DocKind: docKindParam(r)})
	if errors.Is(err, pgx.ErrNoRows) {
		content = ""
	} else if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"content": content})
}

// snippetDTO is the wire shape for a 片段 (id/text/position/section, camelCase).
// section is the outline heading / 线索 label the snippet is filed under, or null
// (未归类) — #5.
type snippetDTO struct {
	ID       string  `json:"id"`
	Text     string  `json:"text"`
	Position int32   `json:"position"`
	Section  *string `json:"section"`
}

func toSnippetDTO(row sqlc.Snippet) snippetDTO {
	return snippetDTO{ID: row.ID.String(), Text: row.Text, Position: row.Position, Section: row.Section}
}

// normalizeSection collapses an empty/whitespace label to NULL (未归类) so a
// snippet is never filed under a blank category.
func normalizeSection(s *string) *string {
	if s == nil || strings.TrimSpace(*s) == "" {
		return nil
	}
	return s
}

// listSnippets returns the project's snippets ordered by position.
func (a *API) listSnippets(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	rows, err := a.d.Queries.ListSnippets(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]snippetDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, toSnippetDTO(row))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"snippets": out})
}

// putSnippets replaces the whole snippet set in one tx (delete + re-insert the
// posted array, position = index), then re-reads it — mirrors putOutline. No
// model call (student scratch); plain owned-project REST.
func (a *API) putSnippets(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	var body struct {
		Snippets []struct {
			Text    string  `json:"text"`
			Section *string `json:"section"`
		} `json:"snippets"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	qtx := a.d.Queries.WithTx(tx)
	if err := qtx.DeleteAllSnippets(r.Context(), projectID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	for i, s := range body.Snippets {
		if _, err := qtx.CreateSnippet(r.Context(), sqlc.CreateSnippetParams{
			ProjectID: projectID, Text: s.Text, Position: int32(i), Section: normalizeSection(s.Section),
		}); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	rows, err := a.d.Queries.ListSnippets(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]snippetDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, toSnippetDTO(row))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"snippets": out})
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
