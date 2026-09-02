package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"unicode/utf8"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// submitFraming persists the student's S1 立题 work: her own key-term
// definitions, her candidate core arguments, and where she plans to look for
// evidence. Three graph node types, all author "student" — the platform never
// authors any of it (铁律 1). No model call.
func (a *API) submitFraming(w http.ResponseWriter, r *http.Request) {
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
		Terms []struct {
			Term       string `json:"term"`
			Definition string `json:"definition"`
		} `json:"terms"`
		Answers    []string `json:"answers"`
		SearchPlan []string `json:"searchPlan"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadJSON(err))
		return
	}

	// One transaction: the delete and the inserts must not be separable, or a
	// failed insert leaves her with LESS than she had before pressing save.
	// Scoped by origin (spec §6.1) — these three types have no other producer
	// today, but the marker is uniform across every station-view write.
	types := []string{"term_definition", "provisional_answer", "preregistration"}
	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	qtx := a.d.Queries.WithTx(tx)

	if err := qtx.DeleteStationViewNodes(r.Context(), sqlc.DeleteStationViewNodesParams{
		ProjectID: projectID, Types: types,
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	definedCount := 0
	for _, t := range req.Terms {
		term := strings.TrimSpace(t.Term)
		if term == "" {
			continue // blank row — an offer is never a wall, dropped not rejected
		}
		def := t.Definition
		if utf8.RuneCountInString(strings.TrimSpace(def)) >= 15 {
			definedCount++
		}
		body, _ := json.Marshal(map[string]any{
			"term": term, "definition": def, "origin": "station_view",
		})
		if _, err := qtx.InsertGraphNode(r.Context(), sqlc.InsertGraphNodeParams{
			ProjectID: projectID, Type: "term_definition", Body: body, Author: "student", SpanRef: nil,
		}); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}
	for _, ans := range req.Answers {
		text := strings.TrimSpace(ans)
		if text == "" {
			continue
		}
		body, _ := json.Marshal(map[string]any{"text": text, "origin": "station_view"})
		if _, err := qtx.InsertGraphNode(r.Context(), sqlc.InsertGraphNodeParams{
			ProjectID: projectID, Type: "provisional_answer", Body: body, Author: "student", SpanRef: nil,
		}); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}
	directions := make([]string, 0, len(req.SearchPlan))
	for _, d := range req.SearchPlan {
		if text := strings.TrimSpace(d); text != "" {
			directions = append(directions, text)
		}
	}
	if len(directions) > 0 {
		body, _ := json.Marshal(map[string]any{"directions": directions, "origin": "station_view"})
		if _, err := qtx.InsertGraphNode(r.Context(), sqlc.InsertGraphNodeParams{
			ProjectID: projectID, Type: "preregistration", Body: body, Author: "student", SpanRef: nil,
		}); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}

	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// terms_defined: 3 definitions at >=15 runes, the binding design's own
	// threshold (dc.html:2153 `ok = t.length>=15`). Cleared when it stops
	// holding — an emptied definition must un-attest, not leave a stale solid
	// behind (attestGate's exact set/delete pattern, writing.go).
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	recorded, err := store.ListGateStates(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	rec := recorded["frame_question"]
	if rec.Items == nil {
		rec.Items = map[string]string{}
	}
	if definedCount >= 3 {
		rec.Items["terms_defined"] = "solid"
	} else {
		delete(rec.Items, "terms_defined")
	}
	if err := store.UpsertGateState(r.Context(), projectID, "frame_question", rec); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	payload, _ := json.Marshal(map[string]any{"defined_count": definedCount})
	if err := store.AppendEvent(r.Context(), agent.EventRow{
		ProjectID: projectID, Surface: "studio", Type: "framing_written", Payload: payload,
	}); err != nil {
		slog.Warn("framing: append event failed", "err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}
	a.advanceGates(r.Context(), projectID)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{})
}
