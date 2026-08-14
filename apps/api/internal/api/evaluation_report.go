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

// getEvaluationReport: read-no-call. Returns the latest stored report or JSON
// null when none has been generated yet — a normal "not yet evaluated" state,
// never a 404.
func (a *API) getEvaluationReport(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	row, err := a.d.Queries.GetLatestEvaluationReport(r.Context(), projectID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteJSON(w, http.StatusOK, nil)
			return
		}
		httpx.WriteError(w, r, err)
		return
	}
	rep, verr := evalreport.Validate(row.Report)
	if verr != nil {
		httpx.WriteError(w, r, verr)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, rep)
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
	row, err := a.d.Queries.GetLatestEvaluationReport(r.Context(), projectID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteJSON(w, http.StatusOK, nil)
			return
		}
		httpx.WriteError(w, r, err)
		return
	}
	rep, verr := evalreport.Validate(row.Report)
	if verr != nil {
		httpx.WriteError(w, r, verr)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, rep)
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

// postGenerateEvaluationReport: first-open-wins generate. If a report already
// exists for this project, it is returned as-is — no regeneration, no spend.
func (a *API) postGenerateEvaluationReport(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	_, err := a.d.Queries.GetLatestEvaluationReport(r.Context(), projectID)
	if err == nil {
		a.getEvaluationReport(w, r) // already exists — return it, no spend
		return
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, err)
		return
	}
	// no row yet → generate
	if err := a.generateAndStoreEvaluationReport(r.Context(), projectID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	a.getEvaluationReport(w, r)
}

// generateAndStoreEvaluationReport is the write core shared by the POST
// generate endpoint and (later) the project-finish flow.
//
// TODAY: emits evalreport.Placeholder — a full, deterministic fixture report,
// standing in for the real generation algorithm. LATER: this resolves
// a.d.EvalResolver, calls the real report-generation agent, and records the
// LLM call's cost — keeping this exact signature so both callers are
// unaffected by the swap.
func (a *API) generateAndStoreEvaluationReport(ctx context.Context, projectID uuid.UUID) error {
	p, err := a.d.Queries.GetProject(ctx, projectID)
	if err != nil {
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
		return err
	}
	_, err = a.d.Queries.InsertEvaluationReport(ctx, sqlc.InsertEvaluationReportParams{
		ProjectID: projectID,
		Version:   1,
		Report:    raw,
	})
	return err
}
