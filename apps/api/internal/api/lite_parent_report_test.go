package api_test

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/disciplines"
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
	seedReport(t, pool, lateAtom, "writing", `{"moments":[`+
		`{"quote":"写一篇关于雨的记叙文","where":""},`+
		`{"quote":"一篇关于雨的记叙文","where":""},`+
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
