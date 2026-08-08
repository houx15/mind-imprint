package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/skills"
	"mindimprint/api/internal/store/sqlc"
	"mindimprint/api/internal/studio"
)

// projectListItem is the summary shape returned by GET /projects.
type projectListItem struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	QualLabel     string `json:"qualLabel"`
	ActiveStation string `json:"activeStation"`
	Status        string `json:"status"`
	// Cover is the raw stored value ("img:<n>" / "grad:<name>" / "" when
	// unset); CoverURL is its signed CDN URL for "img:" covers, "" otherwise
	// (gradients render client-side from the name, see resolveCoverURL).
	Cover    string `json:"cover"`
	CoverURL string `json:"coverUrl"`
}

// anyProposalDim reports whether any of the four kick-off dimensions carries
// content — the "has the student started framing?" signal for displayStatus.
func anyProposalDim(p sqlc.ProjectProposal) bool {
	return strings.TrimSpace(p.Objective) != "" ||
		strings.TrimSpace(p.Reason) != "" ||
		strings.TrimSpace(p.Activities) != "" ||
		strings.TrimSpace(p.Resources) != ""
}

// allRequiredDims reports whether every one of the four required proposal
// dimensions (objective/reason/activities/resources — counterpoints is optional)
// is non-empty. The server-side plan-generation gate: the funnel auto-generates
// the plan the moment this becomes true (see reconcileStudioFunnel), so the coach
// model never has to decide to call generate_plan.
func allRequiredDims(p sqlc.ProjectProposal) bool {
	return strings.TrimSpace(p.Objective) != "" &&
		strings.TrimSpace(p.Reason) != "" &&
		strings.TrimSpace(p.Activities) != "" &&
		strings.TrimSpace(p.Resources) != ""
}

// displayStatus maps a persisted project.status to the four-state lifecycle the
// UI shows: finished→"done", evaluating→"evaluating", else (active) any framing
// started (a non-empty proposal dim OR ≥1 plan item)→"working", else "forming".
func displayStatus(status string, hasProposal, hasPlan bool) string {
	switch status {
	case "finished":
		return "done"
	case "evaluating":
		return "evaluating"
	default:
		if hasProposal || hasPlan {
			return "working"
		}
		return "forming"
	}
}

// deriveDisplayStatus reads the proposal + plan-item existence for one project
// and folds them through displayStatus. Two cheap reads per project — fine for
// the ≤10-project list; best-effort (a read error just reads as "absent").
func (a *API) deriveDisplayStatus(ctx context.Context, projectID uuid.UUID, status string) string {
	hasProposal := false
	if p, err := a.d.Queries.GetProjectProposal(ctx, projectID); err == nil {
		hasProposal = anyProposalDim(p)
	}
	hasPlan := false
	if items, err := a.d.Queries.ListPlanItems(ctx, projectID); err == nil && len(items) > 0 {
		hasPlan = true
	}
	return displayStatus(status, hasProposal, hasPlan)
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
		cover := derefOr(p.Cover, "")
		out = append(out, projectListItem{
			ID:            p.ID.String(),
			Title:         p.Title,
			QualLabel:     p.Qualification,
			ActiveStation: proj.ActiveStation,
			Status:        a.deriveDisplayStatus(r.Context(), p.ID, p.Status),
			Cover:         cover,
			CoverURL:      a.resolveCoverURL(cover),
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

// workspaceProposal is the four required kick-off dimensions plus the optional
// 5th section (counterpoints/反例), zero-valued when absent.
type workspaceProposal struct {
	Objective     string `json:"objective"`
	Reason        string `json:"reason"`
	Activities    string `json:"activities"`
	Resources     string `json:"resources"`
	Counterpoints string `json:"counterpoints"`
}

// workspaceProjection is the lean shape returned by GET /projects/{id}. The four
// rooms fetch their own heavier data; this projection carries only title,
// qualification and the proposal (the workspace shell needs nothing more).
type workspaceProjection struct {
	ID            string            `json:"id"`
	Title         string            `json:"title"`
	Qualification string            `json:"qualification"`
	Proposal      workspaceProposal `json:"proposal"`
	Status        string            `json:"status"`
	// CreatedAt (RFC3339) anchors the plan timeline to real calendar dates —
	// the Gantt date axis + today-line and the Kanban date markers (#14).
	CreatedAt string `json:"createdAt"`
	// WritingFinished (#20) — the 完成写作 milestone: true once the draft is locked
	// read-only and the 回顾 room is unlocked. Both rooms read it (WritingBlock's
	// read-only lock; ReviewBlock's view-only gate).
	WritingFinished bool `json:"writingFinished"`
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
	hasProposal := false
	row, err := a.d.Queries.GetProjectProposal(r.Context(), id)
	switch {
	case err == nil:
		prop = workspaceProposal{
			Objective:     row.Objective,
			Reason:        row.Reason,
			Activities:    row.Activities,
			Resources:     row.Resources,
			Counterpoints: row.Counterpoints,
		}
		hasProposal = anyProposalDim(row)
	case errors.Is(err, pgx.ErrNoRows):
		// no proposal yet — leave zero-value {"","","",""}
	default:
		httpx.WriteError(w, r, err)
		return
	}
	hasPlan := false
	if items, perr := a.d.Queries.ListPlanItems(r.Context(), id); perr == nil && len(items) > 0 {
		hasPlan = true
	}
	httpx.WriteJSON(w, http.StatusOK, workspaceProjection{
		ID:              p.ID.String(),
		Title:           p.Title,
		Qualification:   p.Qualification,
		Proposal:        prop,
		Status:          displayStatus(p.Status, hasProposal, hasPlan),
		CreatedAt:       p.CreatedAt.Format(time.RFC3339),
		WritingFinished: p.WritingFinishedAt.Valid,
	})
}
