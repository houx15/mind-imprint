package api

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// citation.go — G3 · record a source→section citation link when the student
// inserts a material into a specific essay section. Piggybacks the writing
// room's insert-at-caret action (no new UI); the (deferred) generator resolves
// a material's `usedIn` to the essay claim it supported, not just the
// exploration question it hung under. Respects the light-writing 铁律: this
// records provenance, it does not author or place anything.

// postCitation handles POST /projects/{id}/citations with body
// {referenceId, section}. Verifies the reference belongs to this project (IDOR-
// safe: a foreign/unknown reference 404s), then appends the link. `section`
// uses the machine convention `claim:<uuid>` / `subq:<id>` / `prop:<step>`.
func (a *API) postCitation(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	var body struct {
		ReferenceID string `json:"referenceId"`
		Section     string `json:"section"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	refID, perr := uuid.Parse(body.ReferenceID)
	if perr != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_request", "referenceId 无效", nil))
		return
	}
	if body.Section == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_request", "section 不能为空", nil))
		return
	}

	// Ownership: the reference must belong to THIS project — a foreign or
	// unknown id is a not-found, never a cross-project write.
	if _, err := a.d.Queries.GetReferenceForProject(r.Context(), sqlc.GetReferenceForProjectParams{
		ID: refID, ProjectID: projectID,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
			return
		}
		httpx.WriteError(w, r, err)
		return
	}

	row, err := a.d.Queries.CreateCitation(r.Context(), sqlc.CreateCitationParams{
		ProjectID: projectID, ReferenceID: refID, Section: body.Section,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"id": row.ID.String()})
}
