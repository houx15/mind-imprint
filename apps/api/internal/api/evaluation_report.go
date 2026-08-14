package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"mindimprint/api/internal/evalreport"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// getEvaluationReport: read-no-call. Returns the three-state envelope — JSON
// null when no row exists yet, {"status":"generating"|"failed"}, or
// {"status":"ready","report":{…}} — never a 404.
func (a *API) getEvaluationReport(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	row, err := a.d.Queries.GetEvaluationReport(r.Context(), projectID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, err)
		return
	}
	a.writeEvalReportEnvelope(w, r, row, err == nil)
}

// getStudentEvaluationReport handles GET
// /api/v1/classes/{id}/students/{userId}/evaluation-report/{projectId}: the
// teacher's read of the SAME EvaluationReport the student sees for one of
// their projects. Read-no-call, like getEvaluationReport — never generates;
// returns JSON null when the report hasn't been generated yet. Guarded by
// authTeacherStudent (class ownership + this-class student membership) plus
// an explicit project-ownership check, so a correct projectId belonging to a
// DIFFERENT student (or a different class entirely) still 404s — same
// IDOR-safe not-found idiom as loadOwnedProject.
func (a *API) getStudentEvaluationReport(w http.ResponseWriter, r *http.Request) {
	_, userID, ok := a.authTeacherStudent(w, r)
	if !ok {
		return
	}
	projectID, err := uuid.Parse(r.PathValue("projectId"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	p, err := a.d.Queries.GetProject(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err) // pgx.ErrNoRows → 404
		return
	}
	if p.UserID != userID {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	row, err := a.d.Queries.GetEvaluationReport(r.Context(), projectID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, err)
		return
	}
	a.writeEvalReportEnvelope(w, r, row, err == nil)
}

// writeEvalReportEnvelope renders the three-state read contract.
// hadRow=false (no row) → JSON null.
// generating/failed → {"status":"..."}.
// ready → {"status":"ready","report":{…}}.
func (a *API) writeEvalReportEnvelope(w http.ResponseWriter, r *http.Request, row sqlc.EvaluationReport, hadRow bool) {
	if !hadRow {
		httpx.WriteJSON(w, http.StatusOK, nil)
		return
	}
	switch row.Status {
	case "ready":
		rep, verr := evalreport.Validate(row.Report)
		if verr != nil {
			httpx.WriteError(w, r, verr)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"status": "ready", "report": rep})
	default: // generating | failed
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"status": row.Status})
	}
}

// listEvaluationReports: the student's report timeline — newest report per
// project they own that has one. No model call.
func (a *API) listEvaluationReports(w http.ResponseWriter, r *http.Request) {
	u, ok := UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, r, httpx.ErrUnauthorized("未登录"))
		return
	}
	rows, err := a.d.Queries.ListEvaluationReports(r.Context(), u.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	type entry struct {
		ProjectID string `json:"projectId"`
		Title     string `json:"title"`
		Type      string `json:"type"`
		CreatedAt string `json:"createdAt"`
	}
	out := make([]entry, 0, len(rows))
	for _, row := range rows {
		out = append(out, entry{
			ProjectID: row.ProjectID.String(),
			Title:     row.Title,
			Type:      row.Qualification,
			CreatedAt: row.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"entries": out})
}

// postGenerateEvaluationReport: claims-and-generates if no row exists yet
// (safe under the atomic ON CONFLICT claim — can never double-fire even under
// concurrent callers); if a report is already generating/ready/failed, the
// claim is a no-op and this just re-reads and returns the current envelope.
func (a *API) postGenerateEvaluationReport(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	if err := a.generateAndStoreEvaluationReport(r.Context(), projectID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	a.getEvaluationReport(w, r)
}

// generateAndStoreEvaluationReport is the write core shared by the POST
// generate endpoint and the project-finish flow. It claims generation
// atomically (ClaimEvaluationReportGeneration) — a pgx.ErrNoRows result means
// some other caller already owns generation or a ready/fresh-generating row
// already exists, so this returns nil without doing anything (never
// double-generates). Once claimed, any error after the claim fails the row
// (FailEvaluationReport) so it becomes re-claimable rather than stuck.
//
// TODAY: emits evalreport.Placeholder — a full, deterministic fixture report,
// standing in for the real generation algorithm. LATER: this resolves
// a.d.EvalResolver, calls the real report-generation agent, and records the
// LLM call's cost — keeping this exact signature so both callers are
// unaffected by the swap.
func (a *API) generateAndStoreEvaluationReport(ctx context.Context, projectID uuid.UUID) error {
	if _, err := a.d.Queries.ClaimEvaluationReportGeneration(ctx, projectID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil // already owned/generated — never double-generate
		}
		return err
	}

	p, err := a.d.Queries.GetProject(ctx, projectID)
	if err != nil {
		a.d.Queries.FailEvaluationReport(ctx, projectID)
		return err
	}

	rep := evalreport.Placeholder(
		projectID.String(),
		uuid.NewString(),
		p.UserID.String(),
		"", // student name — placeholder era leaves it blank; filled from users table later
		p.Title,
		p.Qualification,
		time.Now().UTC().Format(time.RFC3339),
	)
	raw, err := json.Marshal(rep)
	if err != nil {
		a.d.Queries.FailEvaluationReport(ctx, projectID)
		return err
	}

	if err := a.d.Queries.CompleteEvaluationReport(ctx, sqlc.CompleteEvaluationReportParams{
		ProjectID: projectID, Report: raw,
	}); err != nil {
		a.d.Queries.FailEvaluationReport(ctx, projectID)
		return err
	}
	return nil
}
