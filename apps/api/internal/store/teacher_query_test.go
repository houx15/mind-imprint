package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/store/sqlc"
)

// TestTeacherReadPathQueries seeds a school with two classes (a teacher owns
// class A; student A is enrolled in class A, student B in a DIFFERENT class
// B) and asserts every teacher.sql query is properly class-scoped: class B's
// student never leaks into class A's roster/report reads, and the
// project-evaluation read is guarded by student ownership the same way
// GetLatestProjectEvaluation's owner-filtered siblings are.
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

	// --- Student A: one project + one evaluations row ---------------------------
	reportJSON, err := json.Marshal(agent.Report{
		DepthAxis: []agent.DepthDim{
			{Code: "D1", Name: "任务理解与问题表述", Level: "L3", Evidence: "e1"},
			{Code: "D2", Name: "信息检索与来源评估", Level: "L2", Evidence: "e2"},
			{Code: "D3", Name: "论证构建", Level: "L3", Evidence: "e3"},
			{Code: "D4", Name: "视角与让步", Level: "L2", Evidence: "e4"},
			{Code: "D5", Name: "证据整合", Level: "L3", Evidence: "e5"},
			{Code: "D6", Name: "元认知与反思", Level: "L2", Evidence: "e6"},
		},
		AutonomyAxis: []agent.AutonomySignal{
			{Code: "A1", Name: "a1", Level: 3, Opportunity: "given_taken", Evidence: "e"},
			{Code: "A2", Name: "a2", Level: 2, Opportunity: "given_taken", Evidence: "e"},
			{Code: "A3", Name: "a3", Level: 4, Opportunity: "given_not_taken", Evidence: "e"},
			{Code: "A4", Name: "a4", Level: 1, Opportunity: "not_supplied", Evidence: "e"},
			{Code: "A5", Name: "a5", Level: 3, Opportunity: "given_taken", Evidence: "e"},
			{Code: "A6", Name: "a6", Level: 2, Opportunity: "given_taken", Evidence: "e"},
		},
		Narrative: "student A narrative",
		Axiom:     "两轴永不合成总分；单次会话为事件级证据，不构成人级档位判定",
	})
	if err != nil {
		t.Fatalf("marshal report fixture: %v", err)
	}

	var projectID uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO project (user_id, title) VALUES ($1, '学生A的项目') RETURNING id`,
		studentA).Scan(&projectID); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	mustExec(t, ctx, pool, `
		INSERT INTO evaluations (project_id, scores, narrative, model, tier, status)
		VALUES ($1, $2, 'proj narrative', 'deepseek-v4-pro', 'flagship', 'done')`,
		projectID, reportJSON)

	// A research_question node so GetStudentProjectEvaluationForTeacher's rq_body
	// subquery has something to find (mirrors project_create.go's shape).
	rqBody, _ := json.Marshal(map[string]any{"text": "中国是否让地球变得更可持续？"})
	mustExec(t, ctx, pool, `
		INSERT INTO graph_node (project_id, type, body, author) VALUES ($1, 'research_question', $2, 'student')`,
		projectID, rqBody)

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

	// --- ListClassRosterReport: only student A in class A, never student B -----
	roster, err := q.ListClassRosterReport(ctx, sqlc.ListClassRosterReportParams{
		ClassID: classA, WeekStart: weekStart, WeekEnd: weekEnd,
	})
	if err != nil {
		t.Fatalf("ListClassRosterReport: %v", err)
	}
	if len(roster) != 1 {
		t.Fatalf("roster has %d rows, want 1 (student A only, student B in a different class excluded)", len(roster))
	}
	row := roster[0]
	if row.ID != studentA {
		t.Fatalf("roster row id = %v, want student A", row.ID)
	}
	if row.ActiveDays != wantActiveDays {
		t.Errorf("active_days = %d, want %d", row.ActiveDays, wantActiveDays)
	}
	if row.Turns != wantTurns {
		t.Errorf("turns = %d, want %d", row.Turns, wantTurns)
	}
	hasReport, ok := row.HasReport.(bool)
	if !ok || !hasReport {
		t.Errorf("has_report = %#v, want true", row.HasReport)
	}
	if row.LatestProjectScores == nil {
		t.Error("latest_project_scores is nil, want the seeded report bytes")
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

	// --- ListStudentReportsForTeacher: the project row appears ------------------
	reports, err := q.ListStudentReportsForTeacher(ctx, studentA)
	if err != nil {
		t.Fatalf("ListStudentReportsForTeacher: %v", err)
	}
	if len(reports) != 1 {
		t.Fatalf("got %d student reports, want 1 (project only)", len(reports))
	}
	if reports[0].Surface != "project" || reports[0].Label != "学生A的项目" {
		t.Errorf("report row = %+v, want project/学生A的项目", reports[0])
	}

	// --- ListStudentProjectsForTeacher: the project appears, has_report=true ---
	projects, err := q.ListStudentProjectsForTeacher(ctx, studentA)
	if err != nil {
		t.Fatalf("ListStudentProjectsForTeacher: %v", err)
	}
	if len(projects) != 1 || !projects[0].HasReport {
		t.Fatalf("projects = %+v, want one row with has_report=true", projects)
	}

	// --- GetLatestReportScoresForStudent: student A has a score row, student B (no project) does not ---
	latestScores, err := q.GetLatestReportScoresForStudent(ctx, studentA)
	if err != nil {
		t.Fatalf("GetLatestReportScoresForStudent(A): %v", err)
	}
	var latestReport agent.Report
	if err := json.Unmarshal(latestScores, &latestReport); err != nil {
		t.Fatalf("unmarshal GetLatestReportScoresForStudent scores: %v", err)
	}
	if len(latestReport.DepthAxis) != 6 {
		t.Errorf("GetLatestReportScoresForStudent depth axis = %d, want 6", len(latestReport.DepthAxis))
	}
	if _, err := q.GetLatestReportScoresForStudent(ctx, studentB); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("GetLatestReportScoresForStudent(B) err = %v, want pgx.ErrNoRows", err)
	}

	// --- GetStudentProjectEvaluationForTeacher: ownership guard -----------------
	got, err := q.GetStudentProjectEvaluationForTeacher(ctx, sqlc.GetStudentProjectEvaluationForTeacherParams{
		ScopeID: pgUUID(projectID), UserID: studentA,
	})
	if err != nil {
		t.Fatalf("GetStudentProjectEvaluationForTeacher(A): %v", err)
	}
	if got.ProjectTitle != "学生A的项目" {
		t.Errorf("project_title = %q, want 学生A的项目", got.ProjectTitle)
	}
	var gotReport agent.Report
	if err := json.Unmarshal(got.Scores, &gotReport); err != nil {
		t.Fatalf("unmarshal scores: %v", err)
	}
	if len(gotReport.DepthAxis) != 6 || len(gotReport.AutonomyAxis) != 6 {
		t.Errorf("report axes = %d depth / %d autonomy, want 6/6", len(gotReport.DepthAxis), len(gotReport.AutonomyAxis))
	}
	var rq struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(got.RqBody, &rq); err != nil {
		t.Fatalf("unmarshal rq_body: %v", err)
	}
	if rq.Text != "中国是否让地球变得更可持续？" {
		t.Errorf("rq_body text = %q, want the seeded research question", rq.Text)
	}

	// Ownership guard: student B is not this project's owner.
	_, err = q.GetStudentProjectEvaluationForTeacher(ctx, sqlc.GetStudentProjectEvaluationForTeacherParams{
		ScopeID: pgUUID(projectID), UserID: studentB,
	})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("GetStudentProjectEvaluationForTeacher(B) err = %v, want pgx.ErrNoRows", err)
	}
}

// TestTeacherReadPathSeedData asserts migration 0029's demo data (吴老师's
// IBDP 一年级 · 研究组, 9 students, 3 with CASE-derived evaluations) is shaped
// the way the teacher read-path endpoints need: exactly 9 roster rows with
// exactly 3 has_report=true, and 林知远's project evaluation unmarshals to a
// full agent.Report (6 depth dims, non-nil officialProjection).
func TestTeacherReadPathSeedData(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTestPool(t)
	q := sqlc.New(pool)

	seededClass := uuid.MustParse("00000000-0000-0000-0000-000000000902")
	lin := uuid.MustParse("00000000-0000-0000-0000-000000000911")
	linProject := uuid.MustParse("00000000-0000-0000-0000-000000000951")

	// A window wide enough to be unaffected by where "now" falls relative to
	// the ISO week boundary (has_report/latest_project_scores don't depend on
	// the window at all; active_days/turns do, but this test doesn't assert
	// on those — only on report presence/shape).
	wideStart := time.Now().AddDate(0, -1, 0)
	wideEnd := time.Now().AddDate(0, 1, 0)

	roster, err := q.ListClassRosterReport(ctx, sqlc.ListClassRosterReportParams{
		ClassID: seededClass, WeekStart: wideStart, WeekEnd: wideEnd,
	})
	if err != nil {
		t.Fatalf("ListClassRosterReport(seeded class): %v", err)
	}
	if len(roster) != 9 {
		t.Fatalf("seeded roster has %d rows, want 9", len(roster))
	}
	haveReport := 0
	for _, row := range roster {
		if hr, _ := row.HasReport.(bool); hr {
			haveReport++
		}
	}
	if haveReport != 3 {
		t.Fatalf("seeded roster has %d has_report=true rows, want 3 (林/沈/周)", haveReport)
	}

	got, err := q.GetStudentProjectEvaluationForTeacher(ctx, sqlc.GetStudentProjectEvaluationForTeacherParams{
		ScopeID: pgUUID(linProject), UserID: lin,
	})
	if err != nil {
		t.Fatalf("GetStudentProjectEvaluationForTeacher(林知远): %v", err)
	}
	var rep agent.Report
	if err := json.Unmarshal(got.Scores, &rep); err != nil {
		t.Fatalf("unmarshal 林知远's report: %v", err)
	}
	if len(rep.DepthAxis) != 6 {
		t.Errorf("林知远's depthAxis = %d, want 6", len(rep.DepthAxis))
	}
	if len(rep.AutonomyAxis) != 6 {
		t.Errorf("林知远's autonomyAxis = %d, want 6", len(rep.AutonomyAxis))
	}
	if len(rep.PromptLens.Lenses) != 6 {
		t.Errorf("林知远's promptLens.lenses = %d, want 6", len(rep.PromptLens.Lenses))
	}
	if rep.OfficialProjection == nil {
		t.Fatal("林知远's officialProjection is nil, want the seeded ap-research projection")
	}
	if rep.OfficialProjection.Readiness.Score != 82 {
		t.Errorf("林知远's readiness score = %d, want 82 (round(81.9))", rep.OfficialProjection.Readiness.Score)
	}
}

// TestActiveDaysPinnedToUTCAcrossSessionTimeZone guards FIX 2 of the
// whole-branch review: teacher.sql's active-days bucketing must be pinned to
// UTC via `AT TIME ZONE 'UTC'`, not the DB session's TimeZone GUC. It seeds
// one event per hour across the entire UTC week window and runs both
// active-days queries (GetStudentUsageForTeacher, ListClassRosterReport) over
// a connection whose session TimeZone is forced to Asia/Shanghai (UTC+8) —
// under the pre-fix `(created_at)::date` cast, that offset shifts the
// early-morning UTC hours into the FOLLOWING local calendar day, so a 7×24h
// UTC window reads as 8 distinct active days. The fix must hold at exactly 7
// regardless of the session's timezone.
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

	roster, err := q.ListClassRosterReport(ctx, sqlc.ListClassRosterReportParams{
		ClassID: classID, WeekStart: weekStart, WeekEnd: weekEnd,
	})
	if err != nil {
		t.Fatalf("ListClassRosterReport: %v", err)
	}
	if len(roster) != 1 {
		t.Fatalf("roster has %d rows, want 1", len(roster))
	}
	if roster[0].ActiveDays != 7 {
		t.Fatalf("ListClassRosterReport active_days = %d under an Asia/Shanghai session, want 7 (UTC-pinned)", roster[0].ActiveDays)
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

// TestGetLatestReportScoresPrefersNewestAcrossSurfaces guards D2's
// student_evaluation view: GetLatestReportScoresForStudent (renamed from
// GetLatestProjectScoresForStudent) must return the newest report across ALL
// three scopes, not project scope only — a chat-only report must not be
// invisible to the D/A head badge.
func TestGetLatestReportScoresPrefersNewestAcrossSurfaces(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	pool := newTestPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	student := createStudentRow(t, pool) // helper below, no class/enrollment needed — the query is a bare user_id lookup
	insertProjectEvaluation(t, pool, student, `{"narrative":"older project"}`, time.Now().Add(-48*time.Hour))
	insertThreadEvaluation(t, pool, student, `{"narrative":"newest chat"}`, time.Now().Add(-1*time.Hour))

	got, err := q.GetLatestReportScoresForStudent(ctx, student)
	if err != nil {
		t.Fatalf("GetLatestReportScoresForStudent: %v", err)
	}
	if !strings.Contains(string(got), "newest chat") {
		t.Fatalf("scores = %s; want the chat evaluation — a chat-only report must not be invisible", got)
	}
}

// createStudentRow seeds a minimal school + student user. GetLatestReportScoresForStudent
// is a bare user_id lookup (not class-scoped), so no class/enrollment row is needed.
func createStudentRow(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var schoolID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO schools (name) VALUES ('D2 Cross-Surface Test School') RETURNING id`).
		Scan(&schoolID); err != nil {
		t.Fatalf("seed school: %v", err)
	}
	var studentID uuid.UUID
	email := fmt.Sprintf("d2-student-%s@example.com", uuid.NewString())
	if err := pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, role, school_id, display_name, avatar_color)
		VALUES ($1, 'x', 'student', $2, 'D2 Student', '#666666')
		RETURNING id`, email, schoolID).Scan(&studentID); err != nil {
		t.Fatalf("seed student: %v", err)
	}
	return studentID
}

// insertProjectEvaluation seeds a project owned by student and one project-scoped
// evaluations row, with an explicit created_at so cross-surface ordering is
// deterministic in tests.
func insertProjectEvaluation(t *testing.T, pool *pgxpool.Pool, student uuid.UUID, scoresJSON string, createdAt time.Time) {
	t.Helper()
	ctx := context.Background()
	var projectID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO project (user_id, title) VALUES ($1, 'cross-surface project') RETURNING id`,
		student).Scan(&projectID); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	mustExec(t, ctx, pool, `
		INSERT INTO evaluations (project_id, scores, narrative, model, tier, status, created_at)
		VALUES ($1, $2, 'project narrative', 'deepseek-v4-pro', 'flagship', 'done', $3)`,
		projectID, scoresJSON, createdAt)
}

// insertThreadEvaluation seeds a chat thread owned by student and one
// thread-scoped evaluations row, mirroring insertProjectEvaluation's shape.
func insertThreadEvaluation(t *testing.T, pool *pgxpool.Pool, student uuid.UUID, scoresJSON string, createdAt time.Time) {
	t.Helper()
	ctx := context.Background()
	var threadID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO chat_thread (user_id, title) VALUES ($1, 'cross-surface thread') RETURNING id`,
		student).Scan(&threadID); err != nil {
		t.Fatalf("seed thread: %v", err)
	}
	mustExec(t, ctx, pool, `
		INSERT INTO evaluations (thread_id, scores, narrative, model, tier, status, created_at)
		VALUES ($1, $2, 'thread narrative', 'deepseek-v4-pro', 'flagship', 'done', $3)`,
		threadID, scoresJSON, createdAt)
}
