package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// workspace_review.go — Slice 5 (Review room): the student's own five-dimension
// reflection doc (plain owned-project REST, no model call). Nothing here is
// graded to the student.

// -- Reflection doc ---------------------------------------------------------

// reflectionDTO is the wire shape: an up-to-five-entry string array + done.
type reflectionDTO struct {
	Answers []string `json:"answers"`
	Done    bool     `json:"done"`
}

// decodeReflectionAnswers reads the stored jsonb string array; a malformed or
// absent blob reads as an empty slice (never fails the read).
func decodeReflectionAnswers(raw []byte) []string {
	answers := []string{}
	if len(raw) == 0 {
		return answers
	}
	if err := json.Unmarshal(raw, &answers); err != nil {
		return []string{}
	}
	return answers
}

// getReflection returns the project's reflection answers + done, or the empty
// state (no answers, not done) when no row exists yet.
func (a *API) getReflection(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	row, err := a.d.Queries.GetProjectReflection(r.Context(), projectID)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteJSON(w, http.StatusOK, reflectionDTO{Answers: []string{}, Done: false})
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, reflectionDTO{Answers: decodeReflectionAnswers(row.Answers), Done: row.Done})
}

// putReflection upserts the reflection answers + done. done is optional in the
// body: absent → the current done value is preserved (an autosave of the
// answers must not silently un-finish a completed review). On done flipping
// from false to true, one auto-log line ("完成回顾") is dropped.
func (a *API) putReflection(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	var body struct {
		Answers []string `json:"answers"`
		Done    *bool    `json:"done"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if body.Answers == nil {
		body.Answers = []string{}
	}

	// Read the prior done so the auto-log fires exactly once, on the false→true
	// transition, and so an answers-only PUT preserves the existing done.
	prevDone := false
	if prev, gerr := a.d.Queries.GetProjectReflection(r.Context(), projectID); gerr == nil {
		prevDone = prev.Done
	} else if !errors.Is(gerr, pgx.ErrNoRows) {
		httpx.WriteError(w, r, gerr)
		return
	}
	nextDone := prevDone
	if body.Done != nil {
		nextDone = *body.Done
	}

	answersJSON, merr := json.Marshal(body.Answers)
	if merr != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	row, err := a.d.Queries.UpsertProjectReflection(r.Context(), sqlc.UpsertProjectReflectionParams{
		ProjectID: projectID, Answers: answersJSON, Done: nextDone,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !prevDone && nextDone {
		if err := a.appendAutoLog(r.Context(), a.d.Queries, projectID, "完成回顾"); err != nil {
			slog.Warn("reflection: append auto-log failed", "err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
		}
	}
	httpx.WriteJSON(w, http.StatusOK, reflectionDTO{Answers: decodeReflectionAnswers(row.Answers), Done: row.Done})
}
