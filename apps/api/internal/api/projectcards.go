package api

// projectcards.go — Task 4: POST /projects/{id}/cards/{cid}/activate and
// .../skip, the project-scoped card lifecycle endpoints that sit beside
// postProjectTurn's card surfacing (Task 5c-2, agent-spec §3). Both reuse
// loadOwnedProjectCard, which mirrors disposition.go's ownership+membership
// pattern: loadOwnedProject hides non-owned/missing projects as 404, then a
// ListCardInstancesByProject scan scopes {cid} to that project (a card_id
// has no project filter of its own in the schema, so without this scan a
// caller who owns projectID could act on another user's card by
// guessing/observing a foreign card id — the same IDOR loadOwnedProjectCard's
// sibling in disposition.go documents for interventions). 404, not 403, so
// existence never leaks.

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
)

// loadOwnedProjectCard scopes {cid} to the owned {id} project (404-no-leak).
func (a *API) loadOwnedProjectCard(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return uuid.UUID{}, uuid.UUID{}, false
	}
	cid, err := uuid.Parse(r.PathValue("cid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return uuid.UUID{}, uuid.UUID{}, false
	}
	cis, err := a.d.Queries.ListCardInstancesByProject(r.Context(), pgtype.UUID{Bytes: projectID, Valid: true})
	if err != nil {
		httpx.WriteError(w, r, err)
		return uuid.UUID{}, uuid.UUID{}, false
	}
	for _, ci := range cis {
		if ci.ID == cid {
			return projectID, cid, true
		}
	}
	httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
	return uuid.UUID{}, uuid.UUID{}, false
}

// activateProjectCard marks a surfaced card "active" once the student opens
// it (design's "触发是自动的，但「打开」由学生确认" — opening is a distinct,
// recorded step from being surfaced). Uses the agent.AgentStore seam (Task 2)
// so the write goes through the same path RunAgentStep itself would use.
func (a *API) activateProjectCard(w http.ResponseWriter, r *http.Request) {
	projectID, cid, ok := a.loadOwnedProjectCard(w, r)
	if !ok {
		return
	}
	store := agent.NewSqlcAgentStore(a.d.Queries)
	if err := store.SetCardInstanceStatus(r.Context(), projectID, cid, "active"); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	_ = store.AppendEvent(r.Context(), agent.EventRow{
		ProjectID: projectID, Surface: "studio", Type: "card_activated", Payload: []byte(`{}`),
	})
	w.WriteHeader(http.StatusNoContent)
}

// skipProjectCard records a skipped card: process-is-data (design's 「过程即
// 数据」) means a skip is submitted with an empty field_values payload rather
// than silently discarded, so the event trace still lands in the process
// tree.
func (a *API) skipProjectCard(w http.ResponseWriter, r *http.Request) {
	projectID, cid, ok := a.loadOwnedProjectCard(w, r)
	if !ok {
		return
	}
	var body struct {
		EventTrace json.RawMessage `json:"event_trace"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := validateEventTrace(body.EventTrace); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	store := agent.NewSqlcAgentStore(a.d.Queries)
	if err := store.SubmitProjectCardInstance(r.Context(), projectID, cid, []byte("{}"), body.EventTrace); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := store.SetCardInstanceStatus(r.Context(), projectID, cid, "skipped"); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	_ = store.AppendEvent(r.Context(), agent.EventRow{
		ProjectID: projectID, Surface: "studio", Type: "card_skipped", Payload: []byte(`{}`),
	})
	w.WriteHeader(http.StatusNoContent)
}
