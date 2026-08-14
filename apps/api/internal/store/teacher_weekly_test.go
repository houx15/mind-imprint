package store_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"mindimprint/api/internal/store/sqlc"
)

// TestClassWeeklyProseFirstWriteWins guards DEC-2: class_weekly_prose's
// primary key (class_id, week_start) IS the first-open-wins lock.
// InsertClassWeeklyProse is ON CONFLICT DO NOTHING, so a concurrent second
// generation call is silently discarded and the first writer's prose stands.
func TestClassWeeklyProseFirstWriteWins(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	pool := newStoreTestPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	classID := createClassRow(t, pool)
	week := pgtype.Date{Time: time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC), Valid: true}

	first := sqlc.InsertClassWeeklyProseParams{
		ClassID: classID, WeekStart: week,
		Comment: "第一次写的点评", DepthNote: "d1", AutonomyNote: "a1",
		Cards: []byte(`[{"userId":"u1","lead":"l1","action":"act1"}]`),
	}
	if err := q.InsertClassWeeklyProse(ctx, first); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	second := first
	second.Comment = "第二次写的点评"
	if err := q.InsertClassWeeklyProse(ctx, second); err != nil {
		t.Fatalf("second insert must not error: %v", err)
	}

	got, err := q.GetClassWeeklyProse(ctx, sqlc.GetClassWeeklyProseParams{ClassID: classID, WeekStart: week})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Comment != "第一次写的点评" {
		t.Fatalf("comment = %q; want the FIRST write — first-open-wins", got.Comment)
	}
}

// TestAppendClassWeeklyProseCardsOnlyAppends guards DEC-6: the top-up writer
// may only append to cards. It must never rewrite comment/depth_note/autonomy_note
// on an existing row.
func TestAppendClassWeeklyProseCardsOnlyAppends(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	pool := newStoreTestPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	classID := createClassRow(t, pool)
	week := pgtype.Date{Time: time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC), Valid: true}

	if err := q.InsertClassWeeklyProse(ctx, sqlc.InsertClassWeeklyProseParams{
		ClassID: classID, WeekStart: week,
		Comment: "点评", DepthNote: "d", AutonomyNote: "a",
		Cards: []byte(`[{"userId":"u1","lead":"l1","action":"act1"}]`),
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := q.AppendClassWeeklyProseCards(ctx, sqlc.AppendClassWeeklyProseCardsParams{
		ClassID: classID, WeekStart: week,
		Cards: []byte(`[{"userId":"u2","lead":"l2","action":"act2"}]`),
	}); err != nil {
		t.Fatalf("append: %v", err)
	}

	got, err := q.GetClassWeeklyProse(ctx, sqlc.GetClassWeeklyProseParams{ClassID: classID, WeekStart: week})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Comment != "点评" {
		t.Fatalf("comment = %q; the top-up must never rewrite it", got.Comment)
	}
	s := string(got.Cards)
	if !strings.Contains(s, `"u1"`) || !strings.Contains(s, `"u2"`) {
		t.Fatalf("cards = %s; want both the original and the appended entry", s)
	}
}

// createClassRow seeds a minimal school + class row. No equivalent helper
// exists in the store_test (external) package — teacher_query_test.go's
// class-seeding inline SQL lives in the internal `store` package test file
// and is not reusable from here.
func createClassRow(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var schoolID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO schools (name) VALUES ('D2 Weekly Prose Test School') RETURNING id`).
		Scan(&schoolID); err != nil {
		t.Fatalf("seed school: %v", err)
	}
	var classID uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO classes (school_id, name, join_code) VALUES ($1, 'D2 Weekly Prose Class', $2) RETURNING id`,
		schoolID, uuid.NewString()).Scan(&classID); err != nil {
		t.Fatalf("seed class: %v", err)
	}
	return classID
}

// createStudentRow seeds a minimal school + student user. This mirrors
// teacher_query_test.go's helper of the same name, which lives in the
// internal `store` package and is therefore unexported and unreachable from
// this external `store_test` package — hence the local duplicate.
func createStudentRow(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var schoolID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO schools (name) VALUES ('D2 Weekly Stats Test School') RETURNING id`).
		Scan(&schoolID); err != nil {
		t.Fatalf("seed school: %v", err)
	}
	var studentID uuid.UUID
	email := fmt.Sprintf("d2-weekly-student-%s@example.com", uuid.NewString())
	if err := pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, role, school_id, display_name, avatar_color)
		VALUES ($1, 'x', 'student', $2, 'D2 Weekly Student', '#777777')
		RETURNING id`, email, schoolID).Scan(&studentID); err != nil {
		t.Fatalf("seed student: %v", err)
	}
	return studentID
}

// enrollStudentRow enrolls an existing student user into an existing class as
// a student. No equivalent helper exists in this package; teacher_query_test.go
// enrolls inline via mustExec, which is unexported and unreachable here.
func enrollStudentRow(t *testing.T, pool *pgxpool.Pool, student, classID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `
		INSERT INTO enrollments (user_id, class_id, role_in_class) VALUES ($1, $2, 'student')`,
		student, classID); err != nil {
		t.Fatalf("enroll student: %v", err)
	}
}

// insertEventAt seeds one event row at an explicit timestamp, scoped to a
// fresh project so it satisfies event's event_scope_ck (>=1 of
// project_id/session_id/thread_id/course_id non-null). Surface is fixed to
// 'studio' — GetClassWeekStats and ListClassStudentWeekActivity filter on
// `type`, never `surface`, so the scope choice has no bearing on their counts.
func insertEventAt(t *testing.T, pool *pgxpool.Pool, student uuid.UUID, eventType string, createdAt time.Time) {
	t.Helper()
	ctx := context.Background()
	var projectID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO project (user_id, title) VALUES ($1, 'D2 weekly stats test project') RETURNING id`,
		student).Scan(&projectID); err != nil {
		t.Fatalf("seed project for event: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO event (user_id, project_id, surface, type, created_at) VALUES ($1, $2, 'studio', $3, $4)`,
		student, projectID, eventType, createdAt); err != nil {
		t.Fatalf("insert event: %v", err)
	}
}

// insertReadyEvaluationReport seeds a fresh project owned by student and one
// 'ready' evaluation_report row on it, with an explicit created_at so
// this-week/prior ordering is deterministic in tests. evaluation_report has a
// UNIQUE(project_id) constraint, so each report needs its own project — this
// mirrors production, where "how many reports this week" really means "how
// many projects finished this week".
func insertReadyEvaluationReport(t *testing.T, pool *pgxpool.Pool, student uuid.UUID, createdAt time.Time) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var projectID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO project (user_id, title) VALUES ($1, 'D2 weekly activity test project') RETURNING id`,
		student).Scan(&projectID); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO evaluation_report (project_id, status, created_at) VALUES ($1, 'ready', $2)`,
		projectID, createdAt); err != nil {
		t.Fatalf("insert evaluation_report: %v", err)
	}
	return projectID
}

// TestGetClassWeekStatsCountsOnlyTheWindow guards the half-open window bound
// on GetClassWeekStats, the D1-verbatim 对话轮次 口径 (prompt_sent +
// course_message only — card_surfaced is real activity but not a "turn"),
// and the reports count now reading ready evaluation_report rows (not the
// retired student_evaluation view).
func TestGetClassWeekStatsCountsOnlyTheWindow(t *testing.T) {
	pool := newStoreTestPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	classID := createClassRow(t, pool)
	student := createStudentRow(t, pool)
	enrollStudentRow(t, pool, student, classID)

	week := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	end := week.AddDate(0, 0, 7)
	insertEventAt(t, pool, student, "prompt_sent", week.Add(2*time.Hour))     // in
	insertEventAt(t, pool, student, "course_message", week.Add(30*time.Hour)) // in
	insertEventAt(t, pool, student, "card_surfaced", week.Add(3*time.Hour))   // in, not a turn
	insertEventAt(t, pool, student, "prompt_sent", week.Add(-1*time.Hour))    // before the window
	insertReadyEvaluationReport(t, pool, student, week.Add(4*time.Hour))      // in
	insertReadyEvaluationReport(t, pool, student, week.Add(-2*time.Hour))     // before the window

	got, err := q.GetClassWeekStats(ctx, sqlc.GetClassWeekStatsParams{
		ClassID: classID, WeekStart: week, WeekEnd: end,
	})
	if err != nil {
		t.Fatalf("GetClassWeekStats: %v", err)
	}
	if got.Turns != 2 {
		t.Fatalf("turns = %d; want 2 (prompt_sent + course_message inside the window only)", got.Turns)
	}
	if got.ActiveStudents != 1 {
		t.Fatalf("activeStudents = %d; want 1", got.ActiveStudents)
	}
	if got.ClassSize != 1 {
		t.Fatalf("classSize = %d; want 1", got.ClassSize)
	}
	if got.Reports != 1 {
		t.Fatalf("reports = %d; want 1 (only the in-window ready evaluation_report)", got.Reports)
	}
}

// TestListClassStudentWeekActivityReportCounts guards
// ListClassStudentWeekActivity's report bookkeeping: reports_this_week and
// prior_reports are counted from ready evaluation_report rows split at
// week_start, and latest_report_project_id names the newest one. It also
// exercises the case that motivated restructuring the query away from a
// LATERAL "ON true" join (see the query's own comment): a student with NO
// ready report must scan latest_report_project_id as a NULL pgtype.UUID, not
// error the whole row out.
func TestListClassStudentWeekActivityReportCounts(t *testing.T) {
	pool := newStoreTestPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	classID := createClassRow(t, pool)
	reported := createStudentRow(t, pool)
	enrollStudentRow(t, pool, reported, classID)
	quiet := createStudentRow(t, pool)
	enrollStudentRow(t, pool, quiet, classID)

	week := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	end := week.AddDate(0, 0, 7)
	prevStart := week.AddDate(0, 0, -7)
	prevEnd := week

	insertReadyEvaluationReport(t, pool, reported, week.Add(-48*time.Hour))         // prior week
	latest := insertReadyEvaluationReport(t, pool, reported, week.Add(2*time.Hour)) // this week — the latest

	act, err := q.ListClassStudentWeekActivity(ctx, sqlc.ListClassStudentWeekActivityParams{
		ClassID: classID, WeekStart: week, WeekEnd: end, PrevStart: prevStart, PrevEnd: prevEnd,
	})
	if err != nil {
		t.Fatalf("ListClassStudentWeekActivity: %v", err)
	}
	if len(act) != 2 {
		t.Fatalf("rows = %d; want 2 (both enrolled students)", len(act))
	}

	byUser := map[uuid.UUID]sqlc.ListClassStudentWeekActivityRow{}
	for _, row := range act {
		byUser[row.UserID] = row
	}

	r := byUser[reported]
	if r.ReportsThisWeek != 1 {
		t.Fatalf("reported.ReportsThisWeek = %d; want 1", r.ReportsThisWeek)
	}
	if r.PriorReports != 1 {
		t.Fatalf("reported.PriorReports = %d; want 1", r.PriorReports)
	}
	if !r.LatestReportProjectID.Valid {
		t.Fatal("reported.LatestReportProjectID.Valid = false; want the newest ready report's project id")
	}
	if got := uuid.UUID(r.LatestReportProjectID.Bytes); got != latest {
		t.Fatalf("reported.LatestReportProjectID = %v; want the newest report's project %v", got, latest)
	}

	q2 := byUser[quiet]
	if q2.ReportsThisWeek != 0 || q2.PriorReports != 0 {
		t.Fatalf("quiet student's report counts = %+v; want both 0", q2)
	}
	if q2.LatestReportProjectID.Valid {
		t.Fatalf("quiet.LatestReportProjectID.Valid = true; a student with no ready report must scan as NULL, not error or fabricate an id")
	}
}
