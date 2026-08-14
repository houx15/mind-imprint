-- Teacher read-path (Spec D1). Every query is class-scoped: it JOINs enrollments
-- with role_in_class='student' so a teacher can only read members of the class
-- the handler already authorised via assertTeacherOwnsClass. No writes.

-- name: ListClassRosterCounts :many
-- One row per student: current-state activity counts, all no-LLM. active
-- projects = status='active'; report count = ready evaluation_report on the
-- student's projects; courses finished = course_progress.completed_at set.
-- Class-scoped like every teacher query (enrollments + role_in_class='student').
SELECT
  u.id, u.display_name, u.avatar_color,
  COALESCE(ap.n, 0)::int AS active_projects,
  COALESCE(rc.n, 0)::int AS report_count,
  COALESCE(cf.n, 0)::int AS courses_finished
FROM enrollments e
JOIN users u ON u.id = e.user_id
LEFT JOIN LATERAL (
  SELECT count(*) AS n FROM project p WHERE p.user_id = u.id AND p.status = 'active'
) ap ON true
LEFT JOIN LATERAL (
  SELECT count(*) AS n FROM evaluation_report er JOIN project p ON p.id = er.project_id
  WHERE p.user_id = u.id AND er.status = 'ready'
) rc ON true
LEFT JOIN LATERAL (
  SELECT count(*) AS n FROM course_progress cp WHERE cp.user_id = u.id AND cp.completed_at IS NOT NULL
) cf ON true
WHERE e.class_id = @class_id AND e.role_in_class = 'student'
ORDER BY u.display_name;

-- name: GetClassLiveHeader :one
-- Live class-level snapshot for View B's header. active_students/turns/reports
-- are windowed (the current in-progress week); active_projects is current state.
WITH members AS (
  SELECT u.id FROM enrollments e JOIN users u ON u.id = e.user_id
  WHERE e.class_id = @class_id AND e.role_in_class = 'student'
)
SELECT
  (SELECT count(*) FROM members)::int AS class_size,
  (SELECT count(DISTINCT ev.user_id) FROM event ev JOIN members m ON m.id = ev.user_id
     WHERE ev.created_at >= @week_start AND ev.created_at < @week_end)::int AS active_students,
  (SELECT count(*) FROM event ev JOIN members m ON m.id = ev.user_id
     WHERE ev.created_at >= @week_start AND ev.created_at < @week_end
       AND ev.type IN ('prompt_sent','course_message'))::int AS turns,
  (SELECT count(*) FROM project p JOIN members m ON m.id = p.user_id
     WHERE p.status = 'active')::int AS active_projects,
  (SELECT count(*) FROM evaluation_report er JOIN project p ON p.id = er.project_id
     JOIN members m ON m.id = p.user_id
     WHERE er.status = 'ready' AND er.created_at >= @week_start AND er.created_at < @week_end)::int AS reports;

-- name: GetStudentUsageForTeacher :one
-- This-week active days + turns for one student (used by the student detail head).
-- Bucketed via `AT TIME ZONE 'UTC'` (not the DB session's TimeZone GUC, which
-- may differ from the app's) so a 7x24h window (weekWindow, UTC-pinned) can
-- never span more than 7 distinct dates.
SELECT
  COUNT(DISTINCT (ev.created_at AT TIME ZONE 'UTC')::date) AS active_days,
  COUNT(*) FILTER (WHERE ev.type IN ('prompt_sent','course_message')) AS turns
FROM event ev
WHERE ev.user_id = @user_id
  AND ev.created_at >= @week_start AND ev.created_at < @week_end;

-- name: ListStudentReportsForTeacher :many
-- Every report one class member owns, across both remaining scopes,
-- newest-first, one row per scope. A teacher-scoped read of the student's
-- stored evaluations, filtered by "this user AND a student member
-- of this class". The course-session arm is retired along with course_session
-- itself (migration 0050, course v2, no back-compat).
SELECT surface, scope_id, label, sublabel, created_at
FROM (
  (SELECT DISTINCT ON (e.project_id)
     'project'::text AS surface, e.project_id AS scope_id,
     p.title AS label, NULL::text AS sublabel, e.created_at AS created_at
   FROM evaluations e JOIN project p ON p.id = e.project_id
   WHERE e.project_id IS NOT NULL AND p.user_id = @user_id
   ORDER BY e.project_id, e.created_at DESC)
  UNION ALL
  (SELECT DISTINCT ON (e.thread_id)
     'chat'::text, e.thread_id, t.title, NULL::text, e.created_at
   FROM evaluations e JOIN chat_thread t ON t.id = e.thread_id
   WHERE e.thread_id IS NOT NULL AND t.user_id = @user_id
   ORDER BY e.thread_id, e.created_at DESC)
) rows
ORDER BY created_at DESC;

-- name: ListStudentProjectsForTeacher :many
-- All of a student's projects (title/date + whether a report exists), so
-- in-progress projects without a report still appear in the records list.
SELECT p.id, p.title, p.last_active_at,
       EXISTS (SELECT 1 FROM evaluations ev WHERE ev.project_id = p.id) AS has_report
FROM project p
WHERE p.user_id = @user_id
ORDER BY p.last_active_at DESC NULLS LAST;

-- name: GetLatestReportScoresForStudent :one
-- Latest report scores for one student across ALL scopes (project/course/chat),
-- for D/A head-badge derivation on the student-detail page. (The roster view
-- no longer derives a D/A badge — see ListClassRosterCounts — but the
-- student-detail head still does, until Task 4.)
SELECT se.scores
FROM student_evaluation se
WHERE se.user_id = @user_id
ORDER BY se.created_at DESC
LIMIT 1;

-- name: GetStudentWeekStats :one
-- One student's four stage-card counts for a half-open window. Same口径 as
-- GetClassWeekStats: active days bucketed via AT TIME ZONE 'UTC'; turns =
-- prompt_sent + course_message; reports = student_evaluation rows; course_steps
-- = DISTINCT (course, ordinal) step_viewed. Tenancy is the handler's
-- (authTeacherStudent has proven this student is in the teacher's class).
SELECT
  COUNT(DISTINCT (ev.created_at AT TIME ZONE 'UTC')::date)::int AS active_days,
  COUNT(*) FILTER (WHERE ev.type IN ('prompt_sent','course_message'))::int AS turns,
  (SELECT count(*) FROM student_evaluation se
     WHERE se.user_id = @user_id
       AND se.created_at >= @week_start AND se.created_at < @week_end)::int AS reports,
  COUNT(DISTINCT (ev.course_id, ev.payload->>'ordinal'))
    FILTER (WHERE ev.type = 'step_viewed')::int AS course_steps
FROM event ev
WHERE ev.user_id = @user_id
  AND ev.created_at >= @week_start AND ev.created_at < @week_end;

