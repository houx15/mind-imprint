package api

import (
	"errors"
	"net/http"

	"github.com/google/uuid"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
)

// postInterventionDisposition records the student's accept/reject/rewrite
// decision (plus a >=15-rune reason) on an intervention. Mirrors
// postProjectTurn's ownership + decode pattern (loadOwnedProject hides
// non-owned/missing projects as 404, decodeJSON maps malformed bodies to the
// same 400 validation_failed code the rest of the API uses).
func (a *API) postInterventionDisposition(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	iid, err := uuid.Parse(r.PathValue("iid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}

	// Scope iid to the owned project: InsertDisposition has no project filter
	// in its SQL, so without this check a caller who owns projectID could
	// record a disposition against another user's intervention (IDOR) simply
	// by guessing/observing a foreign intervention id. 404 (not 403) to avoid
	// leaking whether the id exists at all — mirrors loadOwnedProject.
	ivs, err := a.d.Queries.ListInterventionsByProject(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	found := false
	for _, iv := range ivs {
		if iv.ID == iid {
			found = true
			break
		}
	}
	if !found {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}

	var body struct {
		Action string `json:"action"`
		Reason string `json:"reason"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	switch body.Action {
	case "accept", "reject", "rewrite":
	default:
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "action 必须是 accept/reject/rewrite 之一", nil))
		return
	}

	deps := agent.AgentDeps{Store: agent.NewSqlcAgentStore(a.d.Queries)}
	if err := agent.RecordDisposition(r.Context(), deps, iid, body.Action, body.Reason); err != nil {
		if errors.Is(err, agent.ErrDispositionReasonTooShort) {
			httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "处置理由至少 15 个字", nil))
			return
		}
		httpx.WriteError(w, r, err) // real store error → 500 via WriteError's default case
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
