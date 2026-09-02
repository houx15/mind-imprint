package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"unicode/utf8"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
	"mindimprint/api/internal/studio"
)

// submitOnboarding persists the student's S0 work: their own restate (≥15 runes)
// and which rubric rows they judged weakest. Recorded as BOTH a student graph
// node (so it survives reload / feeds the graph) and a studio event (过程即数据 →
// the assessor). No model call.
func (a *API) submitOnboarding(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	projectID, ok := a.loadOwnedProject(w, r) // writes 404 itself on miss
	if !ok {
		return
	}
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
		Restate   string `json:"restate"`
		WeakPicks []int  `json:"weakPicks"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadJSON(err))
		return
	}
	if utf8.RuneCountInString(req.Restate) < 15 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("restate_too_short", "用自己的话多写一点（至少 15 字）。", nil))
		return
	}
	if len(req.WeakPicks) > 2 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("too_many_picks", "最多选 2 条。", nil))
		return
	}
	if req.WeakPicks == nil {
		req.WeakPicks = []int{}
	}

	nodeBody, _ := json.Marshal(map[string]any{"restate": req.Restate, "weak_picks": req.WeakPicks})
	if _, err := a.d.Queries.InsertGraphNode(r.Context(), sqlc.InsertGraphNodeParams{
		ProjectID: projectID, Type: "task_restatement", Body: nodeBody, Author: "student", SpanRef: nil,
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// Each weak pick becomes a weakness_prediction node so decode_task's
	// node_count_at_least{weakness_prediction,2} has a producer at all.
	// plainForRow reads the project's rubric_translation node the same way
	// projectOnboarding does; if it or the index is missing, the node still
	// carries `index` alone rather than fabricating a label.
	rubricNodes, err := a.d.Queries.ListGraphNodesByProject(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	rows := rubricRowsFromNodes(rubricNodes)

	// Delete-then-insert so a re-submit cannot inflate the count past
	// decode_task's node_count_at_least{weakness_prediction,2}: pressing save
	// twice with one pick must leave ONE node. Scoped by origin so it can only
	// ever touch nodes this view wrote.
	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	qtx := a.d.Queries.WithTx(tx)

	if err := qtx.DeleteStationViewNodes(r.Context(), sqlc.DeleteStationViewNodesParams{
		ProjectID: projectID, Types: []string{"weakness_prediction"},
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	for _, i := range req.WeakPicks {
		wpBody, _ := json.Marshal(map[string]any{
			"index": i, "plain": plainForRow(rows, i), "origin": "station_view",
		})
		if _, err := qtx.InsertGraphNode(r.Context(), sqlc.InsertGraphNodeParams{
			ProjectID: projectID, Type: "weakness_prediction", Body: wpBody, Author: "student", SpanRef: nil,
		}); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// S0's deliverable is her restate + her weakness picks; the milestone plan
	// is the board-static scaffold she accepts by proceeding. Recorded on HER
	// action, exactly as reflection.go records `reflection` — Advance never
	// marks a student_written item itself (DEC-3).
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	recorded, err := store.ListGateStates(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	rec := recorded["decode_task"]
	if rec.Items == nil {
		rec.Items = map[string]string{}
	}
	rec.Items["milestone_plan"] = "solid"
	if err := store.UpsertGateState(r.Context(), projectID, "decode_task", rec); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// Best-effort telemetry, exactly like studioturn's prompt_sent event.
	payload, _ := json.Marshal(map[string]string{"text": req.Restate})
	if err := store.AppendEvent(r.Context(), agent.EventRow{
		ProjectID: projectID, Surface: "studio", Type: "onboarding_restated", Payload: payload,
	}); err != nil {
		slog.Warn("onboarding submit: append event failed",
			"err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}
	a.advanceGates(r.Context(), projectID)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{})
}

// rubricRowsFromNodes reads the project's rubric_translation node the same
// way studio.projectOnboarding does. Returns nil if the node is absent or
// unparseable — callers must not fabricate rows.
//
// Minor 5 (whole-branch review): this used to define its own private
// rubricRow struct duplicating studio.RubricRowDTO field-for-field
// (identical fields and json tags) rather than importing the exported type —
// but internal/api already imports internal/studio in seven other files, so
// the stated reason (avoiding the dependency) didn't hold, and a duplicated
// wire shape is a silent-drift risk for no benefit.
func rubricRowsFromNodes(nodes []sqlc.GraphNode) []studio.RubricRowDTO {
	for _, n := range nodes {
		if n.Type != "rubric_translation" {
			continue
		}
		var body struct {
			Rows []studio.RubricRowDTO `json:"rows"`
		}
		if json.Unmarshal(n.Body, &body) == nil {
			return body.Rows
		}
	}
	return nil
}

// plainForRow returns the plain-language label for rubric row i, or "" when
// the rubric node or the index is missing — the weakness_prediction node
// then carries `index` alone rather than a fabricated label.
func plainForRow(rows []studio.RubricRowDTO, i int) string {
	if i < 0 || i >= len(rows) {
		return ""
	}
	return rows[i].Plain
}
