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
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/disciplines"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/liteassign"
	"mindimprint/api/internal/liteparent"
	"mindimprint/api/internal/liteweek"
	"mindimprint/api/internal/liteworkspace"
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
// project's idea. A moment whose quote names a classmate is dropped here,
// before the facts are frozen (plan 4 Ruling 18 B).
//
// It also returns the classmate names it filtered with, which are the same
// otherNames the composer checks the prose against: one list, one source.
func (a *API) loadLiteParentFacts(ctx context.Context, classID, userID, teacherID uuid.UUID, start, end time.Time) (liteparent.Facts, []string, error) {
	rs := start.In(liteweek.Beijing)
	re := end.In(liteweek.Beijing).AddDate(0, 0, 1) // exclusive bound
	q := a.d.Queries

	cls, err := q.GetClassByID(ctx, classID)
	if err != nil {
		return liteparent.Facts{}, nil, err
	}
	student, err := q.GetUserByID(ctx, userID)
	if err != nil {
		return liteparent.Facts{}, nil, err
	}
	teacher, err := q.GetUserByID(ctx, teacherID)
	if err != nil {
		return liteparent.Facts{}, nil, err
	}
	others, err := a.loadLiteParentOtherNames(ctx, classID, userID, student.DisplayName)
	if err != nil {
		return liteparent.Facts{}, nil, err
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
		return liteparent.Facts{}, nil, err
	}
	f.ActiveDays = int(activity.ActiveDays)
	f.Turns = int(activity.Turns)
	if activity.HasBucketInRange {
		f.Minutes = int(activity.Seconds) / 60
	}

	finished, err := q.ParentRangeFinished(ctx, sqlc.ParentRangeFinishedParams{UserID: userID, RangeStart: rs, RangeEnd: re})
	if err != nil {
		return liteparent.Facts{}, nil, err
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
		return liteparent.Facts{}, nil, err
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
	f.Moments = liteparent.DropMomentsNaming(f.Moments, others)

	states, err := q.ParentRangeAssignmentStates(ctx, sqlc.ParentRangeAssignmentStatesParams{
		ClassID: classID, UserID: userID, RangeStart: rs, RangeEnd: re,
	})
	if err != nil {
		return liteparent.Facts{}, nil, err
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
		return liteparent.Facts{}, nil, err
	}
	for _, r := range keywords {
		f.Keywords = append(f.Keywords, liteparent.Keyword{Text: r.TextZh, Field: r.Field, FieldLabel: disciplines.FieldLabels[r.Field]})
	}
	return f, others, nil
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

func errParentStudentLeft() *httpx.APIError {
	return &httpx.APIError{Status: http.StatusConflict, Code: "student_left", Message: "该学生已不在本班"}
}

func errParentInvalidHidden() *httpx.APIError {
	return &httpx.APIError{Status: http.StatusBadRequest, Code: "invalid_hidden", Message: "要隐藏的内容不在这份报告里"}
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
// are null until a draft is stored. Facts are the full frozen facts, hidden
// items included, so the editor can list them with a toggle; Hidden says which
// are hidden. Sections are the section keys the visible facts support, in
// display order. TeacherName (in facts) and CreatedAt make the byline.
type ParentReportDTO struct {
	ID         string            `json:"id"`
	StudentID  string            `json:"studentId"`
	ClassID    string            `json:"classId"`
	RangeStart string            `json:"rangeStart"`
	RangeEnd   string            `json:"rangeEnd"`
	Facts      liteparent.Facts  `json:"facts"`
	Hidden     liteparent.Hidden `json:"hidden"`
	Draft      map[string]string `json:"draft"`
	Body       map[string]string `json:"body"`
	Sections   []string          `json:"sections"`
	// HiddenMentions maps a visible section to the hidden quotes and keywords
	// its body text still contains (liteparent.HiddenMentions). Never stored;
	// always an object, {} when there are none.
	HiddenMentions map[string][]string `json:"hiddenMentions"`
	CreatedAt      string              `json:"createdAt"`
	UpdatedAt      string              `json:"updatedAt"`
}

// ParentReportSummaryDTO is one row of a report list.
type ParentReportSummaryDTO struct {
	ID          string `json:"id"`
	StudentID   string `json:"studentId"`
	StudentName string `json:"studentName"`
	RangeStart  string `json:"rangeStart"`
	RangeEnd    string `json:"rangeEnd"`
	CreatedAt   string `json:"createdAt"`
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

// liteParentHidden decodes a stored hidden set ('{}' by default) with both
// lists non-nil and de-duplicated.
func liteParentHidden(raw []byte) (liteparent.Hidden, error) {
	var h liteparent.Hidden
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &h); err != nil {
			return liteparent.Hidden{}, err
		}
	}
	return h.Normalize(), nil
}

func newParentReportDTO(row sqlc.LiteParentReport) (ParentReportDTO, error) {
	var f liteparent.Facts
	if err := json.Unmarshal(row.Facts, &f); err != nil {
		return ParentReportDTO{}, err
	}
	hidden, err := liteParentHidden(row.Hidden)
	if err != nil {
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
	sections := liteparent.SectionsWithFacts(liteparent.VisibleFacts(f, hidden))
	return ParentReportDTO{
		ID: row.ID.String(), StudentID: row.UserID.String(), ClassID: row.ClassID.String(),
		RangeStart: liteParentDate(row.RangeStart), RangeEnd: liteParentDate(row.RangeEnd),
		Facts: f, Hidden: hidden, Draft: draft, Body: body,
		Sections:       sections,
		HiddenMentions: liteparent.HiddenMentions(body, sections, f, hidden),
		CreatedAt:      row.CreatedAt.Format(time.RFC3339), UpdatedAt: row.UpdatedAt.Format(time.RFC3339),
	}, nil
}

func newParentReportSummaryDTO(row sqlc.ListLiteParentReportsByClassRow) ParentReportSummaryDTO {
	return ParentReportSummaryDTO{
		ID: row.ID.String(), StudentID: row.UserID.String(), StudentName: row.StudentName,
		RangeStart: liteParentDate(row.RangeStart), RangeEnd: liteParentDate(row.RangeEnd),
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
// to read, edit and export after she leaves.
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

// composeLiteParentDraft drafts the sections from facts and records every
// attempt under the teacher. facts are the visible facts: a hidden 金句 or
// keyword is neither in the prompt nor accepted by the quote check. No lock is
// held during the model call. A rejected draft returns draftError and writes
// nothing. An accepted draft is written under a fresh row lock that re-checks
// requireParentStudentEnrolled; if she left in the meantime it is 409
// student_left and the draft is discarded. replaceBody overwrites the
// teacher's body; otherwise the body takes the draft only while it is blank.
func (a *API) composeLiteParentDraft(ctx context.Context, requestID string, teacherID uuid.UUID, resolved gateway.Resolved, reportID, studentID uuid.UUID, facts liteparent.Facts, others []string, replaceBody bool) (sqlc.LiteParentReport, *string, error) {
	// Her gender as it is now: the pronoun the draft may use for her.
	gender, err := a.d.Queries.GetUserGender(ctx, studentID)
	if err != nil {
		return sqlc.LiteParentReport{}, nil, err
	}
	pronoun := liteworkspace.Pronoun(liteworkspace.GenderOf(gender))
	sections, attempts, cerr := agent.ComposeLiteParentReport(ctx, a.d.Provider, resolved, facts, others, pronoun)
	for _, at := range attempts {
		a.recordLiteLLMCall(ctx, teacherID, uuid.Nil, liteParentReportPurpose, resolved, at.Usage)
	}
	if cerr != nil {
		// The cause can quote her words or name a classmate
		// ("mentions other student: …"); names are redacted in the log. The
		// teacher still gets the full cause.
		slog.Warn("lite parent report: draft rejected",
			"err", liteworkspace.RedactNames(cerr.Error(), append([]string{facts.StudentName}, others...)),
			"report_id", reportID, "request_id", requestID)
		msg := cerr.Error()
		return sqlc.LiteParentReport{}, &msg, nil
	}
	draft, err := json.Marshal(sections)
	if err != nil {
		return sqlc.LiteParentReport{}, nil, err
	}
	row, err := a.withLockedParentReport(ctx, reportID, func(q *sqlc.Queries, locked sqlc.LiteParentReport) (sqlc.LiteParentReport, error) {
		if err := requireParentStudentEnrolled(ctx, q, locked); err != nil {
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
		if fill {
			return q.ReplaceLiteParentReportBody(ctx, sqlc.ReplaceLiteParentReportBodyParams{Draft: draft, ID: locked.ID})
		}
		return q.SetLiteParentReportDraft(ctx, sqlc.SetLiteParentReportDraftParams{Draft: draft, ID: locked.ID})
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

	facts, others, err := a.loadLiteParentFacts(ctx, classID, userID, u.ID, start, end)
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
	row, draftError, err := a.composeLiteParentDraft(mctx, requestID, u.ID, resolved, created.ID, userID, facts, others, false)
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
// with {"body": {key: text}?, "hidden": {"moments": [quote…], "keywords":
// [text…]}?}. At least one of the two is required.
//
//   - body: the given sections are merged into the stored body; a section left
//     out keeps its text. Keys must be sections the visible facts support
//     (after this request's hidden set is applied).
//   - hidden: replaces the stored set (send the whole object); duplicates are
//     dropped. Every entry must equal a moment quote or keyword text in the
//     frozen facts, or the request is 400 invalid_hidden. Left out, the stored
//     set is kept.
//
// Hiding every keyword removes interests from sections; its stored body text
// is kept. Edits are allowed after she leaves the class.
func (a *API) patchLiteParentReport(w http.ResponseWriter, r *http.Request) {
	rep, ok := a.loadTeacherParentReport(w, r)
	if !ok {
		return
	}
	var raw struct {
		Body   map[string]string `json:"body"`
		Hidden json.RawMessage   `json:"hidden"`
	}
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadJSON(err))
		return
	}
	// hidden is decoded strictly: a misspelt key ("moment") must be refused,
	// not read as an empty set that clears what the teacher hid.
	var reqHidden *liteparent.Hidden
	if len(raw.Hidden) > 0 && string(raw.Hidden) != "null" {
		dec := json.NewDecoder(strings.NewReader(string(raw.Hidden)))
		dec.DisallowUnknownFields()
		var h liteparent.Hidden
		if err := dec.Decode(&h); err != nil {
			httpx.WriteError(w, r, errParentInvalidHidden())
			return
		}
		reqHidden = &h
	}
	req := struct {
		Body   map[string]string
		Hidden *liteparent.Hidden
	}{raw.Body, reqHidden}
	if req.Body == nil && req.Hidden == nil {
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
		hidden, err := liteParentHidden(locked.Hidden)
		if err != nil {
			return sqlc.LiteParentReport{}, err
		}
		if req.Hidden != nil {
			hidden = req.Hidden.Normalize()
			if _, unknown := liteparent.UnknownHidden(facts, hidden); unknown {
				return sqlc.LiteParentReport{}, errParentInvalidHidden()
			}
		}
		allowed := map[string]bool{}
		for _, k := range liteparent.SectionsWithFacts(liteparent.VisibleFacts(facts, hidden)) {
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
		rawBody := locked.Body
		if req.Body != nil {
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
			if rawBody, err = json.Marshal(body); err != nil {
				return sqlc.LiteParentReport{}, err
			}
		}
		rawHidden, err := json.Marshal(hidden)
		if err != nil {
			return sqlc.LiteParentReport{}, err
		}
		return q.UpdateLiteParentReportEdit(ctx, sqlc.UpdateLiteParentReportEditParams{Body: rawBody, Hidden: rawHidden, ID: locked.ID})
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
// Lock → check enrolled → commit → entitlement → model (no lock held) → lock
// again → check again → write. It drafts from the facts frozen on the row and
// never reloads them (plan 4 Ruling 10), with the items hidden at the first
// lock left out (liteparent.VisibleFacts): a hidden 金句 or keyword is not in
// the prompt, and a draft quoting one fails the quote check.
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
		if err := requireParentStudentEnrolled(ctx, q, locked); err != nil {
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
	var frozen liteparent.Facts
	if err := json.Unmarshal(locked.Facts, &frozen); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	hidden, err := liteParentHidden(locked.Hidden)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	facts := liteparent.VisibleFacts(frozen, hidden)
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
	row, draftError, err := a.composeLiteParentDraft(mctx, httpx.RequestIDFromContext(ctx), u.ID, resolved, locked.ID, locked.UserID, facts, others, req.ReplaceBody)
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
