package api_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	. "mindimprint/api/internal/api"
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
	seedOldReading(t, pool, studentID, "本周创建", ws.AddDate(0, 0, 1)) // excluded
	readLate := seedOldReading(t, pool, studentID, "周末后读完", before)
	mustExec(t, pool, `UPDATE reading SET status = 'finished', finished_at = $2 WHERE atom_id = $1`, readLate, we.Add(2*time.Hour)) // stalled

	after := seedOldWriting(t, pool, studentID, "周末后才动", before) // stalled
	seedBucket(t, pool, after, we, 600)
	seedAtomMessageAt(t, pool, after, we.Add(time.Hour))
	mustExec(t, pool, `UPDATE atom SET last_activity_at = now() WHERE id = $1`, after)

	msg := seedOldWriting(t, pool, studentID, "本周有消息", before) // excluded
	seedAtomMessageAt(t, pool, msg, we.Add(-time.Second))
	bucket := seedOldWriting(t, pool, studentID, "本周有时长", before) // excluded
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
