package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/disciplines"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/liteassign"
	"mindimprint/api/internal/liteparent"
	"mindimprint/api/internal/liteweek"
	"mindimprint/api/internal/store/sqlc"
)

// loadLiteParentFacts gathers the facts of one student's parent report over
// the Beijing dates start..end, both included (start and end are Beijing
// midnights, as liteparent.ParseRange returns them).
//
// Before reading 金句 it makes sure every reading and writing finished in the
// range has a stored report, running phase 1 only (ensureAtomReportWith with
// allowProse=false): no model call. A report created here still owes its
// prose, so its moments are absent from these facts (plan 4 Ruling 2). An
// ensure error is logged and that item's moments are skipped.
//
// Because it writes atom_report rows, its only production caller is the
// generate POST (plan 4 Ruling 10). Redraft reuses the frozen facts and no GET
// calls it.
//
// The facts never carry chat text, writing.assigned_prompt or an assigned
// project's idea.
func (a *API) loadLiteParentFacts(ctx context.Context, classID, userID, teacherID uuid.UUID, start, end time.Time) (liteparent.Facts, error) {
	rs := start.In(liteweek.Beijing)
	re := end.In(liteweek.Beijing).AddDate(0, 0, 1) // exclusive bound
	q := a.d.Queries

	cls, err := q.GetClassByID(ctx, classID)
	if err != nil {
		return liteparent.Facts{}, err
	}
	student, err := q.GetUserByID(ctx, userID)
	if err != nil {
		return liteparent.Facts{}, err
	}
	teacher, err := q.GetUserByID(ctx, teacherID)
	if err != nil {
		return liteparent.Facts{}, err
	}

	f := liteparent.Facts{
		StudentName: student.DisplayName,
		ClassName:   cls.Name,
		TeacherName: teacher.DisplayName,
		RangeStart:  rs.Format("2006-01-02"),
		RangeEnd:    end.In(liteweek.Beijing).Format("2006-01-02"),
		Days:        liteparent.RangeDays(rs, end.In(liteweek.Beijing)),
		Minutes:     -1,
		Readings:    []liteparent.Item{},
		Writings:    []liteparent.Item{},
		Projects:    []liteparent.Item{},
		Moments:     []liteparent.Moment{},
		Keywords:    []liteparent.Keyword{},
	}

	activity, err := q.ParentRangeActivity(ctx, sqlc.ParentRangeActivityParams{
		UserID: userID, StartDay: pgDate(rs), EndDay: pgDate(re), RangeStart: rs, RangeEnd: re,
	})
	if err != nil {
		return liteparent.Facts{}, err
	}
	f.ActiveDays = int(activity.ActiveDays)
	f.Turns = int(activity.Turns)
	if activity.HasBucketInRange {
		f.Minutes = int(activity.Seconds) / 60
	}

	finished, err := q.ParentRangeFinished(ctx, sqlc.ParentRangeFinishedParams{UserID: userID, RangeStart: rs, RangeEnd: re})
	if err != nil {
		return liteparent.Facts{}, err
	}
	noMoments := map[uuid.UUID]bool{}
	for _, r := range finished {
		it := liteparent.Item{Kind: r.Kind, Title: r.Title, FinishedAt: r.FinishedAt.In(liteweek.Beijing).Format("2006-01-02")}
		switch r.Kind {
		case "reading":
			f.Readings = append(f.Readings, it)
		case "writing":
			f.Writings = append(f.Writings, it)
		default:
			f.Projects = append(f.Projects, it)
			continue
		}
		if _, _, err := a.ensureAtomReportWith(ctx, userID, r.AtomID, r.Kind, false); err != nil {
			slog.Warn("lite parent report: ensure atom report failed, moments skipped", "err", err, "atom_id", r.AtomID, "kind", r.Kind)
			noMoments[r.AtomID] = true
		}
	}

	moments, err := q.ParentRangeMoments(ctx, sqlc.ParentRangeMomentsParams{UserID: userID, RangeStart: rs, RangeEnd: re})
	if err != nil {
		return liteparent.Facts{}, err
	}
	for _, r := range moments {
		if r.ProsePending || noMoments[r.AtomID] {
			continue
		}
		ms, err := weekMoments(r.Moments, r.AssignedPrompt)
		if err != nil {
			slog.Warn("lite parent report: unreadable report moments, skipped", "err", err, "atom_id", r.AtomID)
			continue
		}
		for _, quote := range ms {
			f.Moments = append(f.Moments, liteparent.Moment{Quote: quote, ItemTitle: r.Title})
		}
	}

	states, err := q.ParentRangeAssignmentStates(ctx, sqlc.ParentRangeAssignmentStatesParams{
		ClassID: classID, UserID: userID, RangeStart: rs, RangeEnd: re,
	})
	if err != nil {
		return liteparent.Facts{}, err
	}
	for _, r := range states {
		// Work finished after the range ended was still overdue at its end.
		fin := tsPtr(r.FinishedAt)
		if fin != nil && !fin.Before(re) {
			fin = nil
		}
		f.AssignmentsTotal++
		switch liteassign.Status(r.StartedAt.Valid, fin, r.DueAt, re) {
		case "done":
			f.AssignmentsOnTime++
		case "done_late":
			f.AssignmentsLate++
		case "overdue":
			f.AssignmentsMissed++
		}
	}

	keywords, err := q.ParentRangeKeywords(ctx, sqlc.ParentRangeKeywordsParams{UserID: userID, RangeStart: rs, RangeEnd: re})
	if err != nil {
		return liteparent.Facts{}, err
	}
	for _, r := range keywords {
		f.Keywords = append(f.Keywords, liteparent.Keyword{Text: r.TextZh, Field: r.Field, FieldLabel: disciplines.FieldLabels[r.Field]})
	}
	return f, nil
}

// loadLiteParentOtherNames returns the display names of the class's other
// current students, for the prose name check.
func (a *API) loadLiteParentOtherNames(ctx context.Context, classID, userID uuid.UUID, selfName string) ([]string, error) {
	members, err := a.d.Queries.ListLiteWeekClassStudents(ctx, classID)
	if err != nil {
		return nil, err
	}
	return liteParentOtherNames(members, userID, selfName), nil
}

// liteParentOtherNames is every member except her. A blank name is skipped,
// and so is a classmate whose name is part of hers (her exact name, or 王丽
// for 王丽华): naming her would otherwise fail every attempt. A name inside a
// verified 《title》 or 「quote」 needs no skip here; CheckProse removes those
// spans before the name check.
func liteParentOtherNames(members []sqlc.ListLiteWeekClassStudentsRow, userID uuid.UUID, selfName string) []string {
	out := make([]string, 0, len(members))
	for _, m := range members {
		if m.ID == userID || strings.TrimSpace(m.DisplayName) == "" || strings.Contains(selfName, m.DisplayName) {
			continue
		}
		out = append(out, m.DisplayName)
	}
	return out
}

// ---- Teacher endpoints ----

const liteParentReportPurpose = "lite_parent_report"

// liteParentBodySectionMax is the rune cap on each section of the body a
// teacher edits. The model's draft is capped lower (400, in the composer).
const liteParentBodySectionMax = 2000

func errParentInvalidRange() *httpx.APIError {
	return &httpx.APIError{Status: http.StatusBadRequest, Code: "invalid_range", Message: "请选择有效的日期范围"}
}

func errParentRangeBeforeStart() *httpx.APIError {
	return &httpx.APIError{Status: http.StatusBadRequest, Code: "range_before_start", Message: "该时间段早于学生加入班级的时间"}
}

func errParentAlreadyPublished() *httpx.APIError {
	return &httpx.APIError{Status: http.StatusConflict, Code: "already_published", Message: "报告已发布，不能重新生成草稿"}
}

func errParentStudentLeft() *httpx.APIError {
	return &httpx.APIError{Status: http.StatusConflict, Code: "student_left", Message: "该学生已不在本班"}
}

func errParentReportEmpty() *httpx.APIError {
	return &httpx.APIError{Status: http.StatusConflict, Code: "report_empty", Message: "请先生成或填写报告内容"}
}

func errParentInvalidBody() *httpx.APIError {
	return &httpx.APIError{Status: http.StatusBadRequest, Code: "invalid_body", Message: "请提供报告内容"}
}

func errParentInvalidSection(key string) *httpx.APIError {
	return &httpx.APIError{Status: http.StatusBadRequest, Code: "invalid_section", Message: "报告不包含该部分：" + key}
}

func errParentSectionTooLong(key string) *httpx.APIError {
	return &httpx.APIError{Status: http.StatusBadRequest, Code: "section_too_long", Message: "每部分不超过 2000 字：" + key}
}

// ParentReportDTO is one parent report as the teacher sees it. Draft and Body
// are null until a draft is stored. Sections are the section keys the frozen
// facts support, in display order.
type ParentReportDTO struct {
	ID            string            `json:"id"`
	StudentID     string            `json:"studentId"`
	ClassID       string            `json:"classId"`
	RangeStart    string            `json:"rangeStart"`
	RangeEnd      string            `json:"rangeEnd"`
	Status        string            `json:"status"`
	Facts         liteparent.Facts  `json:"facts"`
	Draft         map[string]string `json:"draft"`
	Body          map[string]string `json:"body"`
	Sections      []string          `json:"sections"`
	ShareToken    *string           `json:"shareToken"`
	PublishedAt   *string           `json:"publishedAt"`
	StudentSeenAt *string           `json:"studentSeenAt"`
	CreatedAt     string            `json:"createdAt"`
	UpdatedAt     string            `json:"updatedAt"`
}

// ParentReportSummaryDTO is one row of a report list.
type ParentReportSummaryDTO struct {
	ID          string  `json:"id"`
	StudentID   string  `json:"studentId"`
	StudentName string  `json:"studentName"`
	RangeStart  string  `json:"rangeStart"`
	RangeEnd    string  `json:"rangeEnd"`
	Status      string  `json:"status"`
	PublishedAt *string `json:"publishedAt"`
	Shared      bool    `json:"shared"`
	CreatedAt   string  `json:"createdAt"`
}

type parentReportResponse struct {
	Report ParentReportDTO `json:"report"`
}

type parentReportDraftResponse struct {
	Report     ParentReportDTO `json:"report"`
	DraftError *string         `json:"draftError"`
}

func liteParentDate(d pgtype.Date) string { return d.Time.Format("2006-01-02") }

// liteParentSections decodes a stored draft or body. SQL NULL is a nil map.
func liteParentSections(raw []byte) (map[string]string, error) {
	if raw == nil {
		return nil, nil
	}
	var m map[string]string
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// liteParentBodyBlank reports whether a stored body has no section with
// non-blank text: NULL, {}, or whitespace-only values.
func liteParentBodyBlank(raw []byte) (bool, error) {
	body, err := liteParentSections(raw)
	if err != nil {
		return false, err
	}
	for _, v := range body {
		if strings.TrimSpace(v) != "" {
			return false, nil
		}
	}
	return true, nil
}

func newParentReportDTO(row sqlc.LiteParentReport) (ParentReportDTO, error) {
	var f liteparent.Facts
	if err := json.Unmarshal(row.Facts, &f); err != nil {
		return ParentReportDTO{}, err
	}
	draft, err := liteParentSections(row.Draft)
	if err != nil {
		return ParentReportDTO{}, err
	}
	body, err := liteParentSections(row.Body)
	if err != nil {
		return ParentReportDTO{}, err
	}
	return ParentReportDTO{
		ID: row.ID.String(), StudentID: row.UserID.String(), ClassID: row.ClassID.String(),
		RangeStart: liteParentDate(row.RangeStart), RangeEnd: liteParentDate(row.RangeEnd),
		Status: row.Status, Facts: f, Draft: draft, Body: body,
		Sections:   liteparent.SectionsWithFacts(f),
		ShareToken: row.ShareToken, PublishedAt: tsStringPtr(row.PublishedAt), StudentSeenAt: tsStringPtr(row.StudentSeenAt),
		CreatedAt: row.CreatedAt.Format(time.RFC3339), UpdatedAt: row.UpdatedAt.Format(time.RFC3339),
	}, nil
}

func newParentReportSummaryDTO(row sqlc.ListLiteParentReportsByClassRow) ParentReportSummaryDTO {
	return ParentReportSummaryDTO{
		ID: row.ID.String(), StudentID: row.UserID.String(), StudentName: row.StudentName,
		RangeStart: liteParentDate(row.RangeStart), RangeEnd: liteParentDate(row.RangeEnd),
		Status: row.Status, PublishedAt: tsStringPtr(row.PublishedAt), Shared: row.ShareToken != nil,
		CreatedAt: row.CreatedAt.Format(time.RFC3339),
	}
}

// writeParentReport writes {"report"}, or {"report","draftError"} when
// withDraftError is set (generate and redraft).
func writeParentReport(w http.ResponseWriter, r *http.Request, status int, row sqlc.LiteParentReport, withDraftError bool, draftError *string) {
	dto, err := newParentReportDTO(row)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if withDraftError {
		httpx.WriteJSON(w, status, parentReportDraftResponse{Report: dto, DraftError: draftError})
		return
	}
	httpx.WriteJSON(w, status, parentReportResponse{Report: dto})
}

// decodeLiteParentRequest decodes a JSON body that may be empty. ok=false
// means the error is already written.
func decodeLiteParentRequest(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil && !errors.Is(err, io.EOF) {
		httpx.WriteError(w, r, httpx.ErrBadJSON(err))
		return false
	}
	return true
}

// loadTeacherParentReport parses {rid}, loads the report and checks the
// caller teaches its class. Any failure is 404. It does not require the
// student to still be in the class (plan 4 Ruling 5): a teacher must be able
// to read, edit and revoke after she leaves.
func (a *API) loadTeacherParentReport(w http.ResponseWriter, r *http.Request) (sqlc.LiteParentReport, bool) {
	rid, err := uuid.Parse(r.PathValue("rid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return sqlc.LiteParentReport{}, false
	}
	rep, err := a.d.Queries.GetLiteParentReport(r.Context(), rid)
	if err != nil {
		writeNotFoundOr(w, r, err)
		return sqlc.LiteParentReport{}, false
	}
	if _, err := a.assertTeacherOwnsClass(r.Context(), rep.ClassID); err != nil {
		httpx.WriteError(w, r, err)
		return sqlc.LiteParentReport{}, false
	}
	return rep, true
}

// withLockedParentReport locks the report row FOR UPDATE in one transaction,
// runs fn on the locked row and commits when fn succeeds. fn decides from
// the locked row, never from a read taken before the lock.
func (a *API) withLockedParentReport(ctx context.Context, id uuid.UUID, fn func(q *sqlc.Queries, locked sqlc.LiteParentReport) (sqlc.LiteParentReport, error)) (sqlc.LiteParentReport, error) {
	tx, err := a.d.Pool.Begin(ctx)
	if err != nil {
		return sqlc.LiteParentReport{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := a.d.Queries.WithTx(tx)
	locked, err := qtx.GetLiteParentReportForUpdate(ctx, id)
	if err != nil {
		return sqlc.LiteParentReport{}, err
	}
	out, err := fn(qtx, locked)
	if err != nil {
		return sqlc.LiteParentReport{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return sqlc.LiteParentReport{}, err
	}
	return out, nil
}

// requireParentStudentEnrolled refuses when she is no longer a student in the
// report's class (409 student_left).
func requireParentStudentEnrolled(ctx context.Context, q *sqlc.Queries, locked sqlc.LiteParentReport) error {
	enrolled, err := q.IsEnrolledStudent(ctx, sqlc.IsEnrolledStudentParams{ClassID: locked.ClassID, UserID: locked.UserID})
	if err != nil {
		return err
	}
	if !enrolled {
		return errParentStudentLeft()
	}
	return nil
}

// requireParentDraftable is the check made under each lock before a draft is
// written: the report is still a draft and she is still in the class.
func requireParentDraftable(ctx context.Context, q *sqlc.Queries, locked sqlc.LiteParentReport) error {
	if locked.Status != "draft" {
		return errParentAlreadyPublished()
	}
	return requireParentStudentEnrolled(ctx, q, locked)
}

// composeLiteParentDraft drafts the sections from facts and records every
// attempt under the teacher. No lock is held during the model call. A rejected
// draft returns draftError and writes nothing. An accepted draft is written
// under a fresh row lock that re-checks requireParentDraftable; a report
// published in the meantime is 409 already_published and the draft is
// discarded. replaceBody overwrites the teacher's body; otherwise the body
// takes the draft only while it is still NULL.
func (a *API) composeLiteParentDraft(ctx context.Context, requestID string, teacherID uuid.UUID, resolved gateway.Resolved, reportID uuid.UUID, facts liteparent.Facts, others []string, replaceBody bool) (sqlc.LiteParentReport, *string, error) {
	sections, attempts, cerr := agent.ComposeLiteParentReport(ctx, a.d.Provider, resolved, facts, others)
	for _, at := range attempts {
		a.recordLiteLLMCall(ctx, teacherID, uuid.Nil, liteParentReportPurpose, resolved, at.Usage)
	}
	if cerr != nil {
		slog.Warn("lite parent report: draft rejected", "err", cerr, "report_id", reportID, "request_id", requestID)
		msg := cerr.Error()
		return sqlc.LiteParentReport{}, &msg, nil
	}
	draft, err := json.Marshal(sections)
	if err != nil {
		return sqlc.LiteParentReport{}, nil, err
	}
	row, err := a.withLockedParentReport(ctx, reportID, func(q *sqlc.Queries, locked sqlc.LiteParentReport) (sqlc.LiteParentReport, error) {
		if err := requireParentDraftable(ctx, q, locked); err != nil {
			return sqlc.LiteParentReport{}, err
		}
		// A body with no non-blank section (NULL, {}, or only whitespace, as an
		// autosave of an empty textarea leaves it) takes the draft too; a body
		// with any text of the teacher's is kept unless replaceBody.
		fill := replaceBody
		if !fill {
			blank, err := liteParentBodyBlank(locked.Body)
			if err != nil {
				return sqlc.LiteParentReport{}, err
			}
			fill = blank
		}
		var out sqlc.LiteParentReport
		var werr error
		if fill {
			out, werr = q.ReplaceLiteParentReportBody(ctx, sqlc.ReplaceLiteParentReportBodyParams{Draft: draft, ID: locked.ID})
		} else {
			out, werr = q.SetLiteParentReportDraft(ctx, sqlc.SetLiteParentReportDraftParams{Draft: draft, ID: locked.ID})
		}
		// Both writes match only status = 'draft'.
		if errors.Is(werr, pgx.ErrNoRows) {
			return sqlc.LiteParentReport{}, errParentAlreadyPublished()
		}
		return out, werr
	})
	if err != nil {
		// The model was paid for and its draft is lost. The draft text is not logged.
		var apiErr *httpx.APIError
		if errors.As(err, &apiErr) {
			slog.Warn("lite parent report: draft discarded", "code", apiErr.Code, "report_id", reportID,
				"attempts", len(attempts), "request_id", requestID)
		} else {
			slog.Error("lite parent report: draft write failed after compose", "err", err, "report_id", reportID,
				"attempts", len(attempts), "request_id", requestID)
		}
		return sqlc.LiteParentReport{}, nil, err
	}
	return row, nil, nil
}

// createLiteParentReport handles
// POST /api/v1/lite/teacher/classes/{id}/students/{userId}/parent-reports.
//
// Order: auth (she must be a current student) → range → lower bound (her
// enrollment) → entitlement → facts → route → insert the row with frozen
// facts → model → draft. A rejected draft still returns 201 with the row
// (draft and body null) and draftError. A range with no activity still runs:
// the teacher asked for it.
func (a *API) createLiteParentReport(w http.ResponseWriter, r *http.Request) {
	classID, userID, ok := a.authTeacherStudent(w, r)
	if !ok {
		return
	}
	var req struct {
		RangeStart string `json:"rangeStart"`
		RangeEnd   string `json:"rangeEnd"`
	}
	if !decodeLiteParentRequest(w, r, &req) {
		return
	}
	now := time.Now()
	defStart, defEnd := liteparent.DefaultRange(now)
	startStr, endStr := strings.TrimSpace(req.RangeStart), strings.TrimSpace(req.RangeEnd)
	if startStr == "" {
		startStr = defStart
	}
	if endStr == "" {
		endStr = defEnd
	}
	start, end, err := liteparent.ParseRange(startStr, endStr, now)
	if err != nil {
		if errors.Is(err, liteparent.ErrBadRange) {
			httpx.WriteError(w, r, errParentInvalidRange())
			return
		}
		httpx.WriteError(w, r, err)
		return
	}
	ctx := r.Context()
	joined, err := a.liteEnrollmentStart(ctx, classID, userID)
	if err != nil {
		// Removed after authTeacherStudent: the same 404 it gives.
		writeNotFoundOr(w, r, err)
		return
	}
	// A range that starts before she joined but ends after it is allowed.
	if !liteEndsAfterStart(end.AddDate(0, 0, 1), joined) {
		httpx.WriteError(w, r, errParentRangeBeforeStart())
		return
	}
	u, ok := requireTeacherEntitled(w, r)
	if !ok {
		return
	}

	facts, err := a.loadLiteParentFacts(ctx, classID, userID, u.ID, start, end)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	others, err := a.loadLiteParentOtherNames(ctx, classID, userID, facts.StudentName)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	factsJSON, err := json.Marshal(facts)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	mctx, cancel := detachedModelCtx(r)
	defer cancel()
	resolved, err := a.routeE(mctx, gateway.ClassAssess)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("lite_parent_report route: "+err.Error()))
		return
	}
	created, err := a.d.Queries.CreateLiteParentReport(ctx, sqlc.CreateLiteParentReportParams{
		UserID: userID, ClassID: classID, CreatedBy: u.ID,
		RangeStart: pgDate(start), RangeEnd: pgDate(end), Facts: factsJSON,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	requestID := httpx.RequestIDFromContext(ctx)
	row, draftError, err := a.composeLiteParentDraft(mctx, requestID, u.ID, resolved, created.ID, facts, others, false)
	if err != nil {
		// The row exists: answer 201 with it (its draft was not written) so the
		// client has the id. composeLiteParentDraft has already logged the
		// cause; a non-API error is not echoed, only the request id.
		var apiErr *httpx.APIError
		msg := "保存草稿失败：请求编号 " + requestID
		if errors.As(err, &apiErr) {
			msg = apiErr.Message
		}
		draftError = &msg
	}
	if draftError != nil {
		row = created
	}
	writeParentReport(w, r, http.StatusCreated, row, true, draftError)
}

// listLiteStudentParentReports handles
// GET /api/v1/lite/teacher/classes/{id}/students/{userId}/parent-reports.
// Never loads facts, never calls a model.
func (a *API) listLiteStudentParentReports(w http.ResponseWriter, r *http.Request) {
	classID, userID, ok := a.authTeacherStudent(w, r)
	if !ok {
		return
	}
	rows, err := a.d.Queries.ListLiteParentReportsByStudent(r.Context(), sqlc.ListLiteParentReportsByStudentParams{UserID: userID, ClassID: classID})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]ParentReportSummaryDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, newParentReportSummaryDTO(sqlc.ListLiteParentReportsByClassRow(row)))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"reports": out})
}

// listLiteClassParentReports handles
// GET /api/v1/lite/teacher/classes/{id}/parent-reports, including reports of
// students who have since left. Never loads facts, never calls a model.
func (a *API) listLiteClassParentReports(w http.ResponseWriter, r *http.Request) {
	cls, ok := a.authTeacherClass(w, r)
	if !ok {
		return
	}
	rows, err := a.d.Queries.ListLiteParentReportsByClass(r.Context(), cls.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]ParentReportSummaryDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, newParentReportSummaryDTO(row))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"reports": out})
}

// getLiteParentReport handles GET /api/v1/lite/teacher/parent-reports/{rid}.
// Never loads facts, never calls a model.
func (a *API) getLiteParentReport(w http.ResponseWriter, r *http.Request) {
	rep, ok := a.loadTeacherParentReport(w, r)
	if !ok {
		return
	}
	writeParentReport(w, r, http.StatusOK, rep, false, nil)
}

// patchLiteParentReport handles PATCH /api/v1/lite/teacher/parent-reports/{rid}
// with {"body": {key: text}}. The given sections are merged into the stored
// body; a section left out keeps its text. Keys must be sections the frozen
// facts support. Edits are allowed after publishing and after she leaves.
func (a *API) patchLiteParentReport(w http.ResponseWriter, r *http.Request) {
	rep, ok := a.loadTeacherParentReport(w, r)
	if !ok {
		return
	}
	var req struct {
		Body map[string]string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadJSON(err))
		return
	}
	if req.Body == nil {
		httpx.WriteError(w, r, errParentInvalidBody())
		return
	}
	keys := make([]string, 0, len(req.Body))
	for k := range req.Body {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	ctx := r.Context()
	row, err := a.withLockedParentReport(ctx, rep.ID, func(q *sqlc.Queries, locked sqlc.LiteParentReport) (sqlc.LiteParentReport, error) {
		var facts liteparent.Facts
		if err := json.Unmarshal(locked.Facts, &facts); err != nil {
			return sqlc.LiteParentReport{}, err
		}
		allowed := map[string]bool{}
		for _, k := range liteparent.SectionsWithFacts(facts) {
			allowed[k] = true
		}
		for _, k := range keys {
			if !allowed[k] {
				return sqlc.LiteParentReport{}, errParentInvalidSection(k)
			}
			if utf8.RuneCountInString(req.Body[k]) > liteParentBodySectionMax {
				return sqlc.LiteParentReport{}, errParentSectionTooLong(k)
			}
		}
		body, err := liteParentSections(locked.Body)
		if err != nil {
			return sqlc.LiteParentReport{}, err
		}
		if body == nil {
			body = map[string]string{}
		}
		for _, k := range keys {
			body[k] = req.Body[k]
		}
		raw, err := json.Marshal(body)
		if err != nil {
			return sqlc.LiteParentReport{}, err
		}
		return q.UpdateLiteParentReportBody(ctx, sqlc.UpdateLiteParentReportBodyParams{Body: raw, ID: locked.ID})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	writeParentReport(w, r, http.StatusOK, row, false, nil)
}

// redraftLiteParentReport handles
// POST /api/v1/lite/teacher/parent-reports/{rid}/redraft with
// {"replaceBody": bool}.
//
// Lock → check draft and enrolled → commit → entitlement → model (no lock
// held) → lock again → check again → write. It drafts from the facts frozen
// on the row and never reloads them (plan 4 Ruling 10).
func (a *API) redraftLiteParentReport(w http.ResponseWriter, r *http.Request) {
	rep, ok := a.loadTeacherParentReport(w, r)
	if !ok {
		return
	}
	var req struct {
		ReplaceBody bool `json:"replaceBody"`
	}
	if !decodeLiteParentRequest(w, r, &req) {
		return
	}
	ctx := r.Context()
	locked, err := a.withLockedParentReport(ctx, rep.ID, func(q *sqlc.Queries, locked sqlc.LiteParentReport) (sqlc.LiteParentReport, error) {
		if err := requireParentDraftable(ctx, q, locked); err != nil {
			return sqlc.LiteParentReport{}, err
		}
		return locked, nil
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	u, ok := requireTeacherEntitled(w, r)
	if !ok {
		return
	}
	var facts liteparent.Facts
	if err := json.Unmarshal(locked.Facts, &facts); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	others, err := a.loadLiteParentOtherNames(ctx, locked.ClassID, locked.UserID, facts.StudentName)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	mctx, cancel := detachedModelCtx(r)
	defer cancel()
	resolved, err := a.routeE(mctx, gateway.ClassAssess)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("lite_parent_report route: "+err.Error()))
		return
	}
	row, draftError, err := a.composeLiteParentDraft(mctx, httpx.RequestIDFromContext(ctx), u.ID, resolved, locked.ID, facts, others, req.ReplaceBody)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if draftError != nil {
		// Nothing was written; answer with the row as it is now.
		if row, err = a.d.Queries.GetLiteParentReport(mctx, locked.ID); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}
	writeParentReport(w, r, http.StatusOK, row, true, draftError)
}

// publishLiteParentReport handles
// POST /api/v1/lite/teacher/parent-reports/{rid}/publish. Idempotent: an
// existing share token is kept; after a revoke a new one is minted. Refused
// when she has left the class or the body is empty.
func (a *API) publishLiteParentReport(w http.ResponseWriter, r *http.Request) {
	rep, ok := a.loadTeacherParentReport(w, r)
	if !ok {
		return
	}
	token, err := newShareToken()
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	ctx := r.Context()
	row, err := a.withLockedParentReport(ctx, rep.ID, func(q *sqlc.Queries, locked sqlc.LiteParentReport) (sqlc.LiteParentReport, error) {
		if err := requireParentStudentEnrolled(ctx, q, locked); err != nil {
			return sqlc.LiteParentReport{}, err
		}
		empty, err := liteParentBodyBlank(locked.Body)
		if err != nil {
			return sqlc.LiteParentReport{}, err
		}
		if empty {
			return sqlc.LiteParentReport{}, errParentReportEmpty()
		}
		return q.PublishLiteParentReport(ctx, sqlc.PublishLiteParentReportParams{ShareToken: token, ID: locked.ID})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	writeParentReport(w, r, http.StatusOK, row, false, nil)
}

// revokeLiteParentReportShare handles
// DELETE /api/v1/lite/teacher/parent-reports/{rid}/share. The token becomes
// NULL; the report stays published. Allowed after she leaves the class.
func (a *API) revokeLiteParentReportShare(w http.ResponseWriter, r *http.Request) {
	rep, ok := a.loadTeacherParentReport(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	row, err := a.withLockedParentReport(ctx, rep.ID, func(q *sqlc.Queries, locked sqlc.LiteParentReport) (sqlc.LiteParentReport, error) {
		return q.RevokeLiteParentReportShare(ctx, locked.ID)
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	writeParentReport(w, r, http.StatusOK, row, false, nil)
}
