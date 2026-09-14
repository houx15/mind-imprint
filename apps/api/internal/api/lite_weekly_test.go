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
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/liteweek"
	"mindimprint/api/internal/liteweekly"
	"mindimprint/api/internal/store/sqlc"
)

// weeklyWindow is the last completed Beijing week; every seeded time is an
// offset from its start, so the test does not depend on today's weekday.
func weeklyWindow() (ws, we time.Time) {
	ws = liteweek.LatestCompleted(time.Now())
	return ws, ws.AddDate(0, 0, 7)
}

func mustExec(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("exec %q: %v", sql, err)
	}
}

func seedReport(t *testing.T, pool *pgxpool.Pool, atomID uuid.UUID, kind, report string) {
	t.Helper()
	mustExec(t, pool, `INSERT INTO atom_report (atom_id, kind, report) VALUES ($1, $2, $3::jsonb)`, atomID, kind, report)
}

func seedWeekKeyword(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID, text string, at time.Time) {
	t.Helper()
	mustExec(t, pool, `INSERT INTO interest_keyword (user_id, text_zh, norm, field, first_seen_at) VALUES ($1, $2, $2, 'society', $3)`, userID, text, at)
}

func TestLiteWeeklyStudentFacts(t *testing.T) {
	h, pool, teacher, classIDStr, studentID := liteTeacherFixture(t)
	ctx := context.Background()
	classID := uuid.MustParse(classIDStr)
	ws, we := weeklyWindow()

	// Two bucket days of 600 s and a student message on a third day.
	reading := seedLiteReadingForUser(t, pool, studentID, "finished", 0)
	mustExec(t, pool, `UPDATE reading SET finished_at = $2 WHERE atom_id = $1`, reading, ws.AddDate(0, 0, 2).Add(10*time.Hour))
	seedBucket(t, pool, reading, ws, 600)
	seedBucket(t, pool, reading, ws.AddDate(0, 0, 1), 600)
	seedAtomMessageAt(t, pool, reading, ws.AddDate(0, 0, 3).Add(9*time.Hour))
	seedReport(t, pool, reading, "reading", `{"version":1,"moments":[{"quote":"雨落在屋檐上","where":""},{"quote":"  ","where":""}]}`)

	// An assignment due in the week, never started.
	aid := createAssignment(t, h, teacher, classIDStr, writingAssignmentBody([]string{studentID.String()}))
	mustExec(t, pool, `UPDATE lite_assignment SET due_at = $2 WHERE id = $1`, aid, ws.AddDate(0, 0, 4))

	// A writing last touched 10 days before week end.
	writing := seedLiteWritingForUser(t, pool, studentID, "active")
	mustExec(t, pool, `UPDATE atom SET created_at = $2, last_activity_at = $3 WHERE id = $1`, writing, we.AddDate(0, 0, -12), we.AddDate(0, 0, -10))

	seedWeekKeyword(t, pool, studentID, "金融", ws.AddDate(0, 0, 1).Add(8*time.Hour))

	api := New(DepsForTest(pool))
	got, err := api.LoadLiteStudentWeekForTest(ctx, classID, studentID, ws)
	if err != nil {
		t.Fatal(err)
	}
	if got.ActiveDays != 3 || got.Minutes != 20 || got.Turns != 1 {
		t.Fatalf("activity = days %d minutes %d turns %d, want 3/20/1", got.ActiveDays, got.Minutes, got.Turns)
	}
	if len(got.Finished) != 1 || got.Finished[0] != (liteweekly.Item{Kind: "reading", Title: "Test reading"}) {
		t.Fatalf("finished = %+v", got.Finished)
	}
	if got.AssignmentsOverdue != 1 || got.AssignmentsDone != 0 || got.AssignmentsLate != 0 {
		t.Fatalf("assignments = done %d late %d overdue %d", got.AssignmentsDone, got.AssignmentsLate, got.AssignmentsOverdue)
	}
	if len(got.Stalled) != 1 || got.Stalled[0].Kind != "writing" {
		t.Fatalf("stalled = %+v", got.Stalled)
	}
	if len(got.NewKeywords) != 1 || got.NewKeywords[0] != "金融" {
		t.Fatalf("keywords = %+v", got.NewKeywords)
	}
	if len(got.Moments) != 1 || got.Moments[0].Quote != "雨落在屋檐上" || got.Moments[0].ItemTitle != "Test reading" {
		t.Fatalf("moments = %+v", got.Moments)
	}
}

// TestLiteWeeklyClassMembersAndMinutes: only current student members are
// loaded (a teacher-role user enrolled as a student is not), a student with
// no buckets at all has Minutes -1, and one with only older buckets has 0.
// Buckets from next week and previous-week activity land in the right fields.
func TestLiteWeeklyClassMembersAndMinutes(t *testing.T) {
	_, pool, _, classIDStr, studentID := liteTeacherFixture(t)
	ctx := context.Background()
	classID := uuid.MustParse(classIDStr)
	ws, we := weeklyWindow()

	older := createStudent(t, pool, SeedSchoolID, "lw-older@demo.local")
	enrollStudent(t, pool, older, classIDStr)
	r := seedLiteReadingForUser(t, pool, older, "active", 0)
	seedBucket(t, pool, r, ws.AddDate(0, 0, -3), 300) // previous week
	seedBucket(t, pool, r, we, 900)                   // next Monday: out of the week
	seedAtomMessageAt(t, pool, r, ws.Add(-time.Second))

	teacherMember := createTeacher(t, pool, SeedSchoolID, "lw-teacher@demo.local")
	enrollStudent(t, pool, teacherMember, classIDStr)

	api := New(DepsForTest(pool))
	weeks, err := api.LoadLiteClassWeekForTest(ctx, classID, ws)
	if err != nil {
		t.Fatal(err)
	}
	if len(weeks) != 2 {
		t.Fatalf("weeks = %d, want 2 (teacher excluded): %+v", len(weeks), weeks)
	}
	byID := map[string]liteweekly.StudentWeek{}
	for _, w := range weeks {
		byID[w.UserID] = w
	}
	if s := byID[studentID.String()]; s.Minutes != -1 || s.ActiveDays != 0 {
		t.Fatalf("no-bucket student = %+v, want Minutes -1", s)
	}
	o := byID[older.String()]
	if o.Minutes != 0 || o.ActiveDays != 0 || o.PrevActiveDays != 2 || o.Turns != 0 {
		t.Fatalf("older student = %+v, want Minutes 0, ActiveDays 0, PrevActiveDays 2 (the previous Friday bucket and the previous Sunday 23:59:59 message)", o)
	}
	if _, err := api.LoadLiteStudentWeekForTest(ctx, classID, teacherMember, ws); err == nil {
		t.Fatalf("teacher loaded as a student")
	}
}

// TestLiteWeeklyAssignmentStatesAtWeekEnd: status is judged at week end. Work
// finished after the week is still overdue for that week.
func TestLiteWeeklyAssignmentStatesAtWeekEnd(t *testing.T) {
	h, pool, teacher, classIDStr, studentID := liteTeacherFixture(t)
	ctx := context.Background()
	classID := uuid.MustParse(classIDStr)
	student := signInAs(t, pool, studentID)
	ws, we := weeklyWindow()

	assign := func(due, finished time.Time) {
		t.Helper()
		aid := createAssignment(t, h, teacher, classIDStr, writingAssignmentBody([]string{studentID.String()}))
		started := startAssignment(t, h, student, aid)
		mustExec(t, pool, `UPDATE lite_assignment SET due_at = $2 WHERE id = $1`, aid, due)
		mustExec(t, pool, `UPDATE writing SET status = 'finished', finished_at = $2 WHERE atom_id = $1`, started.AtomID, finished)
	}
	assign(ws.AddDate(0, 0, 4), we.Add(time.Hour))            // finished after week end → overdue
	assign(ws.AddDate(0, 0, 1), ws.AddDate(0, 0, 2))          // finished late in the week → done_late
	assign(ws.AddDate(0, 0, 5), ws.AddDate(0, 0, 2))          // on time → done
	assign(we.AddDate(0, 0, 1), ws.AddDate(0, 0, 2))          // due next week → not counted
	assign(ws.AddDate(0, 0, -2), ws.AddDate(0, 0, -1).Add(1)) // due last week → not counted

	got, err := New(DepsForTest(pool)).LoadLiteStudentWeekForTest(ctx, classID, studentID, ws)
	if err != nil {
		t.Fatal(err)
	}
	if got.AssignmentsOverdue != 1 || got.AssignmentsLate != 1 || got.AssignmentsDone != 1 {
		t.Fatalf("assignments = done %d late %d overdue %d, want 1/1/1", got.AssignmentsDone, got.AssignmentsLate, got.AssignmentsOverdue)
	}
}

// TestLiteWeeklyArchivedAssignments: archiving does not rewrite a week that
// has ended. An assignment due in the week, never started, and archived at or
// after week end is still overdue for that week; one archived before week end
// is not counted.
func TestLiteWeeklyArchivedAssignments(t *testing.T) {
	h, pool, teacher, classIDStr, studentID := liteTeacherFixture(t)
	ctx := context.Background()
	ws, we := weeklyWindow()

	archive := func(at time.Time) {
		t.Helper()
		aid := createAssignment(t, h, teacher, classIDStr, writingAssignmentBody([]string{studentID.String()}))
		mustExec(t, pool, `UPDATE lite_assignment SET due_at = $2, archived_at = $3 WHERE id = $1`, aid, ws.AddDate(0, 0, 3), at)
	}
	archive(we.Add(time.Hour))  // archived after week end → overdue
	archive(we)                 // archived exactly at week end → overdue
	archive(we.Add(-time.Hour)) // archived before week end → not counted

	got, err := New(DepsForTest(pool)).LoadLiteStudentWeekForTest(ctx, uuid.MustParse(classIDStr), studentID, ws)
	if err != nil {
		t.Fatal(err)
	}
	if got.AssignmentsOverdue != 2 || got.AssignmentsDone != 0 || got.AssignmentsLate != 0 {
		t.Fatalf("assignments = done %d late %d overdue %d, want 0/0/2", got.AssignmentsDone, got.AssignmentsLate, got.AssignmentsOverdue)
	}
}

// TestLiteWeeklyNoTeacherTextInMoments: a report whose prose is still pending
// contributes no moments, a quote that is the assigned prompt is dropped, and
// an assigned project with no name never uses the teacher's idea as its title.
func TestLiteWeeklyNoTeacherTextInMoments(t *testing.T) {
	h, pool, teacher, classIDStr, studentID := liteTeacherFixture(t)
	ctx := context.Background()
	classID := uuid.MustParse(classIDStr)
	student := signInAs(t, pool, studentID)
	ws, _ := weeklyWindow()
	inWeek := ws.AddDate(0, 0, 2)

	aid := createAssignment(t, h, teacher, classIDStr, writingAssignmentBody([]string{studentID.String()}))
	started := startAssignment(t, h, student, aid)
	writing := uuid.MustParse(started.AtomID)
	mustExec(t, pool, `UPDATE writing SET status = 'finished', finished_at = $2 WHERE atom_id = $1`, writing, inWeek)
	// Blank title: the title falls back to the assignment title 「雨」.
	mustExec(t, pool, `UPDATE writing SET title = '' WHERE atom_id = $1`, writing)
	// Prompt 写一篇关于雨的记叙文 (10 runes): equal → dropped; 9-rune substring →
	// dropped; 7-rune substring and her short 「雨」 → kept.
	seedReport(t, pool, writing, "writing", `{"moments":[`+
		`{"quote":"写一篇关于雨的记叙文","where":""},`+
		`{"quote":"一篇关于雨的记叙文","where":""},`+
		`{"quote":"关于雨的记叙文","where":""},`+
		`{"quote":"雨","where":""},`+
		`{"quote":"雨把街道洗亮了","where":""}]}`)

	pending := seedLiteReadingForUser(t, pool, studentID, "finished", 0)
	mustExec(t, pool, `UPDATE reading SET finished_at = $2 WHERE atom_id = $1`, pending, inWeek)
	seedReport(t, pool, pending, "reading", `{"prosePending":true,"moments":[{"quote":"还没生成的句子","where":""}]}`)

	q := sqlc.New(pool)
	at, err := q.CreateAtom(ctx, sqlc.CreateAtomParams{Kind: "project", UserID: studentID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := q.CreatePblProject(ctx, sqlc.CreatePblProjectParams{AtomID: at.ID, Idea: "老师的驱动问题", Kind: "investigation"}); err != nil {
		t.Fatal(err)
	}
	mustExec(t, pool, `UPDATE pbl_project SET assigned = true, name = '', finished_at = $2 WHERE atom_id = $1`, at.ID, inWeek)

	got, err := New(DepsForTest(pool)).LoadLiteStudentWeekForTest(ctx, classID, studentID, ws)
	if err != nil {
		t.Fatal(err)
	}
	wantMoments := []liteweekly.Moment{{Quote: "关于雨的记叙文", ItemTitle: "雨"}, {Quote: "雨", ItemTitle: "雨"}, {Quote: "雨把街道洗亮了", ItemTitle: "雨"}}
	if !reflect.DeepEqual(got.Moments, wantMoments) {
		t.Fatalf("moments = %+v, want %+v", got.Moments, wantMoments)
	}
	titles := map[string]string{}
	for _, it := range got.Finished {
		titles[it.Kind] = it.Title
	}
	// The assigned project has no name and no recipient row: kind label, never
	// the teacher's idea and never ''.
	want := map[string]string{"writing": "雨", "reading": "Test reading", "project": "项目"}
	if len(got.Finished) != 3 || !reflect.DeepEqual(titles, want) {
		t.Fatalf("finished = %+v, want titles %v", got.Finished, want)
	}
}

func seedOldReading(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID, title string, created time.Time) uuid.UUID {
	t.Helper()
	id := seedLiteReadingForUser(t, pool, userID, "active", 0)
	mustExec(t, pool, `UPDATE reading SET title = $2 WHERE atom_id = $1`, id, title)
	mustExec(t, pool, `UPDATE atom SET created_at = $2 WHERE id = $1`, id, created)
	return id
}

func seedOldWriting(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID, title string, created time.Time) uuid.UUID {
	t.Helper()
	id := seedLiteWritingForUser(t, pool, userID, "active")
	mustExec(t, pool, `UPDATE writing SET title = $2 WHERE atom_id = $1`, id, title)
	mustExec(t, pool, `UPDATE atom SET created_at = $2 WHERE id = $1`, id, created)
	return id
}

func seedWeekProject(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID, name, status string, created time.Time) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	q := sqlc.New(pool)
	at, err := q.CreateAtom(ctx, sqlc.CreateAtomParams{Kind: "project", UserID: userID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := q.CreatePblProject(ctx, sqlc.CreatePblProjectParams{AtomID: at.ID, Idea: "她的想法", Kind: "investigation"}); err != nil {
		t.Fatal(err)
	}
	mustExec(t, pool, `UPDATE pbl_project SET name = $2, status = $3 WHERE atom_id = $1`, at.ID, name, status)
	mustExec(t, pool, `UPDATE atom SET created_at = $2 WHERE id = $1`, at.ID, created)
	return at.ID
}

// TestLiteWeeklyStalled: stalled is decided from data before week end — an
// item unfinished at week end, created more than 7 days before it, with no
// positive bucket and no student message in the week's last 7 days.
func TestLiteWeeklyStalled(t *testing.T) {
	_, pool, _, classIDStr, studentID := liteTeacherFixture(t)
	ctx := context.Background()
	ws, we := weeklyWindow()
	before := ws.AddDate(0, 0, -3)

	seedWeekProject(t, pool, studentID, "进行中的项目", "running", before)  // stalled
	seedWeekProject(t, pool, studentID, "已归档的项目", "archived", before) // excluded
	late := seedWeekProject(t, pool, studentID, "周末后完成的项目", "keeping", before)
	mustExec(t, pool, `UPDATE pbl_project SET finished_at = $2 WHERE atom_id = $1`, late, we.Add(time.Hour)) // stalled
	done := seedWeekProject(t, pool, studentID, "周中完成的项目", "keeping", before)
	mustExec(t, pool, `UPDATE pbl_project SET finished_at = $2 WHERE atom_id = $1`, done, ws.AddDate(0, 0, 1)) // excluded

	seedOldReading(t, pool, studentID, "周末后创建", we.Add(time.Hour))  // excluded
	seedOldReading(t, pool, studentID, "该周创建", ws.AddDate(0, 0, 1)) // excluded
	readLate := seedOldReading(t, pool, studentID, "周末后读完", before)
	mustExec(t, pool, `UPDATE reading SET status = 'finished', finished_at = $2 WHERE atom_id = $1`, readLate, we.Add(2*time.Hour)) // stalled

	after := seedOldWriting(t, pool, studentID, "周末后才动", before) // stalled
	seedBucket(t, pool, after, we, 600)
	seedAtomMessageAt(t, pool, after, we.Add(time.Hour))
	mustExec(t, pool, `UPDATE atom SET last_activity_at = now() WHERE id = $1`, after)

	msg := seedOldWriting(t, pool, studentID, "该周有消息", before) // excluded
	seedAtomMessageAt(t, pool, msg, we.Add(-time.Second))
	bucket := seedOldWriting(t, pool, studentID, "该周有时长", before) // excluded
	seedBucket(t, pool, bucket, ws.AddDate(0, 0, 6), 60)
	zero := seedOldWriting(t, pool, studentID, "零秒日格", before) // stalled
	seedBucket(t, pool, zero, ws.AddDate(0, 0, 2), 0)

	got, err := New(DepsForTest(pool)).LoadLiteStudentWeekForTest(ctx, uuid.MustParse(classIDStr), studentID, ws)
	if err != nil {
		t.Fatal(err)
	}
	gotTitles := map[string]string{}
	for _, it := range got.Stalled {
		gotTitles[it.Title] = it.Kind
	}
	want := map[string]string{
		"进行中的项目": "project", "周末后完成的项目": "project",
		"周末后读完": "reading", "周末后才动": "writing", "零秒日格": "writing",
	}
	if len(got.Stalled) != len(want) || !reflect.DeepEqual(gotTitles, want) {
		t.Fatalf("stalled = %+v, want %v", got.Stalled, want)
	}
}

// TestLiteWeeklyFinishedBoundaryAndOddReports: finished exactly at week start
// is in, exactly at week end is out; a report with null or missing moments
// gives no moments; an unreadable moments value is skipped without failing.
func TestLiteWeeklyFinishedBoundaryAndOddReports(t *testing.T) {
	_, pool, _, classIDStr, studentID := liteTeacherFixture(t)
	ctx := context.Background()
	ws, we := weeklyWindow()

	finishReading := func(title string, at time.Time, report string) {
		t.Helper()
		id := seedOldReading(t, pool, studentID, title, ws)
		mustExec(t, pool, `UPDATE reading SET status = 'finished', finished_at = $2 WHERE atom_id = $1`, id, at)
		seedReport(t, pool, id, "reading", report)
	}
	finishReading("周一零点", ws, `{"moments":null}`)
	finishReading("下周一零点", we, `{"moments":[{"quote":"不该出现","where":""}]}`)
	finishReading("坏报告", ws.AddDate(0, 0, 2), `{"moments":"oops"}`)
	w := seedOldWriting(t, pool, studentID, "没有金句", ws)
	mustExec(t, pool, `UPDATE writing SET status = 'finished', finished_at = $2 WHERE atom_id = $1`, w, ws.AddDate(0, 0, 1))
	seedReport(t, pool, w, "writing", `{"version":1}`)

	got, err := New(DepsForTest(pool)).LoadLiteStudentWeekForTest(ctx, uuid.MustParse(classIDStr), studentID, ws)
	if err != nil {
		t.Fatal(err)
	}
	want := []liteweekly.Item{{Kind: "reading", Title: "周一零点"}, {Kind: "writing", Title: "没有金句"}, {Kind: "reading", Title: "坏报告"}}
	if !reflect.DeepEqual(got.Finished, want) {
		t.Fatalf("finished = %+v, want %+v", got.Finished, want)
	}
	if len(got.Moments) != 0 {
		t.Fatalf("moments = %+v, want none", got.Moments)
	}
}

// TestLiteWeeklyTwoStudentsNoCrossover: batched rows are grouped by user id.
func TestLiteWeeklyTwoStudentsNoCrossover(t *testing.T) {
	_, pool, _, classIDStr, first := liteTeacherFixture(t)
	ctx := context.Background()
	ws, _ := weeklyWindow()
	second := createStudent(t, pool, SeedSchoolID, "lw-second@demo.local")
	enrollStudent(t, pool, second, classIDStr)

	prefix := map[uuid.UUID]string{first: "甲", second: "乙"}
	for id, p := range prefix {
		r := seedOldReading(t, pool, id, p+"读完", ws)
		mustExec(t, pool, `UPDATE reading SET status = 'finished', finished_at = $2 WHERE atom_id = $1`, r, ws.AddDate(0, 0, 3))
		seedReport(t, pool, r, "reading", `{"moments":[{"quote":"`+p+`的句子","where":""}]}`)
		seedOldWriting(t, pool, id, p+"停滞", ws.AddDate(0, 0, -3))
	}

	weeks, err := New(DepsForTest(pool)).LoadLiteClassWeekForTest(ctx, uuid.MustParse(classIDStr), ws)
	if err != nil {
		t.Fatal(err)
	}
	if len(weeks) != 2 {
		t.Fatalf("weeks = %+v", weeks)
	}
	for _, s := range weeks {
		p := prefix[uuid.MustParse(s.UserID)]
		if !reflect.DeepEqual(s.Finished, []liteweekly.Item{{Kind: "reading", Title: p + "读完"}}) ||
			!reflect.DeepEqual(s.Stalled, []liteweekly.Item{{Kind: "writing", Title: p + "停滞"}}) ||
			!reflect.DeepEqual(s.Moments, []liteweekly.Moment{{Quote: p + "的句子", ItemTitle: p + "读完"}}) {
			t.Fatalf("student %s = %+v", p, s)
		}
	}
}

// --- weekly endpoints --------------------------------------------------------

type weeklyCardJSON struct {
	Kind     string `json:"kind"`
	Code     string `json:"code"`
	Label    string `json:"label"`
	Evidence string `json:"evidence"`
	UserID   string `json:"userId"`
	Name     string `json:"name"`
}

type studentWeeklyJSON struct {
	WeekStart string `json:"weekStart"`
	WeekLabel string `json:"weekLabel"`
	Title     string `json:"title"`
	IsLatest  bool   `json:"isLatest"`
	HasPrev   bool   `json:"hasPrev"`
	Empty     bool   `json:"empty"`
	Facts     struct {
		ActiveDays         int      `json:"activeDays"`
		Minutes            int      `json:"minutes"`
		Turns              int      `json:"turns"`
		AssignmentsOverdue int      `json:"assignmentsOverdue"`
		NewKeywords        []string `json:"newKeywords"`
		Moments            []struct {
			Quote     string `json:"quote"`
			ItemTitle string `json:"itemTitle"`
		} `json:"moments"`
	} `json:"facts"`
	Cards []weeklyCardJSON `json:"cards"`
	Prose *struct {
		Summary     string `json:"summary"`
		Suggestions []struct {
			Text         string `json:"text"`
			EvidenceCode string `json:"evidenceCode"`
		} `json:"suggestions"`
	} `json:"prose"`
	ProseReady bool    `json:"proseReady"`
	ProseError *string `json:"proseError"`
}

type classWeeklyJSON struct {
	WeekStart string `json:"weekStart"`
	WeekLabel string `json:"weekLabel"`
	Title     string `json:"title"`
	IsLatest  bool   `json:"isLatest"`
	HasPrev   bool   `json:"hasPrev"`
	Empty     bool   `json:"empty"`
	Stats     struct {
		ClassSize      int `json:"classSize"`
		ActiveStudents int `json:"activeStudents"`
		Minutes        int `json:"minutes"`
		Turns          int `json:"turns"`
		Finished       int `json:"finished"`
		AssignmentRate int `json:"assignmentRate"`
	} `json:"stats"`
	Praise []weeklyCardJSON `json:"praise"`
	Watch  []weeklyCardJSON `json:"watch"`
	Prose  *struct {
		Comment string `json:"comment"`
		Cards   []struct {
			UserID string `json:"userId"`
			Lead   string `json:"lead"`
			Action string `json:"action"`
		} `json:"cards"`
	} `json:"prose"`
	ProseReady bool    `json:"proseReady"`
	ProseError *string `json:"proseError"`
}

func weeklyReply(text string) []gateway.StreamEvent {
	return []gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: text},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 100, OutputTokens: 50}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	}
}

func studentWeeklyPath(classID string, userID uuid.UUID) string {
	return "/api/v1/lite/teacher/classes/" + classID + "/students/" + userID.String() + "/weekly"
}

func classWeeklyPath(classID string) string {
	return "/api/v1/lite/teacher/classes/" + classID + "/weekly"
}

// weeklyDo sends one request and decodes a 200 body into out.
func weeklyDo(t *testing.T, h http.Handler, c *http.Cookie, method, path string, out any) (int, string) {
	t.Helper()
	rec := doJSON(t, h, c, method, path, "")
	if out != nil && rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
			t.Fatalf("decode %s %s: %v body=%s", method, path, err, rec.Body)
		}
	}
	return rec.Code, rec.Body.String()
}

// llmCallUsers returns the user_id of every llm_call row with this purpose.
func llmCallUsers(t *testing.T, pool *pgxpool.Pool, purpose string) []uuid.UUID {
	t.Helper()
	rows, err := pool.Query(context.Background(), `SELECT user_id FROM llm_call WHERE purpose = $1`, purpose)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		out = append(out, id)
	}
	return out
}

func userIDByEmail(t *testing.T, pool *pgxpool.Pool, email string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(context.Background(), `SELECT id FROM users WHERE email = $1`, email).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func weeklyCount(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// seedWeeklyStudentWeek seeds the facts of TestLiteWeeklyStudentFacts in the
// latest completed week: 3 active days, 20 minutes, 1 turn, a finished
// 《Test reading》 with the moment 「雨落在屋檐上」, an overdue assignment, a
// stalled 《Test writing》 and the keyword 金融. Cards: overdue (watch) and
// new_interest (praise).
func seedWeeklyStudentWeek(t *testing.T, h http.Handler, pool *pgxpool.Pool, teacher *http.Cookie, classID string, studentID uuid.UUID) {
	t.Helper()
	backdateWeeklyStart(t, pool, classID)
	ws, we := weeklyWindow()
	reading := seedLiteReadingForUser(t, pool, studentID, "finished", 0)
	mustExec(t, pool, `UPDATE reading SET finished_at = $2 WHERE atom_id = $1`, reading, ws.AddDate(0, 0, 2).Add(10*time.Hour))
	seedBucket(t, pool, reading, ws, 600)
	seedBucket(t, pool, reading, ws.AddDate(0, 0, 1), 600)
	seedAtomMessageAt(t, pool, reading, ws.AddDate(0, 0, 3).Add(9*time.Hour))
	seedReport(t, pool, reading, "reading", `{"version":1,"moments":[{"quote":"雨落在屋檐上","where":""}]}`)

	aid := createAssignment(t, h, teacher, classID, writingAssignmentBody([]string{studentID.String()}))
	mustExec(t, pool, `UPDATE lite_assignment SET due_at = $2 WHERE id = $1`, aid, ws.AddDate(0, 0, 4))

	writing := seedLiteWritingForUser(t, pool, studentID, "active")
	mustExec(t, pool, `UPDATE atom SET created_at = $2 WHERE id = $1`, writing, we.AddDate(0, 0, -12))

	seedWeekKeyword(t, pool, studentID, "金融", ws.AddDate(0, 0, 1).Add(8*time.Hour))
}

const weeklyValidStudentReply = `{"summary":"该周活跃 3 天，读完《Test reading》，写下「雨落在屋檐上」。有 1 份作业逾期。","suggestions":[{"text":"请她说明逾期的作业卡在哪一步。","evidenceCode":"overdue"},{"text":"请她讲一讲对金融的兴趣从哪里来。","evidenceCode":"new_interest"}]}`

const weeklyFabricatedStudentReply = `{"summary":"她写下「雨是天空的眼泪」。","suggestions":[{"text":"请她说明逾期的作业卡在哪一步。","evidenceCode":"overdue"}]}`

func TestLiteWeeklyStudentGetCallsNoModel(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(weeklyReply(weeklyValidStudentReply))
	h, pool, teacher, classID, studentID := liteTeacherFixtureWithProvider(t, prov)
	seedWeeklyStudentWeek(t, h, pool, teacher, classID, studentID)
	ws, _ := weeklyWindow()

	var got studentWeeklyJSON
	if code, body := weeklyDo(t, h, teacher, "GET", studentWeeklyPath(classID, studentID), &got); code != http.StatusOK {
		t.Fatalf("GET = %d body=%s", code, body)
	}
	if prov.Calls != 0 {
		t.Fatalf("GET called the provider %d times", prov.Calls)
	}
	if got.ProseReady || got.Prose != nil {
		t.Fatalf("GET prose = %+v ready=%v, want none", got.Prose, got.ProseReady)
	}
	label := liteweek.Label(ws)
	if got.WeekStart != ws.In(liteweek.Beijing).Format("2006-01-02") || got.WeekLabel != label ||
		got.Title != "上周表现总结 · "+label || !got.IsLatest {
		t.Fatalf("week = %q %q %q latest=%v", got.WeekStart, got.WeekLabel, got.Title, got.IsLatest)
	}
	wantCards := []weeklyCardJSON{
		{Kind: "watch", Code: "overdue", Label: "作业逾期", Evidence: "该周到期的作业中有 1 份未完成。"},
		{Kind: "praise", Code: "new_interest", Label: "新的兴趣", Evidence: "兴趣树新增关键词：金融。"},
	}
	if !reflect.DeepEqual(got.Cards, wantCards) {
		t.Fatalf("cards = %+v, want %+v", got.Cards, wantCards)
	}
	f := got.Facts
	if f.ActiveDays != 3 || f.Minutes != 20 || f.Turns != 1 || f.AssignmentsOverdue != 1 ||
		len(f.Moments) != 1 || f.Moments[0].Quote != "雨落在屋檐上" || f.Moments[0].ItemTitle != "Test reading" ||
		!reflect.DeepEqual(f.NewKeywords, []string{"金融"}) {
		t.Fatalf("facts = %+v", f)
	}
}

func TestLiteWeeklyStudentProseStoredOnce(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(weeklyReply(weeklyValidStudentReply))
	h, pool, teacher, classID, studentID := liteTeacherFixtureWithProvider(t, prov)
	seedWeeklyStudentWeek(t, h, pool, teacher, classID, studentID)
	teacherID := userIDByEmail(t, pool, "lt-teacher@demo.local")
	path := studentWeeklyPath(classID, studentID) + "/prose"

	var first studentWeeklyJSON
	code, body := weeklyDo(t, h, teacher, "POST", path, &first)
	if code != http.StatusOK {
		t.Fatalf("POST = %d body=%s", code, body)
	}
	if first.Prose == nil || !strings.HasPrefix(first.Prose.Summary, "该周活跃 3 天") || !first.ProseReady || first.ProseError != nil {
		t.Fatalf("POST prose = %+v ready=%v err=%v", first.Prose, first.ProseReady, first.ProseError)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(body), &raw); err != nil || string(raw["proseError"]) != "null" {
		t.Fatalf("proseError must be present and null: %s", body)
	}
	if prov.Calls != 1 {
		t.Fatalf("provider calls = %d, want 1", prov.Calls)
	}
	if ids := llmCallUsers(t, pool, "lite_student_weekly"); len(ids) != 1 || ids[0] != teacherID {
		t.Fatalf("llm_call users = %v, want [teacher %s]", ids, teacherID)
	}

	var second studentWeeklyJSON
	if code, body := weeklyDo(t, h, teacher, "POST", path, &second); code != http.StatusOK {
		t.Fatalf("second POST = %d body=%s", code, body)
	}
	if prov.Calls != 1 {
		t.Fatalf("second POST called the provider: calls = %d", prov.Calls)
	}
	if !reflect.DeepEqual(first.Prose, second.Prose) || second.ProseError != nil {
		t.Fatalf("second prose = %+v, want %+v", second.Prose, first.Prose)
	}
	if n := len(llmCallUsers(t, pool, "lite_student_weekly")); n != 1 {
		t.Fatalf("llm_call rows after second POST = %d, want 1", n)
	}

	var got studentWeeklyJSON
	weeklyDo(t, h, teacher, "GET", studentWeeklyPath(classID, studentID), &got)
	if !got.ProseReady || !reflect.DeepEqual(got.Prose, first.Prose) {
		t.Fatalf("GET after POST prose = %+v ready=%v", got.Prose, got.ProseReady)
	}
}

func TestLiteWeeklyStudentProseRejectedNotStored(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(weeklyReply(weeklyFabricatedStudentReply))
	h, pool, teacher, classID, studentID := liteTeacherFixtureWithProvider(t, prov)
	seedWeeklyStudentWeek(t, h, pool, teacher, classID, studentID)

	var got studentWeeklyJSON
	if code, body := weeklyDo(t, h, teacher, "POST", studentWeeklyPath(classID, studentID)+"/prose", &got); code != http.StatusOK {
		t.Fatalf("POST = %d body=%s", code, body)
	}
	if got.Prose != nil || got.ProseReady || got.ProseError == nil || *got.ProseError == "" {
		t.Fatalf("POST prose = %+v ready=%v err=%v, want null prose and an error", got.Prose, got.ProseReady, got.ProseError)
	}
	if prov.Calls != 2 {
		t.Fatalf("provider calls = %d, want 2", prov.Calls)
	}
	if n := len(llmCallUsers(t, pool, "lite_student_weekly")); n != 2 {
		t.Fatalf("llm_call rows = %d, want 2", n)
	}
	if n := weeklyCount(t, pool, `SELECT count(*) FROM lite_student_weekly_prose WHERE user_id = $1`, studentID); n != 0 {
		t.Fatalf("stored prose rows = %d, want 0", n)
	}
	var after studentWeeklyJSON
	weeklyDo(t, h, teacher, "GET", studentWeeklyPath(classID, studentID), &after)
	if after.ProseReady || after.Prose != nil {
		t.Fatalf("GET after rejected POST prose = %+v", after.Prose)
	}
}

// TestLiteWeeklyStudentProseStalledCardNoSevenInLabel: the only 7 the prose
// may use is the one in the stalled card's evidence (超过 7 天没有进展).
func TestLiteWeeklyStudentProseStalledCardNoSevenInLabel(t *testing.T) {
	ws := liteweek.LatestCompleted(time.Now())
	for strings.Contains(liteweek.Label(ws), "7") {
		ws = ws.AddDate(0, 0, -7)
	}
	reply := `{"summary":"《雨水花园调查》超过 7 天没有进展。","suggestions":[{"text":"请她说明《雨水花园调查》超过 7 天没有进展的原因。","evidenceCode":"stalled"}]}`
	prov := gateway.NewSequenceStubProvider(weeklyReply(reply))
	h, pool, teacher, classID, studentID := liteTeacherFixtureWithProvider(t, prov)
	backdateWeeklyStart(t, pool, classID)

	reading := seedLiteReadingForUser(t, pool, studentID, "active", 0)
	seedBucket(t, pool, reading, ws.AddDate(0, 0, 2), 600)
	seedOldWriting(t, pool, studentID, "雨水花园调查", ws.AddDate(0, 0, -3))

	path := studentWeeklyPath(classID, studentID) + "/prose?weekStart=" + ws.In(liteweek.Beijing).Format("2006-01-02")
	var got studentWeeklyJSON
	if code, body := weeklyDo(t, h, teacher, "POST", path, &got); code != http.StatusOK {
		t.Fatalf("POST = %d body=%s", code, body)
	}
	want := []weeklyCardJSON{{Kind: "watch", Code: "stalled", Label: "进度停滞", Evidence: "《雨水花园调查》超过 7 天没有进展。"}}
	if !reflect.DeepEqual(got.Cards, want) {
		t.Fatalf("cards = %+v, want %+v", got.Cards, want)
	}
	if got.ProseError != nil || got.Prose == nil || prov.Calls != 1 {
		t.Fatalf("prose = %+v err=%v calls=%d, want accepted on the first attempt", got.Prose, got.ProseError, prov.Calls)
	}
}

func TestLiteWeeklyBadWeek(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(weeklyReply(weeklyValidStudentReply))
	h, _, teacher, classID, studentID := liteTeacherFixtureWithProvider(t, prov)
	current := liteweek.WeekStart(time.Now()).Format("2006-01-02")
	tuesday := liteweek.LatestCompleted(time.Now()).AddDate(0, 0, 1).Format("2006-01-02")

	for _, week := range []string{current, tuesday, "not-a-date"} {
		for _, tc := range []struct{ method, path string }{
			{"GET", studentWeeklyPath(classID, studentID)},
			{"POST", studentWeeklyPath(classID, studentID) + "/prose"},
			{"GET", classWeeklyPath(classID)},
			{"POST", classWeeklyPath(classID) + "/prose"},
		} {
			code, body := weeklyDo(t, h, teacher, tc.method, tc.path+"?weekStart="+week, nil)
			if code != http.StatusBadRequest || !strings.Contains(body, `"invalid_week"`) || !strings.Contains(body, "请选择已经结束的一周") {
				t.Fatalf("%s %s?weekStart=%s = %d %s, want 400 invalid_week", tc.method, tc.path, week, code, body)
			}
		}
	}
	if prov.Calls != 0 {
		t.Fatalf("provider calls = %d, want 0", prov.Calls)
	}
}

func TestLiteWeeklyAccess(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(weeklyReply(weeklyValidStudentReply))
	h, pool, _, classID, studentID := liteTeacherFixtureWithProvider(t, prov)
	other := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "lw-other-teacher@demo.local"))
	student := signInAs(t, pool, studentID)

	for _, tc := range []struct{ method, path string }{
		{"GET", studentWeeklyPath(classID, studentID)},
		{"POST", studentWeeklyPath(classID, studentID) + "/prose"},
		{"GET", classWeeklyPath(classID)},
		{"POST", classWeeklyPath(classID) + "/prose"},
	} {
		if code, body := weeklyDo(t, h, other, tc.method, tc.path, nil); code != http.StatusNotFound {
			t.Fatalf("other teacher %s %s = %d %s, want 404", tc.method, tc.path, code, body)
		}
		if code, body := weeklyDo(t, h, student, tc.method, tc.path, nil); code != http.StatusForbidden {
			t.Fatalf("student %s %s = %d %s, want 403", tc.method, tc.path, code, body)
		}
	}
	if prov.Calls != 0 {
		t.Fatalf("provider calls = %d, want 0", prov.Calls)
	}
}

func TestLiteWeeklyClassProseStoredOnce(t *testing.T) {
	// The reply must carry the student's id, which exists only after the
	// fixture has wired the provider in; the script is filled in afterwards.
	prov := gateway.NewSequenceStubProvider()
	h, pool, teacher, classID, studentID := liteTeacherFixtureWithProvider(t, prov)
	reply := `{"comment":"该周 1 名学生有 1 份作业逾期，读完《Test reading》。","cards":[{"userId":"` + studentID.String() +
		`","lead":"她读完《Test reading》，有 1 份作业逾期。","action":"请线下询问逾期作业卡在哪一步。"}]}`
	*prov = *gateway.NewSequenceStubProvider(weeklyReply(reply))

	seedWeeklyStudentWeek(t, h, pool, teacher, classID, studentID)
	ws, _ := weeklyWindow()
	teacherID := userIDByEmail(t, pool, "lt-teacher@demo.local")
	var name string
	if err := pool.QueryRow(context.Background(), `SELECT display_name FROM users WHERE id = $1`, studentID).Scan(&name); err != nil {
		t.Fatal(err)
	}

	var before classWeeklyJSON
	if code, body := weeklyDo(t, h, teacher, "GET", classWeeklyPath(classID), &before); code != http.StatusOK {
		t.Fatalf("GET = %d body=%s", code, body)
	}
	if prov.Calls != 0 || before.ProseReady || before.Prose != nil {
		t.Fatalf("GET calls=%d prose=%+v", prov.Calls, before.Prose)
	}
	label := liteweek.Label(ws)
	if before.Title != "上周班级周报 · "+label || !before.IsLatest || before.WeekLabel != label {
		t.Fatalf("title = %q latest=%v", before.Title, before.IsLatest)
	}
	st := before.Stats
	if st.ClassSize != 1 || st.ActiveStudents != 1 || st.Minutes != 20 || st.Turns != 1 || st.Finished != 1 || st.AssignmentRate != 0 {
		t.Fatalf("stats = %+v", st)
	}
	sid := studentID.String()
	wantWatch := []weeklyCardJSON{{Kind: "watch", Code: "overdue", Label: "作业逾期", Evidence: "该周到期的作业中有 1 份未完成。", UserID: sid, Name: name}}
	wantPraise := []weeklyCardJSON{{Kind: "praise", Code: "new_interest", Label: "新的兴趣", Evidence: "兴趣树新增关键词：金融。", UserID: sid, Name: name}}
	if !reflect.DeepEqual(before.Watch, wantWatch) || !reflect.DeepEqual(before.Praise, wantPraise) {
		t.Fatalf("watch = %+v praise = %+v", before.Watch, before.Praise)
	}

	path := classWeeklyPath(classID) + "/prose"
	var first classWeeklyJSON
	if code, body := weeklyDo(t, h, teacher, "POST", path, &first); code != http.StatusOK {
		t.Fatalf("POST = %d body=%s", code, body)
	}
	if first.Prose == nil || first.ProseError != nil || len(first.Prose.Cards) != 1 || first.Prose.Cards[0].UserID != sid {
		t.Fatalf("POST prose = %+v err=%v", first.Prose, first.ProseError)
	}
	if ids := llmCallUsers(t, pool, "lite_class_weekly"); len(ids) != 1 || ids[0] != teacherID {
		t.Fatalf("llm_call users = %v, want [teacher]", ids)
	}

	var second classWeeklyJSON
	weeklyDo(t, h, teacher, "POST", path, &second)
	if prov.Calls != 1 || !reflect.DeepEqual(first.Prose, second.Prose) {
		t.Fatalf("second POST calls=%d prose=%+v", prov.Calls, second.Prose)
	}
	if n := weeklyCount(t, pool, `SELECT count(*) FROM lite_class_weekly_prose WHERE class_id = $1`, uuid.MustParse(classID)); n != 1 {
		t.Fatalf("stored class prose rows = %d, want 1", n)
	}
	var after classWeeklyJSON
	weeklyDo(t, h, teacher, "GET", classWeeklyPath(classID), &after)
	if !after.ProseReady || !reflect.DeepEqual(after.Prose, first.Prose) {
		t.Fatalf("GET after POST prose = %+v", after.Prose)
	}
}

func TestLiteWeeklyClassAssignmentRate(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	backdateWeeklyStart(t, pool, classID)
	student := signInAs(t, pool, studentID)
	ws, we := weeklyWindow()

	var got classWeeklyJSON
	weeklyDo(t, h, teacher, "GET", classWeeklyPath(classID), &got)
	if got.Stats.AssignmentRate != -1 {
		t.Fatalf("assignmentRate with nothing due = %d, want -1", got.Stats.AssignmentRate)
	}

	done := createAssignment(t, h, teacher, classID, writingAssignmentBody([]string{studentID.String()}))
	started := startAssignment(t, h, student, done)
	mustExec(t, pool, `UPDATE lite_assignment SET due_at = $2 WHERE id = $1`, done, ws.AddDate(0, 0, 5))
	mustExec(t, pool, `UPDATE writing SET status = 'finished', finished_at = $2 WHERE atom_id = $1`, started.AtomID, ws.AddDate(0, 0, 2))

	overdue := createAssignment(t, h, teacher, classID, writingAssignmentBody([]string{studentID.String()}))
	startAssignment(t, h, student, overdue)
	mustExec(t, pool, `UPDATE lite_assignment SET due_at = $2 WHERE id = $1`, overdue, we.AddDate(0, 0, -3))

	got = classWeeklyJSON{}
	weeklyDo(t, h, teacher, "GET", classWeeklyPath(classID), &got)
	if got.Stats.AssignmentRate != 50 {
		t.Fatalf("assignmentRate with one done and one overdue = %d, want 50", got.Stats.AssignmentRate)
	}
}

// backdateWeeklyStart moves the class's creation and every enrollment in it
// 90 days back. liteTeacherFixture creates both now, so without it every
// completed week is before the reporting starts and returns week_before_start.
func backdateWeeklyStart(t *testing.T, pool *pgxpool.Pool, classID string) {
	t.Helper()
	mustExec(t, pool, `UPDATE classes SET created_at = now() - interval '90 days' WHERE id = $1`, classID)
	mustExec(t, pool, `UPDATE enrollments SET created_at = now() - interval '90 days' WHERE class_id = $1`, classID)
}

// setWeeklyStart sets the class's creation and every enrollment in it to at.
func setWeeklyStart(t *testing.T, pool *pgxpool.Pool, classID string, at time.Time) {
	t.Helper()
	mustExec(t, pool, `UPDATE classes SET created_at = $2 WHERE id = $1`, classID, at)
	mustExec(t, pool, `UPDATE enrollments SET created_at = $2 WHERE class_id = $1`, classID, at)
}

// rawWeeklyFields decodes the top-level fields of a response body as raw JSON.
func rawWeeklyFields(t *testing.T, body string) map[string]json.RawMessage {
	t.Helper()
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(body), &raw); err != nil {
		t.Fatalf("decode %s: %v", body, err)
	}
	return raw
}

// TestLiteWeeklyWeekBeforeStart: a week that ended at or before her
// enrollment (student routes) or the class's creation (class routes) is a
// 400 on GET and POST alike, and hasPrev says whether the previous week is
// still in range.
func TestLiteWeeklyWeekBeforeStart(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(weeklyReply(weeklyValidStudentReply))
	h, pool, teacher, classID, studentID := liteTeacherFixtureWithProvider(t, prov)
	ws, _ := weeklyWindow()
	weekQ := func(start time.Time) string {
		return "?weekStart=" + start.In(liteweek.Beijing).Format("2006-01-02")
	}
	type route struct{ method, path, message string }
	studentRoutes := []route{
		{"GET", studentWeeklyPath(classID, studentID), "该周早于学生加入班级的时间"},
		{"POST", studentWeeklyPath(classID, studentID) + "/prose", "该周早于学生加入班级的时间"},
	}
	classRoutes := []route{
		{"GET", classWeeklyPath(classID), "该周早于班级创建的时间"},
		{"POST", classWeeklyPath(classID) + "/prose", "该周早于班级创建的时间"},
	}
	wantBeforeStart := func(rt route, q string) {
		t.Helper()
		code, body := weeklyDo(t, h, teacher, rt.method, rt.path+q, nil)
		if code != http.StatusBadRequest || !strings.Contains(body, `"week_before_start"`) || !strings.Contains(body, rt.message) {
			t.Fatalf("%s %s%s = %d %s, want 400 week_before_start %s", rt.method, rt.path, q, code, body, rt.message)
		}
	}

	// The fixture creates the class and enrolls her now: the latest completed
	// week ended before both.
	for _, rt := range append(append([]route{}, studentRoutes...), classRoutes...) {
		wantBeforeStart(rt, "")
		wantBeforeStart(rt, weekQ(ws))
	}

	// Class created and her enrollment exactly at the latest week's start: that
	// week is in range, and the week before it ended at that moment, so it is not.
	setWeeklyStart(t, pool, classID, ws)
	var s studentWeeklyJSON
	if code, body := weeklyDo(t, h, teacher, "GET", studentWeeklyPath(classID, studentID), &s); code != http.StatusOK || s.HasPrev {
		t.Fatalf("student GET = %d hasPrev=%v %s, want 200 and no previous week", code, s.HasPrev, body)
	}
	var c classWeeklyJSON
	if code, body := weeklyDo(t, h, teacher, "GET", classWeeklyPath(classID), &c); code != http.StatusOK || c.HasPrev {
		t.Fatalf("class GET = %d hasPrev=%v %s, want 200 and no previous week", code, c.HasPrev, body)
	}
	for _, rt := range append(append([]route{}, studentRoutes...), classRoutes...) {
		wantBeforeStart(rt, weekQ(ws.AddDate(0, 0, -7)))
	}

	// One second earlier, the previous week is in range.
	setWeeklyStart(t, pool, classID, ws.Add(-time.Second))
	s, c = studentWeeklyJSON{}, classWeeklyJSON{}
	if code, body := weeklyDo(t, h, teacher, "GET", studentWeeklyPath(classID, studentID), &s); code != http.StatusOK || !s.HasPrev {
		t.Fatalf("student GET = %d hasPrev=%v %s, want 200 with a previous week", code, s.HasPrev, body)
	}
	if code, body := weeklyDo(t, h, teacher, "GET", classWeeklyPath(classID), &c); code != http.StatusOK || !c.HasPrev {
		t.Fatalf("class GET = %d hasPrev=%v %s, want 200 with a previous week", code, c.HasPrev, body)
	}
	s = studentWeeklyJSON{}
	if code, body := weeklyDo(t, h, teacher, "GET", studentWeeklyPath(classID, studentID)+weekQ(ws.AddDate(0, 0, -7)), &s); code != http.StatusOK || s.HasPrev {
		t.Fatalf("student GET previous week = %d hasPrev=%v %s, want 200 and no week before it", code, s.HasPrev, body)
	}
	if prov.Calls != 0 {
		t.Fatalf("provider calls = %d, want 0", prov.Calls)
	}
}

// TestLiteWeeklyEmptyWeekSpendsNothing: a week with nothing in it is marked
// empty, and its POSTs return prose null and proseError null with no
// entitlement check, no model call, no llm_call row and nothing stored.
func TestLiteWeeklyEmptyWeekSpendsNothing(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(weeklyReply(weeklyValidStudentReply))
	h, pool, teacher, classID, studentID := liteTeacherFixtureWithProvider(t, prov)
	backdateWeeklyStart(t, pool, classID)

	var sg studentWeeklyJSON
	if code, body := weeklyDo(t, h, teacher, "GET", studentWeeklyPath(classID, studentID), &sg); code != http.StatusOK || !sg.Empty {
		t.Fatalf("student GET = %d empty=%v %s, want an empty week", code, sg.Empty, body)
	}
	// never_used still fires on an empty week; the cards are sent as they are.
	if len(sg.Cards) != 1 || sg.Cards[0].Code != "never_used" || sg.Cards[0].Label != "未使用" {
		t.Fatalf("student cards = %+v, want never_used", sg.Cards)
	}
	var cg classWeeklyJSON
	if code, body := weeklyDo(t, h, teacher, "GET", classWeeklyPath(classID), &cg); code != http.StatusOK || !cg.Empty {
		t.Fatalf("class GET = %d empty=%v %s, want an empty week", code, cg.Empty, body)
	}

	for _, path := range []string{studentWeeklyPath(classID, studentID) + "/prose", classWeeklyPath(classID) + "/prose"} {
		code, body := weeklyDo(t, h, teacher, "POST", path, nil)
		if code != http.StatusOK {
			t.Fatalf("POST %s = %d %s", path, code, body)
		}
		raw := rawWeeklyFields(t, body)
		if string(raw["prose"]) != "null" || string(raw["proseError"]) != "null" || string(raw["empty"]) != "true" || string(raw["proseReady"]) != "false" {
			t.Fatalf("POST %s = %s, want prose null, proseError null, empty true", path, body)
		}
	}
	if prov.Calls != 0 {
		t.Fatalf("provider calls = %d, want 0", prov.Calls)
	}
	for _, purpose := range []string{"lite_student_weekly", "lite_class_weekly"} {
		if n := len(llmCallUsers(t, pool, purpose)); n != 0 {
			t.Fatalf("llm_call rows for %s = %d, want 0", purpose, n)
		}
	}
	if n := weeklyCount(t, pool, `SELECT count(*) FROM lite_student_weekly_prose WHERE user_id = $1`, studentID); n != 0 {
		t.Fatalf("stored student prose rows = %d, want 0", n)
	}
	if n := weeklyCount(t, pool, `SELECT count(*) FROM lite_class_weekly_prose WHERE class_id = $1`, uuid.MustParse(classID)); n != 0 {
		t.Fatalf("stored class prose rows = %d, want 0", n)
	}

	// One overdue assignment makes the week non-empty for her and for the class.
	aid := createAssignment(t, h, teacher, classID, writingAssignmentBody([]string{studentID.String()}))
	ws, _ := weeklyWindow()
	mustExec(t, pool, `UPDATE lite_assignment SET due_at = $2 WHERE id = $1`, aid, ws.AddDate(0, 0, 3))
	sg, cg = studentWeeklyJSON{}, classWeeklyJSON{}
	weeklyDo(t, h, teacher, "GET", studentWeeklyPath(classID, studentID), &sg)
	weeklyDo(t, h, teacher, "GET", classWeeklyPath(classID), &cg)
	if sg.Empty || cg.Empty {
		t.Fatalf("with an overdue assignment: student empty=%v class empty=%v, want both false", sg.Empty, cg.Empty)
	}
}

func TestLiteWeeklyPastWeekTitles(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	backdateWeeklyStart(t, pool, classID)
	past := liteweek.LatestCompleted(time.Now()).AddDate(0, 0, -7)
	q := "?weekStart=" + past.In(liteweek.Beijing).Format("2006-01-02")
	label := liteweek.Label(past)

	var s studentWeeklyJSON
	if code, body := weeklyDo(t, h, teacher, "GET", studentWeeklyPath(classID, studentID)+q, &s); code != http.StatusOK {
		t.Fatalf("student GET = %d %s", code, body)
	}
	if s.Title != "表现总结 · "+label || s.IsLatest || s.WeekStart != past.In(liteweek.Beijing).Format("2006-01-02") {
		t.Fatalf("student week = %q %q latest=%v", s.WeekStart, s.Title, s.IsLatest)
	}
	var c classWeeklyJSON
	if code, body := weeklyDo(t, h, teacher, "GET", classWeeklyPath(classID)+q, &c); code != http.StatusOK {
		t.Fatalf("class GET = %d %s", code, body)
	}
	if c.Title != "班级周报 · "+label || c.IsLatest {
		t.Fatalf("class week = %q latest=%v", c.Title, c.IsLatest)
	}
}
