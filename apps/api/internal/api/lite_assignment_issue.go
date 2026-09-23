package api

import (
	"context"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

type liteAssignmentIssueDTO struct {
	UserID      string `json:"userId"`
	DisplayName string `json:"displayName"`
	Detail      string `json:"detail"`
	ReportedAt  string `json:"reportedAt"`
}

func (a *API) saveLiteAssignmentIssue(ctx context.Context, assignmentID, userID uuid.UUID, detail string) error {
	tx, err := a.d.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `INSERT INTO lite_assignment_issue (assignment_id, user_id, detail)
		VALUES ($1, $2, $3) ON CONFLICT (assignment_id, user_id)
		DO UPDATE SET detail = EXCLUDED.detail, reported_at = now(), resolved_at = NULL`, assignmentID, userID, detail)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// A student can report a material issue to the teacher who assigned it.
func (a *API) reportLiteAssignmentIssue(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	aid, err := uuid.Parse(r.PathValue("aid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	as, err := a.d.Queries.GetLiteAssignment(r.Context(), aid)
	if err != nil {
		writeNotFoundOr(w, r, err)
		return
	}
	if as.ArchivedAt.Valid || as.Kind != "reading" {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	if _, err := a.d.Queries.GetLiteAssignmentRecipient(r.Context(), sqlc.GetLiteAssignmentRecipientParams{AssignmentID: aid, UserID: u.ID}); err != nil {
		writeNotFoundOr(w, r, err)
		return
	}
	enrolled, err := a.d.Queries.IsEnrolledStudent(r.Context(), sqlc.IsEnrolledStudentParams{ClassID: as.ClassID, UserID: u.ID})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !enrolled {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	var req struct {
		Detail string `json:"detail"`
	}
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	detail := strings.TrimSpace(req.Detail)
	if detail == "" || utf8.RuneCountInString(detail) > 500 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_detail", "请填写 500 字以内的材料问题", nil))
		return
	}
	if err := a.saveLiteAssignmentIssue(r.Context(), aid, u.ID, detail); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) liteAssignmentIssues(ctx context.Context, aid uuid.UUID) ([]liteAssignmentIssueDTO, error) {
	tx, err := a.d.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT i.user_id, u.display_name, i.detail, i.reported_at
		FROM lite_assignment_issue i JOIN users u ON u.id = i.user_id
		WHERE i.assignment_id = $1 AND i.resolved_at IS NULL ORDER BY i.reported_at DESC`, aid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]liteAssignmentIssueDTO, 0)
	for rows.Next() {
		var item liteAssignmentIssueDTO
		var userID uuid.UUID
		var reportedAt time.Time
		if err := rows.Scan(&userID, &item.DisplayName, &item.Detail, &reportedAt); err != nil {
			return nil, err
		}
		item.UserID = userID.String()
		item.ReportedAt = reportedAt.Format(time.RFC3339)
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
