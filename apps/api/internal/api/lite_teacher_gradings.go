package api

// lite_teacher_gradings.go — the teacher side of 一键AI批改: list a homework's
// gradings, queue them, regrade one, read one, edit and send (Task 6).
// No route here calls a model; the worker in lite_grading_jobs.go does.

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/liteassign"
	"mindimprint/api/internal/store/sqlc"
)

func errGradingQueueUnavailable() *httpx.APIError {
	return &httpx.APIError{Status: http.StatusServiceUnavailable, Code: "grading_queue_unavailable", Message: "批改队列未启动"}
}

func errGradingSent() *httpx.APIError {
	return &httpx.APIError{Status: http.StatusConflict, Code: "grading_sent", Message: "这一版的批改已发送，不能重新批改"}
}

func errGradingInProgress() *httpx.APIError {
	return &httpx.APIError{Status: http.StatusConflict, Code: "grading_in_progress", Message: "批改中"}
}

func errGradingExists() *httpx.APIError {
	return &httpx.APIError{Status: http.StatusConflict, Code: "grading_exists", Message: "这一版已有批改草稿，请使用「重新批改」"}
}

func errNotWritingForGrading() *httpx.APIError {
	return httpx.ErrBadRequest("not_writing_assignment", "只有写作作业可以批改", nil)
}

type gradingSummaryDTO struct {
	ID           string  `json:"id"`
	Status       string  `json:"status"`
	OverallGrade *string `json:"overallGrade"`
	Error        *string `json:"error"`
	ReviewedAt   *string `json:"reviewedAt"`
	SentAt       *string `json:"sentAt"`
}

type gradingVersionDTO struct {
	Number      int32  `json:"number"`
	SubmittedAt string `json:"submittedAt"`
}

type gradingRowDTO struct {
	UserID      string             `json:"userId"`
	DisplayName string             `json:"displayName"`
	AtomID      *string            `json:"atomId"`
	Version     *gradingVersionDTO `json:"version"`
	Grading     *gradingSummaryDTO `json:"grading"`
}

type teacherGradingDTO struct {
	ID                  string          `json:"id"`
	ClassID             string          `json:"classId"`
	AssignmentID        *string         `json:"assignmentId"`
	UserID              string          `json:"userId"`
	DisplayName         string          `json:"displayName"`
	AtomID              string          `json:"atomId"`
	VersionNumber       int32           `json:"versionNumber"`
	LatestVersionNumber int32           `json:"latestVersionNumber"`
	Title               string          `json:"title"`
	Body                string          `json:"body"`
	Lang                string          `json:"lang"`
	Rubric              json.RawMessage `json:"rubric"`
	Status              string          `json:"status"`
	Content             json.RawMessage `json:"content"`
	Error               *string         `json:"error"`
	ReviewedAt          *string         `json:"reviewedAt"`
	SentAt              *string         `json:"sentAt"`
	StudentSeenAt       *string         `json:"studentSeenAt"`
	UpdatedAt           string          `json:"updatedAt"`
}

func gradingOverallGrade(content []byte) *string {
	if len(content) == 0 {
		return nil
	}
	var c struct {
		Overall struct {
			Grade string `json:"grade"`
		} `json:"overall"`
	}
	if json.Unmarshal(content, &c) != nil || c.Overall.Grade == "" {
		return nil
	}
	return &c.Overall.Grade
}

func gradingSummaryOf(g sqlc.LiteGrading) gradingSummaryDTO {
	return gradingSummaryDTO{
		ID: g.ID.String(), Status: g.Status, OverallGrade: gradingOverallGrade(g.Content), Error: g.Error,
		ReviewedAt: tsStringPtr(g.ReviewedAt), SentAt: tsStringPtr(g.SentAt),
	}
}

// markStaleGradings turns abandoned running rows into failed ones before a
// read. It reports whether any row it was given was running, so the caller
// knows to read again.
func (a *API) markStaleGradings(ctx context.Context, rows []sqlc.LiteGrading) (bool, error) {
	var running []uuid.UUID
	for _, g := range rows {
		if g.Status == "running" {
			running = append(running, g.ID)
		}
	}
	if len(running) == 0 {
		return false, nil
	}
	return true, a.d.Queries.MarkStaleLiteGradingsFailed(ctx, running)
}

// loadTeacherGrading loads {gid}: the caller teaches the grading's class and
// the student is still a student in it. Anything else is 404.
func (a *API) loadTeacherGrading(w http.ResponseWriter, r *http.Request) (sqlc.LiteGrading, bool) {
	ctx := r.Context()
	gid, err := uuid.Parse(r.PathValue("gid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return sqlc.LiteGrading{}, false
	}
	g, err := a.d.Queries.GetLiteGrading(ctx, gid)
	if err != nil {
		writeNotFoundOr(w, r, err)
		return sqlc.LiteGrading{}, false
	}
	if _, err := a.assertTeacherOwnsClass(ctx, g.ClassID); err != nil {
		httpx.WriteError(w, r, err)
		return sqlc.LiteGrading{}, false
	}
	enrolled, err := a.d.Queries.IsEnrolledStudent(ctx, sqlc.IsEnrolledStudentParams{ClassID: g.ClassID, UserID: g.UserID})
	if err != nil {
		httpx.WriteError(w, r, err)
		return sqlc.LiteGrading{}, false
	}
	if !enrolled {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return sqlc.LiteGrading{}, false
	}
	return g, true
}

// gradingRubricFor is the rubric snapshot for a writing: its homework's
// rubric when it is homework, else the default for the writing's language.
func (a *API) gradingRubricFor(ctx context.Context, atomID, ownerID uuid.UUID) ([]byte, pgtype.UUID, error) {
	row, err := a.d.Queries.GetLiteAssignmentForAtom(ctx, sqlc.GetLiteAssignmentForAtomParams{
		AtomID: pgtype.UUID{Bytes: atomID, Valid: true}, UserID: ownerID,
	})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, pgtype.UUID{}, err
	}
	if err == nil && row.Kind == "writing" {
		as, err := a.d.Queries.GetLiteAssignment(ctx, row.ID)
		if err != nil {
			return nil, pgtype.UUID{}, err
		}
		b, err := json.Marshal(liteassign.EffectiveRubric(as.Payload))
		return b, pgtype.UUID{Bytes: as.ID, Valid: true}, err
	}
	wr, err := a.d.Queries.GetWriting(ctx, atomID)
	if err != nil {
		return nil, pgtype.UUID{}, err
	}
	b, err := json.Marshal(liteassign.DefaultRubric(wr.Lang))
	return b, pgtype.UUID{}, err
}

// teacherGradingResponse writes {"grading": …} for one row, with the version
// text it grades and the student's latest version number.
func (a *API) teacherGradingResponse(w http.ResponseWriter, r *http.Request, g sqlc.LiteGrading) {
	ctx := r.Context()
	src, err := a.d.Queries.GetLiteGradingSource(ctx, g.VersionID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	latest, err := a.d.Queries.GetLatestWritingVersion(ctx, g.AtomID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	student, err := a.d.Queries.GetUserByID(ctx, g.UserID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"grading": teacherGradingDTO{
		ID: g.ID.String(), ClassID: g.ClassID.String(), AssignmentID: uuidStringPtr(g.AssignmentID),
		UserID: g.UserID.String(), DisplayName: student.DisplayName, AtomID: g.AtomID.String(),
		VersionNumber: src.Number, LatestVersionNumber: latest.Number,
		Title: src.Title, Body: src.Body, Lang: src.Lang,
		Rubric: json.RawMessage(g.Rubric), Status: g.Status, Content: json.RawMessage(g.Content), Error: g.Error,
		ReviewedAt: tsStringPtr(g.ReviewedAt), SentAt: tsStringPtr(g.SentAt), StudentSeenAt: tsStringPtr(g.StudentSeenAt),
		UpdatedAt: g.UpdatedAt.Format(time.RFC3339),
	}})
}

// assignmentGradingState reads a writing homework's recipients, each one's
// latest version, and the grading row of that version (stale rows marked first).
//
// Recipients are filtered to students still enrolled in the class: a
// recipient row survives a student leaving (lite_assignment_recipient is not
// cleaned up on unenrollment), but she is no longer this teacher's to grade
// or list, so both the assignment's grading list and 一键AI批改 must skip her.
func (a *API) assignmentGradingState(ctx context.Context, as sqlc.LiteAssignment) ([]sqlc.ListLiteAssignmentRecipientsRow, map[uuid.UUID]sqlc.ListLatestWritingVersionsForAtomsRow, map[uuid.UUID]sqlc.LiteGrading, error) {
	all, err := a.d.Queries.ListLiteAssignmentRecipients(ctx, []uuid.UUID{as.ID})
	if err != nil {
		return nil, nil, nil, err
	}
	recipients := make([]sqlc.ListLiteAssignmentRecipientsRow, 0, len(all))
	for _, rc := range all {
		enrolled, err := a.d.Queries.IsEnrolledStudent(ctx, sqlc.IsEnrolledStudentParams{ClassID: as.ClassID, UserID: rc.UserID})
		if err != nil {
			return nil, nil, nil, err
		}
		if enrolled {
			recipients = append(recipients, rc)
		}
	}
	atomIDs := make([]uuid.UUID, 0, len(recipients))
	for _, rc := range recipients {
		if rc.AtomID.Valid {
			atomIDs = append(atomIDs, uuid.UUID(rc.AtomID.Bytes))
		}
	}
	versions := map[uuid.UUID]sqlc.ListLatestWritingVersionsForAtomsRow{}
	gradings := map[uuid.UUID]sqlc.LiteGrading{}
	if len(atomIDs) == 0 {
		return recipients, versions, gradings, nil
	}
	vrows, err := a.d.Queries.ListLatestWritingVersionsForAtoms(ctx, atomIDs)
	if err != nil {
		return nil, nil, nil, err
	}
	versionIDs := make([]uuid.UUID, 0, len(vrows))
	for _, v := range vrows {
		versions[v.AtomID] = v
		versionIDs = append(versionIDs, v.ID)
	}
	if len(versionIDs) == 0 {
		return recipients, versions, gradings, nil
	}
	grows, err := a.d.Queries.ListLiteGradingsForVersions(ctx, versionIDs)
	if err != nil {
		return nil, nil, nil, err
	}
	if again, err := a.markStaleGradings(ctx, grows); err != nil {
		return nil, nil, nil, err
	} else if again {
		if grows, err = a.d.Queries.ListLiteGradingsForVersions(ctx, versionIDs); err != nil {
			return nil, nil, nil, err
		}
	}
	for _, g := range grows {
		gradings[g.VersionID] = g
	}
	return recipients, versions, gradings, nil
}

// listLiteAssignmentGradings handles GET /api/v1/lite/teacher/assignments/{aid}/gradings.
func (a *API) listLiteAssignmentGradings(w http.ResponseWriter, r *http.Request) {
	as, ok := a.loadTeacherAssignment(w, r)
	if !ok {
		return
	}
	if as.Kind != "writing" {
		httpx.WriteError(w, r, errNotWritingForGrading())
		return
	}
	recipients, versions, gradings, err := a.assignmentGradingState(r.Context(), as)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	rows := make([]gradingRowDTO, 0, len(recipients))
	for _, rc := range recipients {
		row := gradingRowDTO{UserID: rc.UserID.String(), DisplayName: rc.DisplayName, AtomID: uuidStringPtr(rc.AtomID)}
		if rc.AtomID.Valid {
			if v, ok := versions[uuid.UUID(rc.AtomID.Bytes)]; ok {
				row.Version = &gradingVersionDTO{Number: v.Number, SubmittedAt: v.SubmittedAt.Format(time.RFC3339)}
				if g, ok := gradings[v.ID]; ok {
					s := gradingSummaryOf(g)
					row.Grading = &s
				}
			}
		}
		rows = append(rows, row)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"rows": rows})
}

// queueLiteAssignmentGradings handles POST /api/v1/lite/teacher/assignments/{aid}/gradings
// (一键AI批改). It queues each recipient whose latest version has no grading
// row; with retryFailed it also requeues that version's failed row. Drafts and
// sent rows are never touched here.
func (a *API) queueLiteAssignmentGradings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	as, ok := a.loadTeacherAssignment(w, r)
	if !ok {
		return
	}
	if as.Kind != "writing" {
		httpx.WriteError(w, r, errNotWritingForGrading())
		return
	}
	var req struct {
		RetryFailed bool `json:"retryFailed"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		httpx.WriteError(w, r, httpx.ErrBadJSON(err))
		return
	}
	if a.d.River == nil {
		httpx.WriteError(w, r, errGradingQueueUnavailable())
		return
	}
	u, _ := UserFromContext(ctx)
	rubric, err := json.Marshal(liteassign.EffectiveRubric(as.Payload))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	recipients, versions, gradings, err := a.assignmentGradingState(ctx, as)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	queued := 0
	for _, rc := range recipients {
		if !rc.AtomID.Valid {
			continue
		}
		v, ok := versions[uuid.UUID(rc.AtomID.Bytes)]
		if !ok {
			continue
		}
		var id uuid.UUID
		if existing, has := gradings[v.ID]; has {
			if !req.RetryFailed || existing.Status != "failed" {
				continue
			}
			g, err := a.d.Queries.RequeueLiteGrading(ctx, sqlc.RequeueLiteGradingParams{ID: existing.ID, Rubric: rubric, RequestedBy: u.ID})
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			if err != nil {
				httpx.WriteError(w, r, err)
				return
			}
			id = g.ID
		} else {
			g, err := a.d.Queries.CreateLiteGrading(ctx, sqlc.CreateLiteGradingParams{
				AtomID: v.AtomID, VersionID: v.ID, UserID: rc.UserID, ClassID: as.ClassID,
				AssignmentID: pgtype.UUID{Bytes: as.ID, Valid: true}, Rubric: rubric, RequestedBy: u.ID,
			})
			if errors.Is(err, pgx.ErrNoRows) {
				continue // a concurrent request created it
			}
			if err != nil {
				httpx.WriteError(w, r, err)
				return
			}
			id = g.ID
		}
		if a.enqueueLiteGrading(ctx, id) {
			queued++
		}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"queued": queued})
}

// requeueOrRefuse applies the regrade rules to an existing row: sent is never
// regraded, queued/running is already in progress, draft/failed goes back to the queue.
func (a *API) requeueOrRefuse(ctx context.Context, g sqlc.LiteGrading, requestedBy uuid.UUID) (sqlc.LiteGrading, error) {
	switch g.Status {
	case "sent":
		return sqlc.LiteGrading{}, errGradingSent()
	case "queued", "running":
		return sqlc.LiteGrading{}, errGradingInProgress()
	}
	rubric, _, err := a.gradingRubricFor(ctx, g.AtomID, g.UserID)
	if err != nil {
		return sqlc.LiteGrading{}, err
	}
	rq, err := a.d.Queries.RequeueLiteGrading(ctx, sqlc.RequeueLiteGradingParams{ID: g.ID, Rubric: rubric, RequestedBy: requestedBy})
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.LiteGrading{}, errGradingInProgress()
	}
	return rq, err
}

// queueOrRefuseSingle applies the single-writing POST's rules to an existing
// row for the version, which are stricter than regrade's: a draft is a
// 409 grading_exists (the teacher must go through the explicit regrade route,
// which the UI puts behind the 「重新批改会覆盖当前修改」 confirm, to overwrite it),
// sent is 409 grading_sent, queued/running is 409 grading_in_progress, and
// only a failed row (never graded — no content to lose) is quietly requeued.
func (a *API) queueOrRefuseSingle(ctx context.Context, g sqlc.LiteGrading, requestedBy uuid.UUID) (sqlc.LiteGrading, error) {
	switch g.Status {
	case "sent":
		return sqlc.LiteGrading{}, errGradingSent()
	case "draft":
		return sqlc.LiteGrading{}, errGradingExists()
	case "queued", "running":
		return sqlc.LiteGrading{}, errGradingInProgress()
	}
	rubric, _, err := a.gradingRubricFor(ctx, g.AtomID, g.UserID)
	if err != nil {
		return sqlc.LiteGrading{}, err
	}
	rq, err := a.d.Queries.RequeueLiteGrading(ctx, sqlc.RequeueLiteGradingParams{ID: g.ID, Rubric: rubric, RequestedBy: requestedBy})
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.LiteGrading{}, errGradingInProgress()
	}
	return rq, err
}

// queueLiteWritingGrading handles
// POST /api/v1/lite/teacher/classes/{id}/students/{userId}/items/{atomId}/gradings:
// grade one writing's latest version — create the row, or quietly requeue a
// failed one that never produced content. A draft or sent row is 409: only
// the explicit regrade route may overwrite an existing draft.
func (a *API) queueLiteWritingGrading(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	classID, userID, ok := a.authTeacherStudent(w, r)
	if !ok {
		return
	}
	at, ok := a.loadTeacherOwnedAtom(w, r, userID)
	if !ok {
		return
	}
	if at.Kind != "writing" {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	if a.d.River == nil {
		httpx.WriteError(w, r, errGradingQueueUnavailable())
		return
	}
	u, _ := UserFromContext(ctx)
	latest, err := a.d.Queries.GetLatestWritingVersion(ctx, at.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, &httpx.APIError{Status: http.StatusConflict, Code: "no_submission", Message: "这名学生还没有提交"})
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	rubric, assignmentID, err := a.gradingRubricFor(ctx, at.ID, userID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	g, err := a.d.Queries.CreateLiteGrading(ctx, sqlc.CreateLiteGradingParams{
		AtomID: at.ID, VersionID: latest.ID, UserID: userID, ClassID: classID,
		AssignmentID: assignmentID, Rubric: rubric, RequestedBy: u.ID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		existing, gerr := a.d.Queries.GetLiteGradingByVersion(ctx, latest.ID)
		if gerr != nil {
			httpx.WriteError(w, r, gerr)
			return
		}
		g, err = a.queueOrRefuseSingle(ctx, existing, u.ID)
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	a.enqueueLiteGrading(ctx, g.ID)
	if g, err = a.d.Queries.GetLiteGrading(ctx, g.ID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	a.teacherGradingResponse(w, r, g)
}

// regradeLiteGrading handles POST /api/v1/lite/teacher/gradings/{gid}/regrade (重新批改).
func (a *API) regradeLiteGrading(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	g, ok := a.loadTeacherGrading(w, r)
	if !ok {
		return
	}
	if a.d.River == nil {
		httpx.WriteError(w, r, errGradingQueueUnavailable())
		return
	}
	u, _ := UserFromContext(ctx)
	rq, err := a.requeueOrRefuse(ctx, g, u.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	a.enqueueLiteGrading(ctx, rq.ID)
	if rq, err = a.d.Queries.GetLiteGrading(ctx, rq.ID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	a.teacherGradingResponse(w, r, rq)
}

// getLiteGrading handles GET /api/v1/lite/teacher/gradings/{gid}.
func (a *API) getLiteGrading(w http.ResponseWriter, r *http.Request) {
	g, ok := a.loadTeacherGrading(w, r)
	if !ok {
		return
	}
	if again, err := a.markStaleGradings(r.Context(), []sqlc.LiteGrading{g}); err != nil {
		httpx.WriteError(w, r, err)
		return
	} else if again {
		if g, err = a.d.Queries.GetLiteGrading(r.Context(), g.ID); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}
	a.teacherGradingResponse(w, r, g)
}
