package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/liteassign"
	"mindimprint/api/internal/store/sqlc"
)

// AssignmentDTO is one lite assignment as the teacher end sees it.
type AssignmentDTO struct {
	ID           string          `json:"id"`
	ClassID      string          `json:"classId"`
	Kind         string          `json:"kind"`
	Title        string          `json:"title"`
	Instructions string          `json:"instructions"`
	Payload      json.RawMessage `json:"payload"`
	DueAt        string          `json:"dueAt"`
	CreatedAt    string          `json:"createdAt"`
}

// AssignmentSummaryDTO is a list row: the assignment plus a count per wire
// status. Every status key is present, zero included.
type AssignmentSummaryDTO struct {
	AssignmentDTO
	Counts map[string]int `json:"counts"`
}

// RecipientDTO is one student on an assignment. Status is derived on read.
type RecipientDTO struct {
	UserID      string  `json:"userId"`
	DisplayName string  `json:"displayName"`
	AvatarColor string  `json:"avatarColor"`
	Status      string  `json:"status"`
	StatusLabel string  `json:"statusLabel"`
	AtomID      *string `json:"atomId"`
	StartedAt   *string `json:"startedAt"`
	FinishedAt  *string `json:"finishedAt"`
	SeenAt      *string `json:"seenAt"`
	// Return fields are set once the teacher has returned the writing (0153).
	ReturnedAt   *string `json:"returnedAt"`
	ReturnDueAt  *string `json:"returnDueAt"`
	ReturnNote   *string `json:"returnNote"`
	VersionCount int     `json:"versionCount"`
	// Reading is this student's article on a personalized reading homework; nil otherwise.
	Reading *RecipientReadingDTO `json:"reading"`
}

// assignmentStatuses are the wire statuses liteassign.StatusWithReturn returns.
var assignmentStatuses = []string{"not_started", "in_progress", "done", "done_late", "overdue", "returned", "resubmitted"}

const maxAssignmentTitleRunes = 200

// tsPtr converts a nullable timestamp for liteassign.Status. Shared by the
// teacher and student assignment handlers.
func tsPtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}

// tsStringPtr formats a nullable timestamp as RFC3339, nil when unset.
func tsStringPtr(t pgtype.Timestamptz) *string {
	if !t.Valid {
		return nil
	}
	s := t.Time.Format(time.RFC3339)
	return &s
}

func uuidStringPtr(u pgtype.UUID) *string {
	if !u.Valid {
		return nil
	}
	s := uuid.UUID(u.Bytes).String()
	return &s
}

func newAssignmentDTO(as sqlc.LiteAssignment) AssignmentDTO {
	return AssignmentDTO{
		ID: as.ID.String(), ClassID: as.ClassID.String(), Kind: as.Kind,
		Title: as.Title, Instructions: as.Instructions,
		Payload: withEffectiveRubric(as.Kind, json.RawMessage(as.Payload)),
		DueAt:   as.DueAt.Format(time.RFC3339), CreatedAt: as.CreatedAt.Format(time.RFC3339),
	}
}

// withEffectiveRubric bakes the rubric a writing homework is graded with —
// the stored one, or the default for its lang — into the payload the teacher
// end reads. Every place this package shapes a teacher assignment payload
// goes through newAssignmentDTO, so the frontend never has to know the Go
// default rubric names and notes.
func withEffectiveRubric(kind string, payload json.RawMessage) json.RawMessage {
	if kind != "writing" {
		return payload
	}
	raw, err := json.Marshal(liteassign.EffectiveRubric(payload))
	if err != nil {
		return payload
	}
	withRubric, err := liteassign.ApplyRubric(payload, raw)
	if err != nil {
		return payload
	}
	return withRubric
}

// returnOf turns a recipient's return columns into liteassign's input; nil
// when the teacher never returned it.
func returnOf(returnedAt, returnDueAt pgtype.Timestamptz, resubmitted bool) *liteassign.Return {
	if !returnedAt.Valid || !returnDueAt.Valid {
		return nil
	}
	return &liteassign.Return{DueAt: returnDueAt.Time, Resubmitted: resubmitted}
}

func newRecipientDTO(row sqlc.ListLiteAssignmentRecipientsRow, dueAt, now time.Time) RecipientDTO {
	status := liteassign.StatusWithReturn(row.StartedAt.Valid, tsPtr(row.FinishedAt), dueAt, now,
		returnOf(row.ReturnedAt, row.ReturnDueAt, row.Resubmitted))
	return RecipientDTO{
		UserID: row.UserID.String(), DisplayName: row.DisplayName, AvatarColor: row.AvatarColor,
		Status: status, StatusLabel: liteassign.StatusLabel(status),
		AtomID: uuidStringPtr(row.AtomID), StartedAt: tsStringPtr(row.StartedAt),
		FinishedAt: tsStringPtr(row.FinishedAt), SeenAt: tsStringPtr(row.SeenAt),
		ReturnedAt: tsStringPtr(row.ReturnedAt), ReturnDueAt: tsStringPtr(row.ReturnDueAt),
		ReturnNote: row.ReturnNote, VersionCount: int(row.VersionCount),
	}
}

func payloadErrorResponse(err error) error {
	var pe *liteassign.PayloadError
	if errors.As(err, &pe) {
		return httpx.ErrBadRequest(pe.Code, pe.Message, nil)
	}
	return err
}

func errAssignmentStarted(msg string) *httpx.APIError {
	return &httpx.APIError{Status: http.StatusConflict, Code: "assignment_started", Message: msg}
}

func errPickNotRecipient() error {
	return httpx.ErrBadRequest("pick_not_recipient", "个性化名单中有学生不在这份作业的学生名单中", nil)
}

func parseAssignmentTitle(raw string) (string, error) {
	title := strings.TrimSpace(raw)
	if title == "" || utf8.RuneCountInString(title) > maxAssignmentTitleRunes {
		return "", httpx.ErrBadRequest("invalid_title", "请填写作业标题，不超过 200 字", nil)
	}
	return title, nil
}

func parseAssignmentDueAt(raw string) (time.Time, error) {
	due, err := time.Parse(time.RFC3339, strings.TrimSpace(raw))
	if err != nil {
		return time.Time{}, httpx.ErrBadRequest("invalid_due_at", "截止时间格式错误", nil)
	}
	return due, nil
}

// assignmentRecipientIDs parses and de-duplicates ids and checks each is an
// enrolled student of the class. An empty list is returned as empty; the
// caller decides whether that is allowed.
func (a *API) assignmentRecipientIDs(ctx context.Context, classID uuid.UUID, raw []string) ([]uuid.UUID, error) {
	bad := httpx.ErrBadRequest("invalid_recipient", "所选学生不是这个班级的学生", nil)
	seen := make(map[uuid.UUID]bool, len(raw))
	out := make([]uuid.UUID, 0, len(raw))
	for _, s := range raw {
		uid, err := uuid.Parse(strings.TrimSpace(s))
		if err != nil {
			return nil, bad
		}
		if seen[uid] {
			continue
		}
		seen[uid] = true
		enrolled, err := a.d.Queries.IsEnrolledStudent(ctx, sqlc.IsEnrolledStudentParams{ClassID: classID, UserID: uid})
		if err != nil {
			return nil, err
		}
		if !enrolled {
			return nil, bad
		}
		out = append(out, uid)
	}
	return out, nil
}

// loadTeacherAssignment loads {aid} and checks the caller teaches its class.
// A missing, archived or foreign assignment is 404.
func (a *API) loadTeacherAssignment(w http.ResponseWriter, r *http.Request) (sqlc.LiteAssignment, bool) {
	aid, err := uuid.Parse(r.PathValue("aid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return sqlc.LiteAssignment{}, false
	}
	as, err := a.d.Queries.GetLiteAssignment(r.Context(), aid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		} else {
			httpx.WriteError(w, r, err)
		}
		return sqlc.LiteAssignment{}, false
	}
	if as.ArchivedAt.Valid {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return sqlc.LiteAssignment{}, false
	}
	if _, err := a.assertTeacherOwnsClass(r.Context(), as.ClassID); err != nil {
		httpx.WriteError(w, r, err)
		return sqlc.LiteAssignment{}, false
	}
	return as, true
}

// createLiteAssignment handles POST /api/v1/lite/teacher/classes/{id}/assignments.
func (a *API) createLiteAssignment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u, _ := UserFromContext(ctx)
	classID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	if _, err := a.assertTeacherOwnsClass(ctx, classID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var req struct {
		Kind         string          `json:"kind"`
		Title        string          `json:"title"`
		Instructions string          `json:"instructions"`
		Payload      json.RawMessage `json:"payload"`
		DueAt        string          `json:"dueAt"`
		UserIDs      []string        `json:"userIds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadJSON(err))
		return
	}
	title, err := parseAssignmentTitle(req.Title)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	due, err := parseAssignmentDueAt(req.DueAt)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	instructions, err := liteassign.ValidateInstructions(req.Instructions)
	if err != nil {
		httpx.WriteError(w, r, payloadErrorResponse(err))
		return
	}
	payload, err := liteassign.ValidatePayload(req.Kind, req.Payload)
	if err != nil {
		httpx.WriteError(w, r, payloadErrorResponse(err))
		return
	}
	if len(req.UserIDs) == 0 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("no_recipients", "请至少选择一名学生", nil))
		return
	}
	ids, err := a.assignmentRecipientIDs(ctx, classID, req.UserIDs)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if liteassign.PicksOutside(payload, ids) {
		httpx.WriteError(w, r, errPickNotRecipient())
		return
	}

	tx, err := a.d.Pool.Begin(ctx)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := a.d.Queries.WithTx(tx)
	as, err := qtx.CreateLiteAssignment(ctx, sqlc.CreateLiteAssignmentParams{
		ClassID: classID, CreatedBy: u.ID, Kind: req.Kind, Title: title,
		Instructions: instructions, Payload: payload, DueAt: due,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	for _, uid := range ids {
		if err := qtx.AddLiteAssignmentRecipient(ctx, sqlc.AddLiteAssignmentRecipientParams{AssignmentID: as.ID, UserID: uid}); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}
	if err := tx.Commit(ctx); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"assignment": newAssignmentDTO(as)})
}

// listLiteAssignments handles GET /api/v1/lite/teacher/classes/{id}/assignments.
func (a *API) listLiteAssignments(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	classID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	if _, err := a.assertTeacherOwnsClass(ctx, classID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	rows, err := a.d.Queries.ListLiteAssignmentsByClass(ctx, classID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]AssignmentSummaryDTO, 0, len(rows))
	index := make(map[uuid.UUID]int, len(rows))
	ids := make([]uuid.UUID, 0, len(rows))
	for i, row := range rows {
		counts := make(map[string]int, len(assignmentStatuses))
		for _, s := range assignmentStatuses {
			counts[s] = 0
		}
		out = append(out, AssignmentSummaryDTO{
			AssignmentDTO: newAssignmentDTO(sqlc.LiteAssignment{
				ID: row.ID, ClassID: row.ClassID, Kind: row.Kind, Title: row.Title,
				Instructions: row.Instructions, Payload: row.Payload, DueAt: row.DueAt, CreatedAt: row.CreatedAt,
			}),
			Counts: counts,
		})
		index[row.ID] = i
		ids = append(ids, row.ID)
	}
	if len(ids) > 0 {
		recipients, err := a.d.Queries.ListLiteAssignmentRecipients(ctx, ids)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		now := time.Now()
		for _, rc := range recipients {
			i, ok := index[rc.AssignmentID]
			if !ok {
				continue
			}
			status := liteassign.StatusWithReturn(rc.StartedAt.Valid, tsPtr(rc.FinishedAt), rows[i].DueAt, now,
				returnOf(rc.ReturnedAt, rc.ReturnDueAt, rc.Resubmitted))
			out[i].Counts[status]++
		}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"assignments": out})
}

// getLiteAssignment handles GET /api/v1/lite/teacher/assignments/{aid}.
func (a *API) getLiteAssignment(w http.ResponseWriter, r *http.Request) {
	as, ok := a.loadTeacherAssignment(w, r)
	if !ok {
		return
	}
	rows, err := a.d.Queries.ListLiteAssignmentRecipients(r.Context(), []uuid.UUID{as.ID})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	readings, err := a.personalizedRecipientReadings(r.Context(), as.Kind, json.RawMessage(as.Payload), rows)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	now := time.Now()
	recipients := make([]RecipientDTO, 0, len(rows))
	for _, row := range rows {
		dto := newRecipientDTO(row, as.DueAt, now)
		dto.Reading = readings[row.UserID]
		recipients = append(recipients, dto)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"assignment": newAssignmentDTO(as), "recipients": recipients})
}

// samePayload compares two JSON payloads by value. The stored copy comes back
// from jsonb with its keys reordered, so a byte comparison would report a
// change the teacher did not make.
func samePayload(a, b []byte) bool {
	var va, vb any
	if json.Unmarshal(a, &va) != nil || json.Unmarshal(b, &vb) != nil {
		return false
	}
	return reflect.DeepEqual(va, vb)
}

// patchLiteAssignment handles PATCH /api/v1/lite/teacher/assignments/{aid}.
// Title, instructions and deadline stay editable. Kind and payload are locked
// once any recipient has started, and a started recipient cannot be removed.
// A form that resends unchanged settings is not a change.
func (a *API) patchLiteAssignment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	as, ok := a.loadTeacherAssignment(w, r)
	if !ok {
		return
	}
	var req struct {
		Title         *string         `json:"title"`
		Instructions  *string         `json:"instructions"`
		DueAt         *string         `json:"dueAt"`
		Kind          *string         `json:"kind"`
		Payload       json.RawMessage `json:"payload"`
		Rubric        json.RawMessage `json:"rubric"`
		AddUserIDs    []string        `json:"addUserIds"`
		RemoveUserIDs []string        `json:"removeUserIds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadJSON(err))
		return
	}
	var newTitle *string
	if req.Title != nil {
		title, err := parseAssignmentTitle(*req.Title)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		newTitle = &title
	}
	var newInstructions *string
	if req.Instructions != nil {
		instructions, err := liteassign.ValidateInstructions(*req.Instructions)
		if err != nil {
			httpx.WriteError(w, r, payloadErrorResponse(err))
			return
		}
		newInstructions = &instructions
	}
	var newDue *time.Time
	if req.DueAt != nil {
		due, err := parseAssignmentDueAt(*req.DueAt)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		newDue = &due
	}
	// class_id is never updated, so the unlocked read is enough for this check.
	addIDs, err := a.assignmentRecipientIDs(ctx, as.ClassID, req.AddUserIDs)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	removeIDs := make([]uuid.UUID, 0, len(req.RemoveUserIDs))
	for _, s := range req.RemoveUserIDs {
		uid, err := uuid.Parse(strings.TrimSpace(s))
		if err != nil {
			httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_recipient", "所选学生不是这个班级的学生", nil))
			return
		}
		removeIDs = append(removeIDs, uid)
	}

	tx, err := a.d.Pool.Begin(ctx)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := a.d.Queries.WithTx(tx)
	// Lock order, shared with startLiteAssignment: the assignment row first,
	// then recipient rows. A start in progress holds FOR SHARE on the
	// assignment until its item is linked, so this waits for it.
	//
	// Every decision below uses locked, never as: as was read before the lock
	// and may be stale (another PATCH may have changed the settings since).
	locked, err := qtx.GetLiteAssignmentForUpdate(ctx, as.ID)
	if err != nil {
		writeNotFoundOr(w, r, err)
		return
	}
	if locked.ArchivedAt.Valid {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	params := sqlc.UpdateLiteAssignmentParams{
		ID: locked.ID, Title: locked.Title, Instructions: locked.Instructions,
		DueAt: locked.DueAt, Kind: locked.Kind, Payload: locked.Payload,
	}
	if newTitle != nil {
		params.Title = *newTitle
	}
	if newInstructions != nil {
		params.Instructions = *newInstructions
	}
	if newDue != nil {
		params.DueAt = *newDue
	}
	settingsChanged := false
	if req.Kind != nil || req.Payload != nil {
		if req.Kind != nil {
			params.Kind = *req.Kind
		}
		raw := json.RawMessage(locked.Payload)
		if req.Payload != nil {
			raw = req.Payload
		}
		validated, err := liteassign.ValidatePayload(params.Kind, raw)
		if err != nil {
			httpx.WriteError(w, r, payloadErrorResponse(err))
			return
		}
		// The rubric changes only through req.Rubric. Carrying the stored one
		// keeps a resent form from counting as a settings change.
		if params.Kind == "writing" && locked.Kind == "writing" {
			if validated, err = liteassign.CarryRubric(validated, locked.Payload); err != nil {
				httpx.WriteError(w, r, payloadErrorResponse(err))
				return
			}
		}
		params.Payload = validated
		settingsChanged = params.Kind != locked.Kind || !samePayload(validated, locked.Payload)
	}
	// The rubric stays editable after students start: gradings keep their own copy.
	if req.Rubric != nil {
		if params.Kind != "writing" {
			httpx.WriteError(w, r, httpx.ErrBadRequest("rubric_not_writing", "只有写作作业可以设置评分标准", nil))
			return
		}
		withRubric, err := liteassign.ApplyRubric(params.Payload, req.Rubric)
		if err != nil {
			httpx.WriteError(w, r, payloadErrorResponse(err))
			return
		}
		params.Payload = withRubric
	}
	if settingsChanged {
		started, err := qtx.CountStartedLiteAssignmentRecipients(ctx, locked.ID)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		if started > 0 {
			httpx.WriteError(w, r, errAssignmentStarted("已有学生开始这份作业，类型和设置不能再修改"))
			return
		}
	}
	for _, uid := range removeIDs {
		key := sqlc.GetLiteAssignmentRecipientParams{AssignmentID: locked.ID, UserID: uid}
		if _, err := qtx.GetLiteAssignmentRecipient(ctx, key); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				continue // not a recipient: nothing to remove
			}
			httpx.WriteError(w, r, err)
			return
		}
		n, err := qtx.RemoveLiteAssignmentRecipient(ctx, sqlc.RemoveLiteAssignmentRecipientParams{AssignmentID: locked.ID, UserID: uid})
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		if n == 0 {
			httpx.WriteError(w, r, errAssignmentStarted("这名学生已开始这份作业，不能移除"))
			return
		}
	}
	for _, uid := range addIDs {
		if err := qtx.AddLiteAssignmentRecipient(ctx, sqlc.AddLiteAssignmentRecipientParams{AssignmentID: locked.ID, UserID: uid}); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}
	// A new or changed pick is checked against the recipient list this request
	// leaves behind, so a teacher can add a student and her pick in one save.
	// A pick unchanged from the stored payload is not re-checked: a removed
	// recipient's pick stays stored, and does not block a later save.
	if req.Payload != nil {
		current, err := qtx.ListLiteAssignmentRecipients(ctx, []uuid.UUID{locked.ID})
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		userIDs := make([]uuid.UUID, 0, len(current))
		for _, rc := range current {
			userIDs = append(userIDs, rc.UserID)
		}
		if liteassign.PicksOutsideChanged(params.Payload, locked.Payload, userIDs) {
			httpx.WriteError(w, r, errPickNotRecipient())
			return
		}
	}
	updated, err := qtx.UpdateLiteAssignment(ctx, params)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"assignment": newAssignmentDTO(updated)})
}

// archiveLiteAssignment handles DELETE /api/v1/lite/teacher/assignments/{aid}.
// The row is archived, not deleted: recipients who started keep their work.
func (a *API) archiveLiteAssignment(w http.ResponseWriter, r *http.Request) {
	as, ok := a.loadTeacherAssignment(w, r)
	if !ok {
		return
	}
	if err := a.d.Queries.ArchiveLiteAssignment(r.Context(), as.ID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
