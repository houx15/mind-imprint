package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// projectListItem is the summary shape returned by GET /projects.
type projectListItem struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	QualLabel string `json:"qualLabel"`
	Status    string `json:"status"`
	// Cover is the raw stored value ("img:<n>" / "grad:<name>" / "" when
	// unset); CoverURL is its signed CDN URL for "img:" covers, "" otherwise
	// (gradients render client-side from the name, see resolveCoverURL).
	Cover    string `json:"cover"`
	CoverURL string `json:"coverUrl"`
	// CreatedAt (RFC3339) — the project's start date, shown on the project list
	// (title · 开始于 <date> · status). Sorting stays by last_active_at (the query),
	// this is just the displayed calendar anchor.
	CreatedAt string `json:"createdAt"`
	// LastActiveAt (RFC3339) — the project's most-recent-activity timestamp; also
	// the list's sort key. Surfaced on the card as the 最近 chip.
	LastActiveAt string `json:"lastActiveAt"`
	// AICalls / ActivityLog — per-project totals shown as card chips. Both come
	// from ONE grouped query each over the caller's whole list (never per-project
	// reads): AICalls = rows in llm_call (真实 AI 调用), ActivityLog = rows in
	// activity_log_entry.
	AICalls     int `json:"aiCalls"`
	ActivityLog int `json:"activityLog"`
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

// frameworkReadyForPlan (slice 3a, Finding 2) gates plan generation on the four
// required dims AND that 可能的反例 (counterpoints) has either been articulated OR
// explicitly waived. This inserts one 反例 prompt into the framework before the
// plan generates — non-blocking (铁律②): the student can waive and proceed.
func frameworkReadyForPlan(p sqlc.ProjectProposal, waived bool) bool {
	return allRequiredDims(p) && (strings.TrimSpace(p.Counterpoints) != "" || waived)
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
// projects list (title, qualification, status, cover, start date).
//
// This handler used to run a full studio projection (studio.Load + studio.Project,
// ~14 DB queries EACH) per project just to read one field — the active station
// code. That was an N+1 that grew with the catalog and made the list slow; the
// station code was also the unreadable "S1" leaking into the card. Both are gone:
// the list now needs only the row itself plus deriveDisplayStatus's two cheap
// reads per project.
func (a *API) listProjects(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	rows, err := a.d.Queries.ListProjectsByUser(r.Context(), u.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// Per-project card counts, each in ONE grouped query over the caller's whole
	// list (best-effort — a count error just leaves that chip at 0, never fails
	// the list). Keyed by project id string so pgtype.UUID (llm_call) and
	// uuid.UUID (activity_log) fold into the same lookup.
	aiCalls := map[string]int{}
	if crows, err := a.d.Queries.CountLLMCallsByUserProject(r.Context(), u.ID); err == nil {
		for _, c := range crows {
			aiCalls[uuid.UUID(c.ProjectID.Bytes).String()] = int(c.N)
		}
	}
	activityLog := map[string]int{}
	if crows, err := a.d.Queries.CountActivityLogByUserProject(r.Context(), u.ID); err == nil {
		for _, c := range crows {
			activityLog[c.ProjectID.String()] = int(c.N)
		}
	}

	out := make([]projectListItem, 0, len(rows))
	for _, p := range rows {
		cover := derefOr(p.Cover, "")
		id := p.ID.String()
		out = append(out, projectListItem{
			ID:           id,
			Title:        p.Title,
			QualLabel:    p.Qualification,
			Status:       a.deriveDisplayStatus(r.Context(), p.ID, p.Status),
			Cover:        cover,
			CoverURL:     a.resolveCoverURL(cover),
			CreatedAt:    p.CreatedAt.Format(time.RFC3339),
			LastActiveAt: p.LastActiveAt.Format(time.RFC3339),
			AICalls:      aiCalls[id],
			ActivityLog:  activityLog[id],
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
	// Demo projects are shared, read-only content (guided-tour P2): any
	// non-GET request against one is rejected here, before it reaches ~140
	// mutating handlers that all funnel through this chokepoint.
	if p.IsDemo && r.Method != http.MethodGet {
		httpx.WriteError(w, r, httpx.ErrDemoReadonly())
		return uuid.Nil, false
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
	// WritingFinished (#20) — the ESSAY's 完成写作 milestone: true once the final
	// paper is locked read-only and the 回顾 room is unlocked. ReviewBlock's
	// view-only gate reads this (the essay is the paper that gates 定稿).
	WritingFinished bool `json:"writingFinished"`
	// WritingFinish (Phase B) — the per-document 完成写作 state. WritingBlock reads
	// the entry for its ACTIVE doc to lock that document read-only, so the
	// proposal and the essay lock independently.
	WritingFinish writingFinishState `json:"writingFinish"`
	// IsDemo (guided-tour P2) — true for the shared, read-only demo project.
	// The SPA uses this to hide/disable write affordances during the tour.
	IsDemo bool `json:"isDemo"`
}

// writingFinishState mirrors the writing_finish table for the two documents.
type writingFinishState struct {
	Proposal bool `json:"proposal"`
	Essay    bool `json:"essay"`
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
	// Per-document 完成写作 state (Phase B). One row per finished doc; absent = not
	// finished. WritingFinished (essay) drives the ReviewBlock gate.
	finish := writingFinishState{}
	if rows, ferr := a.d.Queries.ListWritingFinish(r.Context(), id); ferr == nil {
		for _, row := range rows {
			switch row.DocKind {
			case string(agent.DocProposal):
				finish.Proposal = true
			case string(agent.DocEssay):
				finish.Essay = true
			}
		}
	}
	httpx.WriteJSON(w, http.StatusOK, workspaceProjection{
		ID:              p.ID.String(),
		Title:           p.Title,
		Qualification:   p.Qualification,
		Proposal:        prop,
		Status:          displayStatus(p.Status, hasProposal, hasPlan),
		CreatedAt:       p.CreatedAt.Format(time.RFC3339),
		WritingFinished: finish.Essay,
		WritingFinish:   finish,
		IsDemo:          p.IsDemo,
	})
}

// renameProject lets a student change their own project's title. Ownership is
// enforced by loadOwnedProject (hidden as 404); only the title is mutable here.
func (a *API) renameProject(w http.ResponseWriter, r *http.Request) {
	id, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	var req struct {
		Title string `json:"title"`
	}
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("missing_title", "项目名称不能为空。", nil))
		return
	}
	if len([]rune(title)) > 200 {
		title = string([]rune(title)[:200])
	}
	if err := a.d.Queries.SetProjectTitle(r.Context(), sqlc.SetProjectTitleParams{ID: id, Title: title}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"title": title})
}
