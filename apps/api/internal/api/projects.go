package api

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

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
// as not-found, never 403, so project existence doesn't leak.
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

// workspaceProposal is the four kick-off dimensions, zero-valued when absent.
type workspaceProposal struct {
	Objective  string `json:"objective"`
	Reason     string `json:"reason"`
	Activities string `json:"activities"`
	Resources  string `json:"resources"`
}

// workspaceProjection is the lean shape returned by GET /projects/{id}. The four
// rooms fetch their own heavier data; this projection carries only title,
// qualification and the proposal (the workspace shell needs nothing more).
type workspaceProjection struct {
	ID            string            `json:"id"`
	Title         string            `json:"title"`
	Qualification string            `json:"qualification"`
	Proposal      workspaceProposal `json:"proposal"`
}

// getProject returns the lean WorkspaceProjection for one project.
func (a *API) getProject(w http.ResponseWriter, r *http.Request) {
	id, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	p, err := a.d.Queries.GetProject(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	prop := workspaceProposal{}
	row, err := a.d.Queries.GetProjectProposal(r.Context(), id)
	switch {
	case err == nil:
		prop = workspaceProposal{
			Objective:  row.Objective,
			Reason:     row.Reason,
			Activities: row.Activities,
			Resources:  row.Resources,
		}
	case errors.Is(err, pgx.ErrNoRows):
		// no proposal yet — leave zero-value {"","","",""}
	default:
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, workspaceProjection{
		ID:            p.ID.String(),
		Title:         p.Title,
		Qualification: p.Qualification,
		Proposal:      prop,
	})
}
