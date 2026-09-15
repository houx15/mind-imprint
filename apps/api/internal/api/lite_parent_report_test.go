package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/disciplines"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/liteparent"
	"mindimprint/api/internal/liteweek"
	"mindimprint/api/internal/store/sqlc"
)

// TestLiteParentFacts runs every fact query (ParentRangeActivity,
// ParentRangeFinished, ParentRangeMoments, ParentRangeAssignmentStates,
// ParentRangeKeywords, ListLiteWeekClassStudents) against seeded data inside
// a completed range, with a nil provider: facts loading never calls a model.
func TestLiteParentFacts(t *testing.T) {
	h, pool, teacher, classIDStr, studentID := liteTeacherFixture(t)
	ctx := context.Background()
	classID := uuid.MustParse(classIDStr)
	teacherID := userIDByEmail(t, pool, "lt-teacher@demo.local")
	student := signInAs(t, pool, studentID)
	mustExec(t, pool, `UPDATE users SET display_name = '林知遥' WHERE id = $1`, studentID)

	// start..end is 8 Beijing days, both included; re is the exclusive bound.
	today := liteweek.Day(time.Now())
	start := today.AddDate(0, 0, -10)
	end := today.AddDate(0, 0, -3)
	re := end.AddDate(0, 0, 1)

	// A finished reading with a stored report moment, two buckets in range, one
	// bucket on the day after the range, one student message in range and one
	// just before it.
	reading := seedLiteReadingForUser(t, pool, studentID, "finished", 0)
	mustExec(t, pool, `UPDATE reading SET title = '城市里的雨水花园', finished_at = $2 WHERE atom_id = $1`, reading, start.AddDate(0, 0, 1).Add(10*time.Hour))
	seedReport(t, pool, reading, "reading", `{"version":1,"moments":[{"quote":"雨水不是废水","where":""},{"quote":" ","where":""}]}`)
	seedBucket(t, pool, reading, start, 600)
	seedBucket(t, pool, reading, start.AddDate(0, 0, 1), 1200)
	seedBucket(t, pool, reading, re, 900)
	seedAtomMessageAt(t, pool, reading, start.AddDate(0, 0, 3).Add(9*time.Hour))
	seedAtomMessageAt(t, pool, reading, start.Add(-time.Second))

	// A finished writing with no report: the loader creates one (phase 1, so
	// its prose is pending and it contributes no moments).
	bare := seedLiteWritingForUser(t, pool, studentID, "active")
	mustExec(t, pool, `UPDATE writing SET status = 'finished', title = '雨水花园调查报告', finished_at = $2 WHERE atom_id = $1`, bare, start.AddDate(0, 0, 2))

	// Assignments due in range: one finished late (its report quotes the
	// teacher's prompt), one finished on time, one never started.
	late := createAssignment(t, h, teacher, classIDStr, writingAssignmentBody([]string{studentID.String()}))
	lateAtom := uuid.MustParse(startAssignment(t, h, student, late).AtomID)
	mustExec(t, pool, `UPDATE lite_assignment SET due_at = $2 WHERE id = $1`, late, start.AddDate(0, 0, 2))
	mustExec(t, pool, `UPDATE writing SET status = 'finished', finished_at = $2 WHERE atom_id = $1`, lateAtom, start.AddDate(0, 0, 4))
	// 我和王小明一起做实验 names an enrolled classmate: dropped before the
	// facts are frozen (Ruling 18 B); 雨把街道洗亮了 names no one and stays.
	seedReport(t, pool, lateAtom, "writing", `{"moments":[`+
		`{"quote":"写一篇关于雨的记叙文","where":""},`+
		`{"quote":"一篇关于雨的记叙文","where":""},`+
		`{"quote":"我和王小明一起做实验","where":""},`+
		`{"quote":"雨把街道洗亮了","where":""}]}`)

	onTime := createAssignment(t, h, teacher, classIDStr, writingAssignmentBody([]string{studentID.String()}))
	onTimeAtom := uuid.MustParse(startAssignment(t, h, student, onTime).AtomID)
	mustExec(t, pool, `UPDATE lite_assignment SET due_at = $2 WHERE id = $1`, onTime, start.AddDate(0, 0, 6))
	mustExec(t, pool, `UPDATE writing SET status = 'finished', finished_at = $2 WHERE atom_id = $1`, onTimeAtom, start.AddDate(0, 0, 5))

	missed := createAssignment(t, h, teacher, classIDStr, writingAssignmentBody([]string{studentID.String()}))
	mustExec(t, pool, `UPDATE lite_assignment SET due_at = $2 WHERE id = $1`, missed, start.AddDate(0, 0, 3))

	// Projects finished in range: hers, and an assigned one with no name whose
	// title must never be the teacher's idea.
	own := seedWeekProject(t, pool, studentID, "校园雨水调查", "keeping", start.AddDate(0, 0, -5))
	mustExec(t, pool, `UPDATE pbl_project SET finished_at = $2 WHERE atom_id = $1`, own, start.AddDate(0, 0, 3))
	q := sqlc.New(pool)
	assigned, err := q.CreateAtom(ctx, sqlc.CreateAtomParams{Kind: "project", UserID: studentID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := q.CreatePblProject(ctx, sqlc.CreatePblProjectParams{AtomID: assigned.ID, Idea: "老师的驱动问题", Kind: "investigation"}); err != nil {
		t.Fatal(err)
	}
	mustExec(t, pool, `UPDATE pbl_project SET assigned = true, name = '', finished_at = $2 WHERE atom_id = $1`, assigned.ID, start.AddDate(0, 0, 4))

	// A keyword first seen in range and one first seen just before it.
	seedWeekKeyword(t, pool, studentID, "海绵城市", start.AddDate(0, 0, 1).Add(8*time.Hour))
	seedWeekKeyword(t, pool, studentID, "旧词", start.Add(-time.Second))

	// Classmates: 王小明 (whose own finished reading must not leak in), 林知
	// (part of her name, so left out of otherNames), and a teacher enrolled as
	// a student (not a student member).
	wang := createStudent(t, pool, SeedSchoolID, "lp-wang@demo.local")
	enrollStudent(t, pool, wang, classIDStr)
	mustExec(t, pool, `UPDATE users SET display_name = '王小明' WHERE id = $1`, wang)
	wangReading := seedLiteReadingForUser(t, pool, wang, "finished", 0)
	mustExec(t, pool, `UPDATE reading SET title = '王小明的文章', finished_at = $2 WHERE atom_id = $1`, wangReading, start.AddDate(0, 0, 1))
	part := createStudent(t, pool, SeedSchoolID, "lp-part@demo.local")
	enrollStudent(t, pool, part, classIDStr)
	mustExec(t, pool, `UPDATE users SET display_name = '林知' WHERE id = $1`, part)
	enrollStudent(t, pool, createTeacher(t, pool, SeedSchoolID, "lp-teacher2@demo.local"), classIDStr)

	api := New(DepsForTest(pool))
	got, err := api.LoadLiteParentFactsForTest(ctx, classID, studentID, teacherID, start, end)
	if err != nil {
		t.Fatalf("LoadLiteParentFacts: %v", err)
	}

	day := func(t time.Time) string { return t.Format("2006-01-02") }
	if got.StudentName != "林知遥" || got.ClassName != "Lite Class" || got.TeacherName != "T lt-teacher@demo.local" {
		t.Fatalf("names = %q %q %q", got.StudentName, got.ClassName, got.TeacherName)
	}
	if got.RangeStart != day(start) || got.RangeEnd != day(end) || got.Days != 8 {
		t.Fatalf("range = %s..%s (%d days), want %s..%s (8)", got.RangeStart, got.RangeEnd, got.Days, day(start), day(end))
	}
	if got.ActiveDays != 3 || got.Minutes != 30 || got.Turns != 1 {
		t.Fatalf("activity = days %d minutes %d turns %d, want 3/30/1", got.ActiveDays, got.Minutes, got.Turns)
	}
	if want := []liteparent.Item{{Kind: "reading", Title: "城市里的雨水花园", FinishedAt: day(start.AddDate(0, 0, 1))}}; !reflect.DeepEqual(got.Readings, want) {
		t.Fatalf("readings = %+v, want %+v", got.Readings, want)
	}
	if len(got.Writings) != 3 || got.Writings[0] != (liteparent.Item{Kind: "writing", Title: "雨水花园调查报告", FinishedAt: day(start.AddDate(0, 0, 2))}) {
		t.Fatalf("writings = %+v", got.Writings)
	}
	projectTitles := []string{}
	for _, p := range got.Projects {
		projectTitles = append(projectTitles, p.Title)
	}
	if !reflect.DeepEqual(projectTitles, []string{"校园雨水调查", "项目"}) {
		t.Fatalf("projects = %+v", got.Projects)
	}
	if got.AssignmentsTotal != 3 || got.AssignmentsOnTime != 1 || got.AssignmentsLate != 1 || got.AssignmentsMissed != 1 {
		t.Fatalf("assignments = total %d on time %d late %d missed %d, want 3/1/1/1",
			got.AssignmentsTotal, got.AssignmentsOnTime, got.AssignmentsLate, got.AssignmentsMissed)
	}
	if want := []liteparent.Keyword{{Text: "海绵城市", Field: "society", FieldLabel: disciplines.FieldLabels["society"]}}; !reflect.DeepEqual(got.Keywords, want) || want[0].FieldLabel == "" {
		t.Fatalf("keywords = %+v, want %+v", got.Keywords, want)
	}
	quotes := []string{}
	for _, m := range got.Moments {
		quotes = append(quotes, m.Quote)
	}
	if !reflect.DeepEqual(quotes, []string{"雨水不是废水", "雨把街道洗亮了"}) || got.Moments[0].ItemTitle != "城市里的雨水花园" {
		t.Fatalf("moments = %+v", got.Moments)
	}

	// No teacher text anywhere in the snapshot, and nothing of 王小明's.
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	for _, banned := range []string{"写一篇关于雨的记叙文", "老师的驱动问题", "王小明"} {
		if strings.Contains(string(raw), banned) {
			t.Fatalf("facts carry %q: %s", banned, raw)
		}
	}

	// The bare writing now has a stored report, created without prose.
	var pending bool
	if err := pool.QueryRow(ctx, `SELECT COALESCE((report->>'prosePending')::boolean, false) FROM atom_report WHERE atom_id = $1`, bare).Scan(&pending); err != nil {
		t.Fatalf("atom_report for the bare writing: %v", err)
	}
	if !pending {
		t.Fatal("the report the loader created must still owe its prose")
	}
	if n := countAllLLMCalls(t, pool); n != 0 {
		t.Fatalf("llm_call rows = %d, want 0", n)
	}

	others, err := api.LoadLiteParentOtherNamesForTest(ctx, classID, studentID, got.StudentName)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(others, []string{"王小明"}) {
		t.Fatalf("otherNames = %v, want [王小明]", others)
	}
}

// TestLiteParentFactsQuietRange: a range with nothing in it has zero counts,
// Minutes -1 and empty (non-nil) lists.
func TestLiteParentFactsQuietRange(t *testing.T) {
	_, pool, _, classIDStr, studentID := liteTeacherFixture(t)
	today := liteweek.Day(time.Now())
	got, err := New(DepsForTest(pool)).LoadLiteParentFactsForTest(context.Background(), uuid.MustParse(classIDStr), studentID,
		userIDByEmail(t, pool, "lt-teacher@demo.local"), today.AddDate(0, 0, -5), today.AddDate(0, 0, -1))
	if err != nil {
		t.Fatal(err)
	}
	if got.Minutes != -1 || got.ActiveDays != 0 || got.Turns != 0 || got.Days != 5 || got.AssignmentsTotal != 0 {
		t.Fatalf("facts = %+v", got)
	}
	if got.Readings == nil || got.Writings == nil || got.Projects == nil || got.Moments == nil || got.Keywords == nil {
		t.Fatalf("lists must be empty, not null: %+v", got)
	}
}

// ---- Task 3: teacher endpoints ----

// Replies for a report whose facts hold one reading, 《城市里的雨水花园》, with
// the moment 「雨水不是废水」: sections overview, reading, next. None of them
// carries a digit.
const (
	parentValidReply  = `{"overview":"这段时间读完《城市里的雨水花园》，写下「雨水不是废水」。","reading":"读完《城市里的雨水花园》。","next":"请和她聊一聊雨水花园。"}`
	parentSecondReply = `{"overview":"这段时间读完《城市里的雨水花园》。","reading":"她在《城市里的雨水花园》里写下「雨水不是废水」。","next":"请和她一起读一篇新文章。"}`
	parentThirdReply  = `{"overview":"读完《城市里的雨水花园》。","reading":"写下「雨水不是废水」。","next":"请和她聊一聊城市里的雨水。"}`
	// parentPlainReply quotes nothing, so it passes whether or not the moment
	// is in the facts.
	parentPlainReply = `{"overview":"这段时间读完《城市里的雨水花园》。","reading":"读完《城市里的雨水花园》。","next":"请和她聊一聊雨水花园。"}`
	// parentQuietReply is for a range with no activity: overview and next only.
	parentQuietReply = `{"overview":"这段时间没有完成的学习记录。","next":"请和她聊一聊最近读的内容。"}`
)

type parentReportJSON struct {
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
	CreatedAt  string            `json:"createdAt"`
	UpdatedAt  string            `json:"updatedAt"`
}

type parentReportResp struct {
	Report     parentReportJSON `json:"report"`
	DraftError *string          `json:"draftError"`
}

type parentSummaryJSON struct {
	ID          string `json:"id"`
	StudentID   string `json:"studentId"`
	StudentName string `json:"studentName"`
	RangeStart  string `json:"rangeStart"`
	RangeEnd    string `json:"rangeEnd"`
	CreatedAt   string `json:"createdAt"`
}

// wantReportKeys is the teacher report DTO's exact key set: no publish or
// share keys remain.
var wantReportKeys = []string{"body", "classId", "createdAt", "draft", "facts", "hidden", "hiddenMentions", "id", "rangeEnd", "rangeStart", "sections", "studentId", "updatedAt"}

// wantSummaryKeys is a report list row's exact key set.
var wantSummaryKeys = []string{"createdAt", "id", "rangeEnd", "rangeStart", "studentId", "studentName"}

type parentListResp struct {
	Reports []parentSummaryJSON `json:"reports"`
}

func parentReportsPath(classID string, userID uuid.UUID) string {
	return "/api/v1/lite/teacher/classes/" + classID + "/students/" + userID.String() + "/parent-reports"
}

func classParentReportsPath(classID string) string {
	return "/api/v1/lite/teacher/classes/" + classID + "/parent-reports"
}

func parentReportPath(id string) string {
	return "/api/v1/lite/teacher/parent-reports/" + id
}

// parentDo sends one request and decodes a 200 or 201 body into out.
func parentDo(t *testing.T, h http.Handler, c *http.Cookie, method, path, body string, out any) (int, string) {
	t.Helper()
	rec := doJSON(t, h, c, method, path, body)
	if out != nil && (rec.Code == http.StatusOK || rec.Code == http.StatusCreated) {
		if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
			t.Fatalf("decode %s %s: %v body=%s", method, path, err, rec.Body)
		}
	}
	return rec.Code, rec.Body.String()
}

func wantParentError(t *testing.T, what string, code int, body string, wantStatus int, wantCode string) {
	t.Helper()
	if code != wantStatus || !strings.Contains(body, `"`+wantCode+`"`) {
		t.Fatalf("%s = %d %s, want %d %s", what, code, body, wantStatus, wantCode)
	}
}

func parentSectionsOf(t *testing.T, reply string) map[string]string {
	t.Helper()
	var m map[string]string
	if err := json.Unmarshal([]byte(reply), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func parentBodyJSON(t *testing.T, sections map[string]string) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{"body": sections})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func parentRangeJSON(start, end time.Time) string {
	return `{"rangeStart":"` + start.Format("2006-01-02") + `","rangeEnd":"` + end.Format("2006-01-02") + `"}`
}

// seedParentReading: a reading finished three days ago, inside the default
// range, titled 城市里的雨水花园. withMoment stores its report with the moment
// 「雨水不是废水」; without it the reading has no report yet.
func seedParentReading(t *testing.T, pool *pgxpool.Pool, studentID uuid.UUID, withMoment bool) uuid.UUID {
	t.Helper()
	reading := seedLiteReadingForUser(t, pool, studentID, "finished", 0)
	finished := liteweek.Day(time.Now()).AddDate(0, 0, -3).Add(10 * time.Hour)
	mustExec(t, pool, `UPDATE reading SET title = '城市里的雨水花园', finished_at = $2 WHERE atom_id = $1`, reading, finished)
	if withMoment {
		seedReport(t, pool, reading, "reading", `{"version":1,"moments":[{"quote":"雨水不是废水","where":""}]}`)
	}
	return reading
}

// parentFixture is the lite teacher fixture with her enrollment 90 days back,
// her name set to 林知遥, and a finished reading with a moment.
func parentFixture(t *testing.T, prov gateway.Provider) (h http.Handler, pool *pgxpool.Pool, teacher *http.Cookie, classID string, studentID uuid.UUID) {
	t.Helper()
	h, pool, teacher, classID, studentID = liteTeacherFixtureWithProvider(t, prov)
	backdateWeeklyStart(t, pool, classID)
	mustExec(t, pool, `UPDATE users SET display_name = '林知遥' WHERE id = $1`, studentID)
	seedParentReading(t, pool, studentID, true)
	return
}

func generateParentReport(t *testing.T, h http.Handler, teacher *http.Cookie, classID string, studentID uuid.UUID) parentReportResp {
	t.Helper()
	var got parentReportResp
	if code, body := parentDo(t, h, teacher, "POST", parentReportsPath(classID, studentID), "", &got); code != http.StatusCreated {
		t.Fatalf("generate = %d %s", code, body)
	}
	return got
}

// parentHookProvider runs before ahead of every model call.
type parentHookProvider struct {
	inner  *gateway.SequenceStubProvider
	before func()
}

func (p *parentHookProvider) Stream(ctx context.Context, r gateway.Resolved, req gateway.ChatRequest) (<-chan gateway.StreamEvent, error) {
	if p.before != nil {
		p.before()
	}
	return p.inner.Stream(ctx, r, req)
}

// TestLiteParentReportGenerate: the default range, frozen facts, body equal
// to the first draft, one llm_call under the teacher, and the report in both
// lists and on GET.
func TestLiteParentReportGenerate(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(weeklyReply(parentValidReply))
	h, pool, teacher, classID, studentID := parentFixture(t, prov)
	teacherID := userIDByEmail(t, pool, "lt-teacher@demo.local")

	code, body := parentDo(t, h, teacher, "POST", parentReportsPath(classID, studentID), "", nil)
	if code != http.StatusCreated {
		t.Fatalf("generate = %d %s", code, body)
	}
	if raw := rawWeeklyFields(t, body); string(raw["draftError"]) != "null" {
		t.Fatalf("draftError must be present and null: %s", body)
	}
	var got parentReportResp
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatal(err)
	}
	rep := got.Report
	wantStart, wantEnd := liteparent.DefaultRange(time.Now())
	if rep.RangeStart != wantStart || rep.RangeEnd != wantEnd ||
		rep.StudentID != studentID.String() || rep.ClassID != classID {
		t.Fatalf("report = %+v, want a report over %s..%s", rep, wantStart, wantEnd)
	}
	reportRaw := rawWeeklyFields(t, string(rawWeeklyFields(t, body)["report"]))
	if got := keysOfRaw(reportRaw); !reflect.DeepEqual(got, wantReportKeys) {
		t.Fatalf("report keys = %v, want %v", got, wantReportKeys)
	}
	// Generate starts with nothing hidden, written as two empty arrays.
	if string(reportRaw["hidden"]) != `{"moments":[],"keywords":[]}` {
		t.Fatalf("hidden = %s, want empty arrays", reportRaw["hidden"])
	}
	f := rep.Facts
	if f.StudentName != "林知遥" || len(f.Readings) != 1 || f.Readings[0].Title != "城市里的雨水花园" ||
		len(f.Moments) != 1 || f.Moments[0].Quote != "雨水不是废水" || f.RangeStart != wantStart {
		t.Fatalf("facts = %+v", f)
	}
	if !reflect.DeepEqual(rep.Sections, []string{"overview", "reading", "next"}) {
		t.Fatalf("sections = %v", rep.Sections)
	}
	want := parentSectionsOf(t, parentValidReply)
	if !reflect.DeepEqual(rep.Draft, want) || !reflect.DeepEqual(rep.Body, want) {
		t.Fatalf("draft = %v body = %v, want both %v", rep.Draft, rep.Body, want)
	}
	if prov.Calls != 1 {
		t.Fatalf("provider calls = %d, want 1", prov.Calls)
	}
	if ids := llmCallUsers(t, pool, "lite_parent_report"); len(ids) != 1 || ids[0] != teacherID {
		t.Fatalf("llm_call users = %v, want [teacher %s]", ids, teacherID)
	}

	var one parentReportResp
	if code, body := parentDo(t, h, teacher, "GET", parentReportPath(rep.ID), "", &one); code != http.StatusOK || !reflect.DeepEqual(one.Report, rep) {
		t.Fatalf("GET = %d %s, want the generated report", code, body)
	}
	for _, path := range []string{parentReportsPath(classID, studentID), classParentReportsPath(classID)} {
		var list parentListResp
		if code, body := parentDo(t, h, teacher, "GET", path, "", &list); code != http.StatusOK {
			t.Fatalf("GET %s = %d %s", path, code, body)
		}
		wantRow := parentSummaryJSON{ID: rep.ID, StudentID: studentID.String(), StudentName: "林知遥",
			RangeStart: wantStart, RangeEnd: wantEnd, CreatedAt: rep.CreatedAt}
		if len(list.Reports) != 1 || list.Reports[0] != wantRow {
			t.Fatalf("GET %s reports = %+v, want [%+v]", path, list.Reports, wantRow)
		}
		var rawList struct {
			Reports []map[string]any `json:"reports"`
		}
		parentDo(t, h, teacher, "GET", path, "", &rawList)
		if got := keysOf(rawList.Reports[0]); !reflect.DeepEqual(got, wantSummaryKeys) {
			t.Fatalf("GET %s row keys = %v, want %v", path, got, wantSummaryKeys)
		}
	}
}

// TestLiteParentReportGenerateRejected: a reply that fails twice still leaves
// the report row with its facts; draft and body are null, draftError says why,
// and both attempts are recorded.
func TestLiteParentReportGenerateRejected(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(weeklyReply("不是 JSON"))
	h, pool, teacher, classID, studentID := parentFixture(t, prov)

	code, body := parentDo(t, h, teacher, "POST", parentReportsPath(classID, studentID), "", nil)
	if code != http.StatusCreated {
		t.Fatalf("generate = %d %s", code, body)
	}
	report := rawWeeklyFields(t, string(rawWeeklyFields(t, body)["report"]))
	if string(report["draft"]) != "null" || string(report["body"]) != "null" {
		t.Fatalf("draft and body must be null: %s", body)
	}
	var got parentReportResp
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatal(err)
	}
	if got.DraftError == nil || *got.DraftError == "" || len(got.Report.Facts.Readings) != 1 {
		t.Fatalf("draftError = %v facts = %+v", got.DraftError, got.Report.Facts)
	}
	if prov.Calls != 2 {
		t.Fatalf("provider calls = %d, want 2", prov.Calls)
	}
	if n := len(llmCallUsers(t, pool, "lite_parent_report")); n != 2 {
		t.Fatalf("llm_call rows = %d, want 2", n)
	}
	if n := weeklyCount(t, pool, `SELECT count(*) FROM lite_parent_report WHERE user_id = $1`, studentID); n != 1 {
		t.Fatalf("report rows = %d, want 1", n)
	}
}

// TestLiteParentReportEditAndRedraft: PATCH validates keys against the
// frozen facts and the 2000-rune cap and merges into the body; a redraft
// without replaceBody keeps the teacher's text, and with it replaces it.
func TestLiteParentReportEditAndRedraft(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(weeklyReply(parentValidReply), weeklyReply(parentSecondReply), weeklyReply(parentThirdReply))
	h, pool, teacher, classID, studentID := parentFixture(t, prov)
	rep := generateParentReport(t, h, teacher, classID, studentID).Report
	path := parentReportPath(rep.ID)
	first := parentSectionsOf(t, parentValidReply)

	for _, tc := range []struct {
		what, body string
		status     int
		code       string
	}{
		{"unknown key", parentBodyJSON(t, map[string]string{"hobbies": "画画"}), http.StatusBadRequest, "invalid_section"},
		{"section without facts", parentBodyJSON(t, map[string]string{"projects": "项目"}), http.StatusBadRequest, "invalid_section"},
		{"2001 runes", parentBodyJSON(t, map[string]string{"overview": strings.Repeat("字", 2001)}), http.StatusBadRequest, "section_too_long"},
		{"no body", `{}`, http.StatusBadRequest, "invalid_body"},
		{"not json", `{`, http.StatusBadRequest, "bad_json"},
	} {
		code, body := parentDo(t, h, teacher, "PATCH", path, tc.body, nil)
		wantParentError(t, "PATCH "+tc.what, code, body, tc.status, tc.code)
	}
	var unchanged parentReportResp
	parentDo(t, h, teacher, "GET", path, "", &unchanged)
	if !reflect.DeepEqual(unchanged.Report.Body, first) {
		t.Fatalf("body after refused PATCHes = %v, want %v", unchanged.Report.Body, first)
	}

	var patched parentReportResp
	if code, body := parentDo(t, h, teacher, "PATCH", path, parentBodyJSON(t, map[string]string{"overview": strings.Repeat("字", 2000)}), &patched); code != http.StatusOK {
		t.Fatalf("PATCH 2000 runes = %d %s", code, body)
	}
	if patched.Report.Body["overview"] != strings.Repeat("字", 2000) || patched.Report.Body["reading"] != first["reading"] {
		t.Fatalf("PATCH must merge into the body: %v", patched.Report.Body)
	}
	const edited = "老师改过的概述"
	parentDo(t, h, teacher, "PATCH", path, parentBodyJSON(t, map[string]string{"overview": edited}), &patched)

	var kept parentReportResp
	if code, body := parentDo(t, h, teacher, "POST", path+"/redraft", `{"replaceBody":false}`, &kept); code != http.StatusOK || kept.DraftError != nil {
		t.Fatalf("redraft = %d %s", code, body)
	}
	second := parentSectionsOf(t, parentSecondReply)
	wantKept := map[string]string{"overview": edited, "reading": first["reading"], "next": first["next"]}
	if !reflect.DeepEqual(kept.Report.Draft, second) || !reflect.DeepEqual(kept.Report.Body, wantKept) {
		t.Fatalf("redraft keep: draft = %v body = %v, want %v and %v", kept.Report.Draft, kept.Report.Body, second, wantKept)
	}

	var replaced parentReportResp
	if code, body := parentDo(t, h, teacher, "POST", path+"/redraft", `{"replaceBody":true}`, &replaced); code != http.StatusOK || replaced.DraftError != nil {
		t.Fatalf("redraft replace = %d %s", code, body)
	}
	third := parentSectionsOf(t, parentThirdReply)
	if !reflect.DeepEqual(replaced.Report.Draft, third) || !reflect.DeepEqual(replaced.Report.Body, third) {
		t.Fatalf("redraft replace: draft = %v body = %v, want both %v", replaced.Report.Draft, replaced.Report.Body, third)
	}
	if prov.Calls != 3 || len(llmCallUsers(t, pool, "lite_parent_report")) != 3 {
		t.Fatalf("provider calls = %d, want 3 with 3 llm_call rows", prov.Calls)
	}
}

// TestLiteParentReportRedraftFillsBlankBody (fix round 1): a failed generate
// leaves body NULL; an autosave of an empty section turns it into
// {"overview":""}. A redraft without replaceBody must still fill that blank
// body with the draft.
func TestLiteParentReportRedraftFillsBlankBody(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(weeklyReply("不是 JSON"), weeklyReply("不是 JSON"), weeklyReply(parentValidReply))
	h, _, teacher, classID, studentID := parentFixture(t, prov)
	got := generateParentReport(t, h, teacher, classID, studentID)
	if got.DraftError == nil || got.Report.Body != nil {
		t.Fatalf("generate = %+v err = %v, want a failed draft", got.Report, got.DraftError)
	}
	path := parentReportPath(got.Report.ID)

	var patched parentReportResp
	if code, body := parentDo(t, h, teacher, "PATCH", path, parentBodyJSON(t, map[string]string{"overview": ""}), &patched); code != http.StatusOK {
		t.Fatalf("PATCH blank = %d %s", code, body)
	}
	if !reflect.DeepEqual(patched.Report.Body, map[string]string{"overview": ""}) {
		t.Fatalf("body after blank PATCH = %v", patched.Report.Body)
	}

	var redrafted parentReportResp
	if code, body := parentDo(t, h, teacher, "POST", path+"/redraft", `{"replaceBody":false}`, &redrafted); code != http.StatusOK || redrafted.DraftError != nil {
		t.Fatalf("redraft = %d %s", code, body)
	}
	want := parentSectionsOf(t, parentValidReply)
	if !reflect.DeepEqual(redrafted.Report.Draft, want) || !reflect.DeepEqual(redrafted.Report.Body, want) {
		t.Fatalf("redraft: draft = %v body = %v, want both %v", redrafted.Report.Draft, redrafted.Report.Body, want)
	}
}

// TestLiteParentReportGenerateStudentLeftMidCall (fix round 1): she is
// removed while generate's model call runs. The row exists, so the answer is
// 201 with the report (no draft) and the refusal in draftError.
func TestLiteParentReportGenerateStudentLeftMidCall(t *testing.T) {
	prov := &parentHookProvider{inner: gateway.NewSequenceStubProvider(weeklyReply(parentValidReply))}
	h, pool, teacher, classID, studentID := parentFixture(t, prov)
	prov.before = func() {
		mustExec(t, pool, `DELETE FROM enrollments WHERE user_id = $1 AND class_id = $2`, studentID, classID)
	}

	code, body := parentDo(t, h, teacher, "POST", parentReportsPath(classID, studentID), "", nil)
	if code != http.StatusCreated {
		t.Fatalf("generate = %d %s, want 201", code, body)
	}
	var got parentReportResp
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatal(err)
	}
	if got.Report.ID == "" || got.DraftError == nil || *got.DraftError != "该学生已不在本班" || got.Report.Draft != nil || got.Report.Body != nil {
		t.Fatalf("generate = %s, want the report id, no draft and draftError 该学生已不在本班", body)
	}
	if n := weeklyCount(t, pool, `SELECT count(*) FROM lite_parent_report WHERE id = $1 AND draft IS NULL`, uuid.MustParse(got.Report.ID)); n != 1 {
		t.Fatalf("stored rows without a draft = %d, want 1", n)
	}
	if n := len(llmCallUsers(t, pool, "lite_parent_report")); n != 1 {
		t.Fatalf("llm_call rows = %d, want 1", n)
	}
}

// TestLiteParentReportOtherSchoolAdmin (fix round 1): an admin of another
// lite school gets 404 on a report of this school.
func TestLiteParentReportOtherSchoolAdmin(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(weeklyReply(parentValidReply))
	h, pool, teacher, classID, studentID := parentFixture(t, prov)
	rep := generateParentReport(t, h, teacher, classID, studentID).Report

	otherSchool := seedSecondSchool(t, pool)
	mustExec(t, pool, `UPDATE schools SET edition = 'lite' WHERE id = $1`, otherSchool)
	adminID := createTeacher(t, pool, otherSchool, "lp-other-admin@other.local")
	mustExec(t, pool, `UPDATE users SET role = 'admin' WHERE id = $1`, adminID)
	admin := signInAs(t, pool, adminID)

	for _, rt := range []struct{ method, path, body string }{
		{"GET", parentReportPath(rep.ID), ""},
		{"PATCH", parentReportPath(rep.ID), `{"hidden":{"moments":["雨水不是废水"],"keywords":[]}}`},
		{"GET", classParentReportsPath(classID), ""},
	} {
		code, body := parentDo(t, h, admin, rt.method, rt.path, rt.body, nil)
		wantParentError(t, "other-school admin "+rt.method+" "+rt.path, code, body, http.StatusNotFound, "not_found")
	}
	var after parentReportResp
	parentDo(t, h, teacher, "GET", parentReportPath(rep.ID), "", &after)
	if len(after.Report.Hidden.Moments) != 0 || !reflect.DeepEqual(after.Report.Body, rep.Body) {
		t.Fatalf("report after other-school admin = %+v, want unchanged", after.Report)
	}
}

// TestLiteParentReportOtherTeacher: another teacher gets 404 on every route,
// a student gets 403, and nothing reaches the model.
func TestLiteParentReportOtherTeacher(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(weeklyReply(parentValidReply))
	h, pool, teacher, classID, studentID := parentFixture(t, prov)
	rep := generateParentReport(t, h, teacher, classID, studentID).Report
	other := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "lp-other-teacher@demo.local"))
	student := signInAs(t, pool, studentID)

	routes := []struct{ method, path, body string }{
		{"GET", parentReportPath(rep.ID), ""},
		{"PATCH", parentReportPath(rep.ID), parentBodyJSON(t, map[string]string{"overview": "改写"})},
		{"PATCH", parentReportPath(rep.ID), `{"hidden":{"moments":["雨水不是废水"],"keywords":[]}}`},
		{"POST", parentReportPath(rep.ID) + "/redraft", `{"replaceBody":true}`},
		{"POST", parentReportsPath(classID, studentID), ""},
		{"GET", parentReportsPath(classID, studentID), ""},
		{"GET", classParentReportsPath(classID), ""},
	}
	for _, rt := range routes {
		code, body := parentDo(t, h, other, rt.method, rt.path, rt.body, nil)
		wantParentError(t, "other teacher "+rt.method+" "+rt.path, code, body, http.StatusNotFound, "not_found")
		if code, body := parentDo(t, h, student, rt.method, rt.path, rt.body, nil); code != http.StatusForbidden {
			t.Fatalf("student %s %s = %d %s, want 403", rt.method, rt.path, code, body)
		}
	}
	for _, id := range []string{uuid.NewString(), "not-a-uuid"} {
		code, body := parentDo(t, h, teacher, "GET", parentReportPath(id), "", nil)
		wantParentError(t, "GET unknown report "+id, code, body, http.StatusNotFound, "not_found")
	}
	if prov.Calls != 1 {
		t.Fatalf("provider calls = %d, want 1 (the generate)", prov.Calls)
	}
	var after parentReportResp
	parentDo(t, h, teacher, "GET", parentReportPath(rep.ID), "", &after)
	if len(after.Report.Hidden.Moments) != 0 || !reflect.DeepEqual(after.Report.Body, rep.Body) {
		t.Fatalf("report after other teacher = %+v, want unchanged", after.Report)
	}
}

// TestLiteParentReportStudentLeft (Ruling 5): after she leaves the class the
// teacher can still read and edit (body and hidden); redraft answers 409
// student_left; generate for her is 404.
func TestLiteParentReportStudentLeft(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(weeklyReply(parentValidReply))
	h, pool, teacher, classID, studentID := parentFixture(t, prov)
	rep := generateParentReport(t, h, teacher, classID, studentID).Report
	path := parentReportPath(rep.ID)
	mustExec(t, pool, `DELETE FROM enrollments WHERE user_id = $1 AND class_id = $2`, studentID, classID)

	if code, body := parentDo(t, h, teacher, "GET", path, "", nil); code != http.StatusOK {
		t.Fatalf("GET after she left = %d %s", code, body)
	}
	if code, body := parentDo(t, h, teacher, "PATCH", path, parentBodyJSON(t, map[string]string{"overview": "改写"}), nil); code != http.StatusOK {
		t.Fatalf("PATCH after she left = %d %s", code, body)
	}
	if code, body := parentDo(t, h, teacher, "PATCH", path, `{"hidden":{"moments":["雨水不是废水"],"keywords":[]}}`, nil); code != http.StatusOK {
		t.Fatalf("PATCH hidden after she left = %d %s", code, body)
	}
	code, body := parentDo(t, h, teacher, "POST", path+"/redraft", `{"replaceBody":true}`, nil)
	wantParentError(t, "redraft after she left", code, body, http.StatusConflict, "student_left")
	if !strings.Contains(body, "该学生已不在本班") {
		t.Fatalf("redraft message = %s", body)
	}
	code, body = parentDo(t, h, teacher, "POST", parentReportsPath(classID, studentID), "", nil)
	wantParentError(t, "generate after she left", code, body, http.StatusNotFound, "not_found")
	var list parentListResp
	if code, body := parentDo(t, h, teacher, "GET", classParentReportsPath(classID), "", &list); code != http.StatusOK || len(list.Reports) != 1 {
		t.Fatalf("class list after she left = %d %s, want her report", code, body)
	}
	if prov.Calls != 1 {
		t.Fatalf("provider calls = %d, want 1 (the generate)", prov.Calls)
	}
}

// TestLiteParentReportInvalidRange: a malformed, reversed, future or
// over-long range is 400 invalid_range before any spend.
func TestLiteParentReportInvalidRange(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(weeklyReply(parentValidReply))
	h, pool, teacher, classID, studentID := parentFixture(t, prov)
	yesterday := liteweek.Day(time.Now()).AddDate(0, 0, -1)
	path := parentReportsPath(classID, studentID)

	for _, tc := range []struct{ what, body string }{
		{"malformed", `{"rangeStart":"2026/08/01","rangeEnd":"2026-08-10"}`},
		{"reversed", parentRangeJSON(yesterday, yesterday.AddDate(0, 0, -5))},
		{"future end", parentRangeJSON(yesterday, yesterday.AddDate(0, 0, 2))},
		{"over 366 days", parentRangeJSON(yesterday.AddDate(0, 0, -400), yesterday)},
	} {
		code, body := parentDo(t, h, teacher, "POST", path, tc.body, nil)
		wantParentError(t, "generate "+tc.what, code, body, http.StatusBadRequest, "invalid_range")
		if !strings.Contains(body, "请选择有效的日期范围") {
			t.Fatalf("%s message = %s", tc.what, body)
		}
	}
	code, body := parentDo(t, h, teacher, "POST", path, `{`, nil)
	wantParentError(t, "generate not json", code, body, http.StatusBadRequest, "bad_json")
	if prov.Calls != 0 || weeklyCount(t, pool, `SELECT count(*) FROM lite_parent_report`) != 0 {
		t.Fatalf("invalid ranges spent: calls = %d", prov.Calls)
	}
}

// TestLiteParentReportRangeBeforeStart (Ruling 6): a range that ends before
// she joined is 400 range_before_start; one that starts before and ends after
// runs, even with nothing in it.
func TestLiteParentReportRangeBeforeStart(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(weeklyReply(parentQuietReply))
	h, pool, teacher, classID, studentID := liteTeacherFixtureWithProvider(t, prov)
	path := parentReportsPath(classID, studentID)
	wantBefore := func(what, body string) {
		t.Helper()
		code, resp := parentDo(t, h, teacher, "POST", path, body, nil)
		wantParentError(t, what, code, resp, http.StatusBadRequest, "range_before_start")
		if !strings.Contains(resp, "该时间段早于学生加入班级的时间") {
			t.Fatalf("%s message = %s", what, resp)
		}
	}

	// The fixture enrolls her now; the default range ends yesterday.
	wantBefore("default range, enrolled today", "")

	joined := liteweek.Day(time.Now()).AddDate(0, 0, -5)
	mustExec(t, pool, `UPDATE enrollments SET created_at = $3 WHERE class_id = $1 AND user_id = $2`, classID, studentID, joined)
	wantBefore("range ending the day before she joined", parentRangeJSON(joined.AddDate(0, 0, -10), joined.AddDate(0, 0, -1)))
	if prov.Calls != 0 {
		t.Fatalf("refused ranges called the model %d times", prov.Calls)
	}

	var got parentReportResp
	if code, body := parentDo(t, h, teacher, "POST", path, parentRangeJSON(joined.AddDate(0, 0, -3), joined), &got); code != http.StatusCreated {
		t.Fatalf("overlapping range = %d %s", code, body)
	}
	if got.DraftError != nil || got.Report.RangeStart != joined.AddDate(0, 0, -3).Format("2006-01-02") ||
		got.Report.Facts.ActiveDays != 0 || !reflect.DeepEqual(got.Report.Sections, []string{"overview", "next"}) {
		t.Fatalf("overlapping range report = %+v err = %v", got.Report, got.DraftError)
	}
	if prov.Calls != 1 {
		t.Fatalf("provider calls = %d, want 1", prov.Calls)
	}
}

// TestLiteParentReportGetsNeverSpend (Ruling 10): the list and report GETs
// never load facts: no model call and no new atom_report row, even with a
// finished writing that has no report.
func TestLiteParentReportGetsNeverSpend(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(weeklyReply(parentValidReply))
	h, pool, teacher, classID, studentID := parentFixture(t, prov)
	rep := generateParentReport(t, h, teacher, classID, studentID).Report

	bare := seedLiteWritingForUser(t, pool, studentID, "active")
	mustExec(t, pool, `UPDATE writing SET status = 'finished', title = '雨水花园调查报告', finished_at = $2 WHERE atom_id = $1`,
		bare, liteweek.Day(time.Now()).AddDate(0, 0, -2))
	reports := weeklyCount(t, pool, `SELECT count(*) FROM atom_report`)
	calls := prov.Calls

	for _, path := range []string{parentReportsPath(classID, studentID), classParentReportsPath(classID), parentReportPath(rep.ID)} {
		if code, body := parentDo(t, h, teacher, "GET", path, "", nil); code != http.StatusOK {
			t.Fatalf("GET %s = %d %s", path, code, body)
		}
	}
	if prov.Calls != calls {
		t.Fatalf("GETs called the provider: %d → %d", calls, prov.Calls)
	}
	if n := weeklyCount(t, pool, `SELECT count(*) FROM atom_report`); n != reports {
		t.Fatalf("atom_report rows after GETs = %d, want %d", n, reports)
	}
	if n := weeklyCount(t, pool, `SELECT count(*) FROM atom_report WHERE atom_id = $1`, bare); n != 0 {
		t.Fatalf("a GET created a report for the finished writing")
	}
}

// TestLiteParentReportEntitlementBeforeSpend: without entitlement, generate
// and redraft answer 403 before loading facts or calling the model.
func TestLiteParentReportEntitlementBeforeSpend(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(weeklyReply(parentPlainReply))
	h, pool, teacher, classID, studentID := liteTeacherFixtureWithProvider(t, prov)
	backdateWeeklyStart(t, pool, classID)
	reading := seedParentReading(t, pool, studentID, false)
	deny := func(context.Context, User) (bool, error) { return false, nil }
	path := parentReportsPath(classID, studentID)

	restore := SetLiteTeacherEntitlementForTest(deny)
	t.Cleanup(restore)
	code, body := parentDo(t, h, teacher, "POST", path, "", nil)
	wantParentError(t, "generate not entitled", code, body, http.StatusForbidden, "not_entitled")
	if prov.Calls != 0 || countAllLLMCalls(t, pool) != 0 ||
		weeklyCount(t, pool, `SELECT count(*) FROM lite_parent_report`) != 0 ||
		weeklyCount(t, pool, `SELECT count(*) FROM atom_report WHERE atom_id = $1`, reading) != 0 {
		t.Fatalf("a refused generate spent or wrote: calls = %d", prov.Calls)
	}

	restore()
	rep := generateParentReport(t, h, teacher, classID, studentID).Report
	// The allowed generate does create the phase-1 report the refused one did not.
	if weeklyCount(t, pool, `SELECT count(*) FROM atom_report WHERE atom_id = $1`, reading) != 1 || prov.Calls != 1 {
		t.Fatalf("allowed generate: calls = %d", prov.Calls)
	}

	t.Cleanup(SetLiteTeacherEntitlementForTest(deny))
	code, body = parentDo(t, h, teacher, "POST", parentReportPath(rep.ID)+"/redraft", `{"replaceBody":true}`, nil)
	wantParentError(t, "redraft not entitled", code, body, http.StatusForbidden, "not_entitled")
	if prov.Calls != 1 || countAllLLMCalls(t, pool) != 1 {
		t.Fatalf("a refused redraft spent: calls = %d", prov.Calls)
	}
}
