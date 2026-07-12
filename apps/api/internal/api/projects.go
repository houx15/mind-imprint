package api

import (
	"net/http"

	"github.com/google/uuid"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/skills"
	"mindimprint/api/internal/studio"
)

// projectListItem is the summary shape returned by GET /projects.
type projectListItem struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	QualLabel     string `json:"qualLabel"`
	ActiveStation string `json:"activeStation"`
}

// listProjects returns the caller's projects with enough state to render the
// projects list (title, qualification, active station).
func (a *API) listProjects(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	rows, err := a.d.Queries.ListProjectsByUser(r.Context(), u.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	sk, _ := skills.ByID("writing-project")
	out := make([]projectListItem, 0, len(rows))
	for _, p := range rows {
		d, err := studio.Load(r.Context(), a.d.Queries, p.ID)
		if err != nil {
			continue
		}
		proj, err := studio.Project(sk, a.d.SpecByID, d)
		if err != nil {
			continue
		}
		out = append(out, projectListItem{
			ID:            p.ID.String(),
			Title:         p.Title,
			QualLabel:     p.Qualification,
			ActiveStation: proj.ActiveStation,
		})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"projects": out})
}

// loadOwnedProject parses {id} and confirms the request user owns it. On any
// failure it writes a 404 envelope and returns ok=false — ownership is hidden
// as not-found, never 403, so project existence doesn't leak (mirrors
// loadOwnedTask in tasks.go).
func (a *API) loadOwnedProject(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	u, _ := UserFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return uuid.UUID{}, false
	}
	p, err := a.d.Queries.GetProject(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err) // pgx.ErrNoRows → 404
		return uuid.UUID{}, false
	}
	if p.UserID != u.ID {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return uuid.UUID{}, false
	}
	return id, true
}

// getProject returns the full StudioProjection for one project.
func (a *API) getProject(w http.ResponseWriter, r *http.Request) {
	id, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	d, err := studio.Load(r.Context(), a.d.Queries, id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	sk, _ := skills.ByID("writing-project")
	proj, err := studio.Project(sk, a.d.SpecByID, d)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, proj)
}
