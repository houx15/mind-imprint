package api

// project_writing_finish.go — Slice 5 (#20) · the 完成写作 milestone. The
// two-stage 写作→回顾 flow: finishing writing locks the draft read-only and
// unlocks the 回顾 room. This is a separate timestamp (project.writing_finished_at),
// NOT a project.status change — the lifecycle enum (active/evaluating/finished)
// is untouched. Reversible (铁律②: 重新打开写作 — never trap a student who
// finished by accident).

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// finishWriting sets the 完成写作 milestone for the requested document
// (?doc=proposal|essay, Phase B). Ownership-gated. Guard: that document's draft
// buffer must carry real content (an empty draft can't be "finished").
// Idempotent — if the milestone is already set, it returns the current state
// without error. No LLM spend.
func (a *API) finishWriting(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	doc := docKindParam(r)

	// Idempotent: already finished → no-op, return the current milestone.
	if _, ferr := a.d.Queries.GetWritingFinish(r.Context(), sqlc.GetWritingFinishParams{ProjectID: projectID, DocKind: doc}); ferr == nil {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"writingFinished": true})
		return
	} else if !errors.Is(ferr, pgx.ErrNoRows) {
		httpx.WriteError(w, r, ferr)
		return
	}

	// Guard: the draft must have real content before it can be locked. Read the
	// edit buffer server-side (never trust the client); no row / whitespace-only
	// → 422 so the student can't finish an empty draft.
	content, berr := a.d.Queries.GetEditBuffer(r.Context(), sqlc.GetEditBufferParams{ProjectID: projectID, DocKind: doc})
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

	if err := a.d.Queries.SetWritingFinish(r.Context(), sqlc.SetWritingFinishParams{ProjectID: projectID, DocKind: doc}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// 过程即数据: locking a draft is a headline milestone. Log it (per document)
	// so the 活动日志 records 完成提案 / 完成正文, not only framework/plan events.
	// The idempotent early-return above means this fires exactly once per doc.
	milestone := "完成研究提案初稿"
	if doc == "essay" {
		milestone = "完成论文正文、锁定初稿"
	}
	if err := a.appendAutoLog(r.Context(), a.d.Queries, projectID, milestone); err != nil {
		slog.Warn("finish-writing: append auto-log failed", "err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
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
	if err := a.d.Queries.ClearWritingFinish(r.Context(), sqlc.ClearWritingFinishParams{ProjectID: projectID, DocKind: docKindParam(r)}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"writingFinished": false})
}
