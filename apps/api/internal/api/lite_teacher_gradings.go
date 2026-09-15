package api

// lite_teacher_gradings.go — the teacher side of 一键AI批改: list a homework's
// gradings, queue them, regrade one, read one, edit and send (Task 6).
// No route here calls a model; the worker in lite_grading_jobs.go does.

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
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

// errGradingEnqueueFailedCode marks an error as an enqueue-time failure from
// enqueueLiteGradingTx (the job insert failed inside the row's transaction,
// which was then rolled back — the row change never happened). Callers that
// can retry later (queue-all) check this code to skip quietly instead of
// aborting the whole request; callers that cannot (single-writing POST,
// regrade) surface it to the teacher as-is.
const errGradingEnqueueFailedCode = "grading_enqueue_failed"

func errGradingEnqueueFailed(err error) *httpx.APIError {
	return errGradingEnqueueFailedMsg("入队失败：" + err.Error())
}

// errGradingEnqueueFailedMsg builds the same error from an already-formatted
// message — queue-all's all-recipients-failed response reuses the first
// recipient's message verbatim rather than re-wrapping it.
func errGradingEnqueueFailedMsg(msg string) *httpx.APIError {
	return &httpx.APIError{Status: http.StatusServiceUnavailable, Code: errGradingEnqueueFailedCode, Message: msg}
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

// sweptGrading re-reads g if it is stuck `running` past the 15-minute
// timeout, so a stale row does not block a regrade/single-writing POST with
// a false 「批改中」 until someone happens to GET it first (the read routes
// already swept; the write routes below did not).
func (a *API) sweptGrading(ctx context.Context, g sqlc.LiteGrading) (sqlc.LiteGrading, error) {
	again, err := a.markStaleGradings(ctx, []sqlc.LiteGrading{g})
	if err != nil {
		return sqlc.LiteGrading{}, err
	}
	if !again {
		return g, nil
	}
	return a.d.Queries.GetLiteGrading(ctx, g.ID)
}

// enqueueLiteGradingTx runs fn — a row create/requeue bound to a transaction
// — and the river job insert in the SAME Postgres transaction, so a
// committed queued row always has its job: if the process dies between "row
// written" and "job inserted", there used to be a window where the row sat
// queued forever (MarkStaleLiteGradingsFailed ignores queued rows, and every
// route above treats queued as 「批改中」). Wrapping both in one transaction
// closes that window — either both commit, or neither does, and fn's row
// change is rolled back along with the missing job.
//
// The caller must have already checked a.d.River != nil.
func (a *API) enqueueLiteGradingTx(ctx context.Context, fn func(qtx *sqlc.Queries) (sqlc.LiteGrading, error)) (sqlc.LiteGrading, error) {
	tx, err := a.d.Pool.Begin(ctx)
	if err != nil {
		return sqlc.LiteGrading{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	g, err := fn(a.d.Queries.WithTx(tx))
	if err != nil {
		return sqlc.LiteGrading{}, err
	}
	if _, err := a.d.River.InsertTx(ctx, tx, LiteGradingArgs{GradingID: g.ID}, nil); err != nil {
		return sqlc.LiteGrading{}, errGradingEnqueueFailed(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return sqlc.LiteGrading{}, err
	}
	return g, nil
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
	queued, failed := 0, 0
	var firstFailure string
	for _, rc := range recipients {
		if !rc.AtomID.Valid {
			continue
		}
		v, ok := versions[uuid.UUID(rc.AtomID.Bytes)]
		if !ok {
			continue
		}
		var err error
		if existing, has := gradings[v.ID]; has {
			if !req.RetryFailed || existing.Status != "failed" {
				continue
			}
			_, err = a.enqueueLiteGradingTx(ctx, func(qtx *sqlc.Queries) (sqlc.LiteGrading, error) {
				return qtx.RequeueLiteGrading(ctx, sqlc.RequeueLiteGradingParams{ID: existing.ID, Rubric: rubric, RequestedBy: u.ID})
			})
		} else {
			_, err = a.enqueueLiteGradingTx(ctx, func(qtx *sqlc.Queries) (sqlc.LiteGrading, error) {
				return qtx.CreateLiteGrading(ctx, sqlc.CreateLiteGradingParams{
					AtomID: v.AtomID, VersionID: v.ID, UserID: rc.UserID, ClassID: as.ClassID,
					AssignmentID: pgtype.UUID{Bytes: as.ID, Valid: true}, Rubric: rubric, RequestedBy: u.ID,
				})
			})
		}
		if errors.Is(err, pgx.ErrNoRows) {
			continue // a concurrent request already changed this version's row
		}
		if apiErr, ok := err.(*httpx.APIError); ok && apiErr.Code == errGradingEnqueueFailedCode {
			// The row change rolled back with the failed job insert — nothing
			// was left behind to mark failed. Surface it in the response
			// (AI/backend errors must reach the teacher, never fail silently)
			// instead of only logging it: a systematic failure (queue down
			// after a deploy) must not read as "nothing to grade".
			slog.Warn("lite grading: queue-all enqueue failed", "err", apiErr.Message, "user_id", rc.UserID.String())
			failed++
			if firstFailure == "" {
				firstFailure = apiErr.Message
			}
			continue
		}
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		queued++
	}
	if failed > 0 && queued == 0 {
		httpx.WriteError(w, r, errGradingEnqueueFailedMsg(firstFailure))
		return
	}
	var errField any
	if failed > 0 {
		errField = firstFailure
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"queued": queued, "failed": failed, "error": errField})
}

// requeueOrRefuse applies the regrade rules to an existing row: sent is never
// regraded, queued/running is already in progress, draft/failed goes back to
// the queue. Sweeps a stale `running` row first, so a regrade that crashed
// 15+ minutes ago does not read as still-in-progress (Task 5 review ruling 4).
func (a *API) requeueOrRefuse(ctx context.Context, g sqlc.LiteGrading, requestedBy uuid.UUID) (sqlc.LiteGrading, error) {
	g, err := a.sweptGrading(ctx, g)
	if err != nil {
		return sqlc.LiteGrading{}, err
	}
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
	rq, err := a.enqueueLiteGradingTx(ctx, func(qtx *sqlc.Queries) (sqlc.LiteGrading, error) {
		return qtx.RequeueLiteGrading(ctx, sqlc.RequeueLiteGradingParams{ID: g.ID, Rubric: rubric, RequestedBy: requestedBy})
	})
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
// Sweeps a stale `running` row first, same reasoning as requeueOrRefuse.
func (a *API) queueOrRefuseSingle(ctx context.Context, g sqlc.LiteGrading, requestedBy uuid.UUID) (sqlc.LiteGrading, error) {
	g, err := a.sweptGrading(ctx, g)
	if err != nil {
		return sqlc.LiteGrading{}, err
	}
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
	rq, err := a.enqueueLiteGradingTx(ctx, func(qtx *sqlc.Queries) (sqlc.LiteGrading, error) {
		return qtx.RequeueLiteGrading(ctx, sqlc.RequeueLiteGradingParams{ID: g.ID, Rubric: rubric, RequestedBy: requestedBy})
	})
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
	g, err := a.enqueueLiteGradingTx(ctx, func(qtx *sqlc.Queries) (sqlc.LiteGrading, error) {
		return qtx.CreateLiteGrading(ctx, sqlc.CreateLiteGradingParams{
			AtomID: at.ID, VersionID: latest.ID, UserID: userID, ClassID: classID,
			AssignmentID: assignmentID, Rubric: rubric, RequestedBy: u.ID,
		})
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
