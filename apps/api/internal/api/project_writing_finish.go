package api

// project_writing_finish.go — Slice 5 (#20) · the 完成写作 milestone. The
// two-stage 写作→回顾 flow: finishing writing locks the draft read-only and
// unlocks the 回顾 room. This is a separate timestamp (project.writing_finished_at),
// NOT a project.status change — the lifecycle enum (active/evaluating/finished)
// is untouched. Reversible (铁律②: 重新打开写作 — never trap a student who
// finished by accident).

import (
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"

	"mindimprint/api/internal/httpx"
)

// finishWriting sets the 完成写作 milestone. Ownership-gated. Guard: the draft
// buffer must carry real content (an empty draft can't be "finished"). Idempotent
// — if the milestone is already set, it returns the current state without error.
// No LLM spend.
func (a *API) finishWriting(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}

	proj, err := a.d.Queries.GetProject(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// Idempotent: already finished → no-op, return the current milestone.
	if proj.WritingFinishedAt.Valid {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"writingFinished": true})
		return
	}

	// Guard: the draft must have real content before it can be locked. Read the
	// edit buffer server-side (never trust the client); no row / whitespace-only
	// → 422 so the student can't finish an empty draft.
	content, berr := a.d.Queries.GetEditBuffer(r.Context(), projectID)
	if errors.Is(berr, pgx.ErrNoRows) {
		content = ""
	} else if berr != nil {
		httpx.WriteError(w, r, berr)
		return
	}
	if strings.TrimSpace(content) == "" {
		httpx.WriteError(w, r, &httpx.APIError{
			Status: http.StatusUnprocessableEntity, Code: "draft_empty",
			Message: "正文还是空的，先写点东西再完成写作",
		})
		return
	}

	if err := a.d.Queries.SetProjectWritingFinished(r.Context(), projectID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"writingFinished": true})
}

// reopenWriting clears the 完成写作 milestone so the draft is editable again
// (铁律②). Only allowed while the project is still pre-evaluation — once the
// finish-and-evaluate path has started (evaluating/finished) the draft is
// terminally locked and reopening writing would be incoherent. No LLM spend.
func (a *API) reopenWriting(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	proj, err := a.d.Queries.GetProject(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if proj.Status == "evaluating" || proj.Status == "finished" {
		httpx.WriteError(w, r, &httpx.APIError{
			Status: http.StatusConflict, Code: "already_finalizing",
			Message: "项目已进入评估 / 归档，无法重新打开写作",
		})
		return
	}
	if err := a.d.Queries.ClearProjectWritingFinished(r.Context(), projectID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"writingFinished": false})
}
