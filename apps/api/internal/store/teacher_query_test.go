package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"mindimprint/api/internal/store/sqlc"
)

// TestTeacherReadPathQueries seeds a school with two classes (a teacher owns
// class A; student A is enrolled in class A, student B in a DIFFERENT class
// B) and asserts every remaining teacher.sql query is properly class-scoped:
// class B's student never leaks into class A's roster/report reads. (The old
// per-student-report queries this test used to also exercise —
// GetStudentProjectEvaluationForTeacher and its thread sibling, and later
// ListStudentReportsForTeacher/GetLatestReportScoresForStudent — were retired
// 2026-08-14 along with the whole D/A-axis teacher read-path; the axis-era
// `evaluations` table is no longer written or read by any query this test
// exercises.)
func TestTeacherReadPathQueries(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTestPool(t)
	q := sqlc.New(pool)

	// --- School + two classes -------------------------------------------------
	var schoolID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO schools (name) VALUES ('D1 Test School') RETURNING id`).
		Scan(&schoolID); err != nil {
		t.Fatalf("seed school: %v", err)
	}

	var classA, classB uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO classes (school_id, name, join_code) VALUES ($1, 'Class A', 'D1-A') RETURNING id`,
		schoolID).Scan(&classA); err != nil {
		t.Fatalf("seed class A: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO classes (school_id, name, join_code) VALUES ($1, 'Class B', 'D1-B') RETURNING id`,
		schoolID).Scan(&classB); err != nil {
		t.Fatalf("seed class B: %v", err)
	}

	// --- Users: one teacher (class A), student A (class A), student B (class B) -
	var teacherID, studentA, studentB uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, role, school_id, display_name, avatar_color)
		VALUES ('d1-teacher@example.com', 'x', 'teacher', $1, 'Ms. Teacher', '#111111')
		RETURNING id`, schoolID).Scan(&teacherID); err != nil {
		t.Fatalf("seed teacher: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, role, school_id, display_name, avatar_color)
		VALUES ('d1-student-a@example.com', 'x', 'student', $1, 'Student A', '#222222')
		RETURNING id`, schoolID).Scan(&studentA); err != nil {
		t.Fatalf("seed student A: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, role, school_id, display_name, avatar_color)
		VALUES ('d1-student-b@example.com', 'x', 'student', $1, 'Student B', '#333333')
		RETURNING id`, schoolID).Scan(&studentB); err != nil {
		t.Fatalf("seed student B: %v", err)
	}

	mustExec(t, ctx, pool, `INSERT INTO enrollments (user_id, class_id, role_in_class) VALUES ($1, $2, 'teacher')`, teacherID, classA)
	mustExec(t, ctx, pool, `INSERT INTO enrollments (user_id, class_id, role_in_class) VALUES ($1, $2, 'student')`, studentA, classA)
	mustExec(t, ctx, pool, `INSERT INTO enrollments (user_id, class_id, role_in_class) VALUES ($1, $2, 'student')`, studentB, classB)

	// --- Student A: one project -------------------------------------------------
	var projectID uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO project (user_id, title) VALUES ($1, '学生A的项目') RETURNING id`,
		studentA).Scan(&projectID); err != nil {
		t.Fatalf("seed project: %v", err)
	}

	// --- Week window + events ---------------------------------------------------
	weekStart := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	weekEnd := time.Date(2026, 1, 12, 0, 0, 0, 0, time.UTC)

	// event_scope_ck requires num_nonnulls(project_id, session_id, thread_id) >= 1;
	// every one of student A's events is scoped to her one project.
	// Day 1: one prompt_sent (turn).
	insertEvent(t, ctx, pool, studentA, projectID, "studio", "prompt_sent", time.Date(2026, 1, 6, 10, 0, 0, 0, time.UTC))
	// Day 2: a course_message and a prompt_sent (two turns, one active day).
	insertEvent(t, ctx, pool, studentA, projectID, "course", "course_message", time.Date(2026, 1, 7, 11, 0, 0, 0, time.UTC))
	insertEvent(t, ctx, pool, studentA, projectID, "studio", "prompt_sent", time.Date(2026, 1, 7, 12, 0, 0, 0, time.UTC))
	// Day 3: a non-turn event type — counts toward active_days, not turns.
	insertEvent(t, ctx, pool, studentA, projectID, "studio", "card_opened", time.Date(2026, 1, 8, 9, 0, 0, 0, time.UTC))
	// Out of window: before weekStart.
	insertEvent(t, ctx, pool, studentA, projectID, "studio", "prompt_sent", time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC))
	// Out of window: exactly at weekEnd (half-open, excluded).
	insertEvent(t, ctx, pool, studentA, projectID, "studio", "prompt_sent", weekEnd)

	wantActiveDays := int32(3)
	wantTurns := int32(3)

	// Student A also gets a ready evaluation_report on her project, so
	// ListClassRosterCounts' report_count lateral (and ListStudentProjectsForTeacher's
	// has_report EXISTS) have something to count.
	if _, err := q.ClaimEvaluationReportGeneration(ctx, projectID); err != nil {
		t.Fatalf("claim evaluation report: %v", err)
	}
	if err := q.CompleteEvaluationReport(ctx, sqlc.CompleteEvaluationReportParams{
		ProjectID: projectID, Report: []byte(`{"version":1}`),
	}); err != nil {
		t.Fatalf("complete evaluation report: %v", err)
	}

	// --- ListClassRosterCounts: only student A in class A, never student B -----
	roster, err := q.ListClassRosterCounts(ctx, classA)
	if err != nil {
		t.Fatalf("ListClassRosterCounts: %v", err)
	}
	if len(roster) != 1 {
		t.Fatalf("roster has %d rows, want 1 (student A only, student B in a different class excluded)", len(roster))
	}
	row := roster[0]
	if row.ID != studentA {
		t.Fatalf("roster row id = %v, want student A", row.ID)
	}
	if row.ActiveProjects != 1 {
		t.Errorf("active_projects = %d, want 1", row.ActiveProjects)
	}
	if row.ReportCount != 1 {
		t.Errorf("report_count = %d, want 1", row.ReportCount)
	}
	if row.CoursesFinished != 0 {
		t.Errorf("courses_finished = %d, want 0 (none seeded)", row.CoursesFinished)
	}

	// --- GetStudentUsageForTeacher matches the roster's own count ---------------
	usage, err := q.GetStudentUsageForTeacher(ctx, sqlc.GetStudentUsageForTeacherParams{
		UserID: studentA, WeekStart: weekStart, WeekEnd: weekEnd,
	})
	if err != nil {
		t.Fatalf("GetStudentUsageForTeacher: %v", err)
	}
	if int32(usage.ActiveDays) != wantActiveDays || int32(usage.Turns) != wantTurns {
		t.Errorf("usage = %+v, want active_days=%d turns=%d", usage, wantActiveDays, wantTurns)
	}

	// --- ListStudentProjectsForTeacher: the project appears, has_report=true ---
	// (has_report now reads the ready evaluation_report row claimed/completed
	// above, not the retired `evaluations` table.)
	projects, err := q.ListStudentProjectsForTeacher(ctx, studentA)
	if err != nil {
		t.Fatalf("ListStudentProjectsForTeacher: %v", err)
	}
	if len(projects) != 1 || !projects[0].HasReport {
		t.Fatalf("projects = %+v, want one row with has_report=true", projects)
	}

	// studentB never touched a project — ListStudentProjectsForTeacher must
	// come back empty (also proves the query is scoped to user_id, not leaked
	// across students).
	studentBProjects, err := q.ListStudentProjectsForTeacher(ctx, studentB)
	if err != nil {
		t.Fatalf("ListStudentProjectsForTeacher(B): %v", err)
	}
	if len(studentBProjects) != 0 {
		t.Fatalf("studentB projects = %+v, want none", studentBProjects)
	}
}

// TestTeacherReadPathSeedData asserts migration 0029's demo data (吴老师's
// IBDP 一年级 · 研究组, 9 students) is shaped the way the teacher read-path
// endpoints need: exactly 9 roster rows via ListClassRosterCounts. (The old
// has_report=4 assertion this test used to make is retired — it read the OLD
// `evaluations` pipeline migrations 0029/0034 seed into, but
// ListClassRosterCounts' report_count now reads the NEW evaluation_report
// table, which this seed data never populates, so every seeded student's
// report_count is legitimately 0; TestRosterReportHappyPath in
// internal/api/teacher_read_test.go covers report_count=1 with a freshly
// seeded evaluation_report row instead. The 林知远 rich-report-shape
// assertion this test used to also make, via the now-retired
// GetLatestReportScoresForStudent, has no replacement — the teacher path no
// longer reads per-student report score shapes at all.)
func TestTeacherReadPathSeedData(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTestPool(t)
	q := sqlc.New(pool)

	seededClass := uuid.MustParse("00000000-0000-0000-0000-000000000902")

	roster, err := q.ListClassRosterCounts(ctx, seededClass)
	if err != nil {
		t.Fatalf("ListClassRosterCounts(seeded class): %v", err)
	}
	if len(roster) != 9 {
		t.Fatalf("seeded roster has %d rows, want 9", len(roster))
	}
}

// TestActiveDaysPinnedToUTCAcrossSessionTimeZone guards FIX 2 of the
// whole-branch review: teacher.sql's active-days bucketing must be pinned to
// UTC via `AT TIME ZONE 'UTC'`, not the DB session's TimeZone GUC. It seeds
// one event per hour across the entire UTC week window and runs
// GetStudentUsageForTeacher (student-detail head's active-days query — the
// roster's own active_days/turns fields were retired along with
// ListClassRosterReport, see ListClassRosterCounts; the roster's replacement
// windowed count, GetClassLiveHeader's active_students, does a plain
// timestamptz range comparison with no `::date` cast, so it has no
// session-timezone exposure to guard here) over a connection whose session
// TimeZone is forced to Asia/Shanghai (UTC+8) — under the pre-fix
// `(created_at)::date` cast, that offset shifts the early-morning UTC hours
// into the FOLLOWING local calendar day, so a 7×24h UTC window reads as 8
// distinct active days. The fix must hold at exactly 7 regardless of the
// session's timezone.
func TestActiveDaysPinnedToUTCAcrossSessionTimeZone(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTestPool(t)

	var schoolID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO schools (name) VALUES ('D1 TZ Test School') RETURNING id`).
		Scan(&schoolID); err != nil {
		t.Fatalf("seed school: %v", err)
	}
	var classID uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO classes (school_id, name, join_code) VALUES ($1, 'TZ Class', 'D1-TZ') RETURNING id`,
		schoolID).Scan(&classID); err != nil {
		t.Fatalf("seed class: %v", err)
	}
	var studentID uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, role, school_id, display_name, avatar_color)
		VALUES ('tz-student@example.com', 'x', 'student', $1, 'TZ Student', '#444444')
		RETURNING id`, schoolID).Scan(&studentID); err != nil {
		t.Fatalf("seed student: %v", err)
	}
	mustExec(t, ctx, pool, `INSERT INTO enrollments (user_id, class_id, role_in_class) VALUES ($1, $2, 'student')`, studentID, classID)

	var projectID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO project (user_id, title) VALUES ($1, 'tz project') RETURNING id`,
		studentID).Scan(&projectID); err != nil {
		t.Fatalf("seed project: %v", err)
	}

	weekStart := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC) // a Monday, UTC midnight
	weekEnd := weekStart.AddDate(0, 0, 7)
	for h := 0; h < 24*7; h++ {
		insertEvent(t, ctx, pool, studentID, projectID, "studio", "prompt_sent", weekStart.Add(time.Duration(h)*time.Hour))
	}

	// Pin ONE physical connection's session TimeZone to Asia/Shanghai (UTC+8)
	// and run the queries over that exact connection — pgxpool would
	// otherwise silently hand back a connection whose session GUC never
	// changed.
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire conn: %v", err)
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, `SET TIME ZONE 'Asia/Shanghai'`); err != nil {
		t.Fatalf("set session time zone: %v", err)
	}
	q := sqlc.New(conn)

	usage, err := q.GetStudentUsageForTeacher(ctx, sqlc.GetStudentUsageForTeacherParams{
		UserID: studentID, WeekStart: weekStart, WeekEnd: weekEnd,
	})
	if err != nil {
		t.Fatalf("GetStudentUsageForTeacher: %v", err)
	}
	if usage.ActiveDays != 7 {
		t.Fatalf("GetStudentUsageForTeacher active_days = %d under an Asia/Shanghai session, want 7 (UTC-pinned)", usage.ActiveDays)
	}
}

func mustExec(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(ctx, sql, args...); err != nil {
		t.Fatalf("exec %q: %v", sql, err)
	}
}

func insertEvent(t *testing.T, ctx context.Context, pool *pgxpool.Pool, userID, projectID uuid.UUID, surface, typ string, createdAt time.Time) {
	t.Helper()
	mustExec(t, ctx, pool, `
		INSERT INTO event (user_id, project_id, surface, type, created_at) VALUES ($1, $2, $3, $4, $5)`,
		userID, projectID, surface, typ, createdAt)
}
