package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

const maxReturnNoteRunes = 500

// returnLiteAssignmentRecipient handles
// POST /api/v1/lite/teacher/assignments/{aid}/recipients/{userId}/return (退回修改).
// Writing homework only; the student must have submitted at least one
// version; the new deadline must be in the future. Returning again
// overwrites returned_at, return_due_at and return_note.
func (a *API) returnLiteAssignmentRecipient(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	as, ok := a.loadTeacherAssignment(w, r)
	if !ok {
		return
	}
	userID, err := uuid.Parse(r.PathValue("userId"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	if as.Kind != "writing" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("not_writing_assignment", "只有写作作业可以退回修改", nil))
		return
	}
	var req struct {
		DueAt string `json:"dueAt"`
		Note  string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadJSON(err))
		return
	}
	due, err := parseAssignmentDueAt(req.DueAt)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !due.After(time.Now()) {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_due_at", "新的截止时间需要晚于现在", nil))
		return
	}
	note := strings.TrimSpace(req.Note)
	if utf8.RuneCountInString(note) > maxReturnNoteRunes {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_note", "退回说明不超过 500 字", nil))
		return
	}
	var notePtr *string
	if note != "" {
		notePtr = &note
	}

	tx, err := a.d.Pool.Begin(ctx)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := a.d.Queries.WithTx(tx)

	rc, err := qtx.GetLiteAssignmentRecipientForUpdate(ctx, sqlc.GetLiteAssignmentRecipientForUpdateParams{AssignmentID: as.ID, UserID: userID})
	if err != nil {
		writeNotFoundOr(w, r, err)
		return
	}
	noSubmission := &httpx.APIError{Status: http.StatusConflict, Code: "no_submission", Message: "这名学生还没有提交"}
	if !rc.AtomID.Valid {
		httpx.WriteError(w, r, noSubmission)
		return
	}
	n, err := qtx.CountWritingVersions(ctx, uuid.UUID(rc.AtomID.Bytes))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if n == 0 {
		httpx.WriteError(w, r, noSubmission)
		return
	}
	if _, err := qtx.SetLiteAssignmentReturned(ctx, sqlc.SetLiteAssignmentReturnedParams{
		AssignmentID: as.ID, UserID: userID,
		ReturnDueAt: pgtype.Timestamptz{Time: due, Valid: true}, ReturnNote: notePtr,
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	rows, err := a.d.Queries.ListLiteAssignmentRecipients(ctx, []uuid.UUID{as.ID})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	now := time.Now()
	for _, row := range rows {
		if row.UserID == userID {
			httpx.WriteJSON(w, http.StatusOK, map[string]any{"recipient": newRecipientDTO(row, as.DueAt, now)})
			return
		}
	}
	httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
}
