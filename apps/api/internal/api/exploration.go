package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// exploration.go — S3 rabbit-hole exploration surface, Task 4: the lead
// lifecycle handlers (no LLM spend). A "lead" is a thread worth following —
// spawned from a reading takeaway's newLeads (Task 4b, extends
// postFinalizeReading), typed in manually here, or proposed by the 深挖一层
// guide (Task 4c, SPENDS, not this file). getExploration also projects
// danglingSourceIds: read sources with nothing following up on them yet, a
// nudge to connect or prune rather than leaving them orphaned.

// -- wire DTO -----------------------------------------------------------

// explorationLeadDTO mirrors the contracts ExplorationLead (camelCase,
// following referenceDTO's conventions): DB source_reference_id/
// connected_reference_id -> sourceReferenceId/connectedReferenceId.
type explorationLeadDTO struct {
	ID                   string  `json:"id"`
	Text                 string  `json:"text"`
	Status               string  `json:"status"`
	Origin               string  `json:"origin"`
	SourceReferenceID    *string `json:"sourceReferenceId"`
	ConnectedReferenceID *string `json:"connectedReferenceId"`
	Position             int32   `json:"position"`
}

func toExplorationLeadDTO(row sqlc.ExplorationLead) explorationLeadDTO {
	return explorationLeadDTO{
		ID:                   row.ID.String(),
		Text:                 row.Text,
		Status:               row.Status,
		Origin:               row.Origin,
		SourceReferenceID:    pgUUIDToStringPtr(row.SourceReferenceID),
		ConnectedReferenceID: pgUUIDToStringPtr(row.ConnectedReferenceID),
		Position:             row.Position,
	}
}

var validLeadStatus = map[string]bool{"open": true, "connected": true, "pruned": true}

// -- GET /exploration -----------------------------------------------------

// getExploration returns every lead for the project plus danglingSourceIds —
// no spend, purely a projection over ListExplorationLeads + ListReferences.
func (a *API) getExploration(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	leads, err := a.d.Queries.ListExplorationLeads(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	refs, err := a.d.Queries.ListReferences(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	dtos := make([]explorationLeadDTO, 0, len(leads))
	for _, l := range leads {
		dtos = append(dtos, toExplorationLeadDTO(l))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"leads":             dtos,
		"danglingSourceIds": computeDanglingSourceIds(refs, leads),
	})
}

// computeDanglingSourceIds is a PURE function (spec §4a): a reference is
// dangling iff it has an engaged material (material_id valid — the student
// has actually opened it), its decision is undecided or "drop" (decision
// "use"/"maybe" means the student has already made something of it), AND its
// id is not the connected_reference_id of any lead whose status != "pruned"
// (a pruned connection doesn't count — the student explicitly abandoned that
// thread, so the source is dangling again). Returns ids in stable order
// (refs' own ListReferences order).
func computeDanglingSourceIds(refs []sqlc.Reference, leads []sqlc.ExplorationLead) []string {
	connected := map[string]bool{}
	for _, l := range leads {
		if l.Status == "pruned" {
			continue
		}
		if l.ConnectedReferenceID.Valid {
			connected[uuid.UUID(l.ConnectedReferenceID.Bytes).String()] = true
		}
	}
	out := []string{}
	for _, ref := range refs {
		if !ref.MaterialID.Valid {
			continue
		}
		if ref.Decision != nil && *ref.Decision != "drop" {
			continue
		}
		id := ref.ID.String()
		if connected[id] {
			continue
		}
		out = append(out, id)
	}
	return out
}

// -- POST /exploration/leads ------------------------------------------------

// createExplorationLead adds a manual lead: origin "manual", status "open",
// appended after whatever already exists.
func (a *API) createExplorationLead(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	var body struct {
		Text string `json:"text"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if strings.TrimSpace(body.Text) == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "线索不能是空的", nil))
		return
	}
	existing, err := a.d.Queries.ListExplorationLeads(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	row, err := a.d.Queries.CreateExplorationLead(r.Context(), sqlc.CreateExplorationLeadParams{
		ProjectID: projectID,
		Text:      body.Text,
		Status:    "open",
		Origin:    "manual",
		Position:  int32(len(existing)),
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"lead": toExplorationLeadDTO(row)})
}

// -- PATCH /exploration/leads/{lid} -----------------------------------------

// patchExplorationLead partial-merges { text?, status?, connectedReferenceId? }
// over the current row — connect (status:"connected" + connectedReferenceId),
// prune (status:"pruned"), reopen (status:"open", + null the connection), or
// a plain text edit. status must be a valid enum member when present;
// connectedReferenceId, when present & non-null, must resolve to a reference
// in this project (IDOR guard), else 400.
func (a *API) patchExplorationLead(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	lid, err := uuid.Parse(r.PathValue("lid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	cur, err := a.d.Queries.GetExplorationLeadForProject(r.Context(), sqlc.GetExplorationLeadForProjectParams{ID: lid, ProjectID: projectID})
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}

	// text/status use absent-means-keep pointers; connectedReferenceId uses
	// json.RawMessage so a present-null can clear the connection (reopen),
	// which a plain pointer can't distinguish from "absent" (patchReference's
	// pattern, workspace_library.go).
	var body struct {
		Text                 *string         `json:"text"`
		Status               *string         `json:"status"`
		ConnectedReferenceID json.RawMessage `json:"connectedReferenceId"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	next := sqlc.UpdateExplorationLeadParams{
		ID:                   lid,
		ProjectID:            projectID,
		Text:                 cur.Text,
		Status:               cur.Status,
		ConnectedReferenceID: cur.ConnectedReferenceID,
		Position:             cur.Position,
	}
	if body.Text != nil {
		next.Text = *body.Text
	}
	if body.Status != nil {
		if !validLeadStatus[*body.Status] {
			httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "status 只能是 open/connected/pruned", nil))
			return
		}
		next.Status = *body.Status
	}
	if body.ConnectedReferenceID != nil {
		pg, err := parseNullableUUID(body.ConnectedReferenceID)
		if err != nil {
			httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "connectedReferenceId 不是有效的 id", nil))
			return
		}
		if pg.Valid {
			if _, err := a.d.Queries.GetReferenceForProject(r.Context(), sqlc.GetReferenceForProjectParams{
				ID: uuid.UUID(pg.Bytes), ProjectID: projectID,
			}); err != nil {
				httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "connectedReferenceId 不是这个项目里的来源", nil))
				return
			}
		}
		next.ConnectedReferenceID = pg
	}

	row, err := a.d.Queries.UpdateExplorationLead(r.Context(), next)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"lead": toExplorationLeadDTO(row)})
}

// -- DELETE /exploration/leads/{lid} ----------------------------------------

// deleteExplorationLead hard-deletes a lead (manual/mistaken adds). IDOR-
// guarded by the WHERE id AND project_id in the query itself, but we still
// 404 up front via GetExplorationLeadForProject so a stray id in another
// project can't even be probed for existence.
func (a *API) deleteExplorationLead(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	lid, err := uuid.Parse(r.PathValue("lid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	if _, err := a.d.Queries.GetExplorationLeadForProject(r.Context(), sqlc.GetExplorationLeadForProjectParams{ID: lid, ProjectID: projectID}); err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	if err := a.d.Queries.DeleteExplorationLead(r.Context(), sqlc.DeleteExplorationLeadParams{ID: lid, ProjectID: projectID}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
