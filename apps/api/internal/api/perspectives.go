package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// perspectiveLevels is the binding design's own closed set of the three
// levels a perspective row can be written at — anything else is a validation
// error, not a silently-dropped row.
var perspectiveLevels = map[string]bool{
	"national":       true,
	"global_for":     true,
	"global_against": true,
}

// submitPerspectives persists the student's S1 视角与素材 perspective list.
// Every row becomes a `perspective` graph node, author "student" (铁律 1),
// body marked origin:"station_view" so a re-save only ever replaces what
// THIS endpoint wrote.
//
// The perspective-matrix tool card ALSO mints `perspective` nodes
// (agent/card_effects.go:~130), body {"text","cells"} with NO origin key —
// those are a different producer entirely and must survive untouched; that
// is exactly what DeleteStationViewNodes's origin scoping (Task 2) buys us.
// This endpoint attests nothing: sources_per_perspective rides the generic
// POST .../gate/{contractId}/attest endpoint (spec §6.2).
func (a *API) submitPerspectives(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r) // 404 hides other users'
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
	var req struct {
		Perspectives []struct {
			Text  string `json:"text"`
			Level string `json:"level"`
		} `json:"perspectives"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadJSON(err))
		return
	}

	// Validate levels up front, before touching the database — an unknown
	// level is a real client bug, not partial work to save.
	for _, p := range req.Perspectives {
		if strings.TrimSpace(p.Text) == "" {
			continue // blank row — an offer is never a wall, dropped not rejected
		}
		if !perspectiveLevels[p.Level] {
			httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "视角层级不对", nil))
			return
		}
	}

	// One transaction: the delete and the inserts must not be separable, or a
	// failed insert leaves her with LESS than she had before pressing save.
	// Scoped by origin (spec §6.1) — the perspective-matrix tool card mints
	// `perspective` nodes with no origin key at all, so it is never touched.
	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	qtx := a.d.Queries.WithTx(tx)

	if err := qtx.DeleteStationViewNodes(r.Context(), sqlc.DeleteStationViewNodesParams{
		ProjectID: projectID, Types: []string{"perspective"},
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	for _, p := range req.Perspectives {
		text := strings.TrimSpace(p.Text)
		if text == "" {
			continue // blank row — an offer is never a wall, dropped not rejected
		}
		body, _ := json.Marshal(map[string]any{
			"text": text, "level": p.Level, "origin": "station_view",
		})
		if _, err := qtx.InsertGraphNode(r.Context(), sqlc.InsertGraphNodeParams{
			ProjectID: projectID, Type: "perspective", Body: body, Author: "student", SpanRef: nil,
		}); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}

	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	payload, _ := json.Marshal(map[string]any{"perspective_count": len(req.Perspectives)})
	if err := store.AppendEvent(r.Context(), agent.EventRow{
		ProjectID: projectID, Surface: "studio", Type: "perspectives_written", Payload: payload,
	}); err != nil {
		slog.Warn("perspectives: append event failed", "err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}
	a.advanceGates(r.Context(), projectID)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{})
}
