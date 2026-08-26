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
  SELECT count(*) AS n FROM project p WHERE p.user_id = u.id AND p.status = 'active' AND p.kind = 'project'
) ap ON true
LEFT JOIN LATERAL (
  SELECT count(*) AS n FROM evaluation_report er JOIN project p ON p.id = er.project_id
  WHERE p.user_id = u.id AND er.status = 'ready' AND p.kind = 'project'
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
     WHERE p.status = 'active' AND p.kind = 'project')::int AS active_projects,
  (SELECT count(*) FROM evaluation_report er JOIN project p ON p.id = er.project_id
     JOIN members m ON m.id = p.user_id
     WHERE er.status = 'ready' AND er.created_at >= @week_start AND er.created_at < @week_end AND p.kind = 'project')::int AS reports;

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

-- name: ListStudentProjectsForTeacher :many
-- All of a student's projects (title/date + whether a ready report exists),
-- so in-progress projects without a report still appear in the records list.
SELECT p.id, p.title, p.last_active_at,
       EXISTS (SELECT 1 FROM evaluation_report er WHERE er.project_id = p.id AND er.status = 'ready') AS has_report
FROM project p
WHERE p.user_id = @user_id AND p.kind = 'project'
ORDER BY p.last_active_at DESC NULLS LAST;

-- name: CountFinishedCoursesForStudent :one
SELECT count(*)::int FROM course_progress WHERE user_id = @user_id AND completed_at IS NOT NULL;

-- name: GetStudentWeekStats :one
-- One student's four stage-card counts for a half-open window. Same口径 as
-- GetClassWeekStats: active days bucketed via AT TIME ZONE 'UTC'; turns =
-- prompt_sent + course_message; reports = ready evaluation_report rows on
-- this student's projects; course_steps = DISTINCT (course, ordinal)
-- step_viewed. Tenancy is the handler's (authTeacherStudent has proven this
-- student is in the teacher's class).
SELECT
  COUNT(DISTINCT (ev.created_at AT TIME ZONE 'UTC')::date)::int AS active_days,
  COUNT(*) FILTER (WHERE ev.type IN ('prompt_sent','course_message'))::int AS turns,
  (SELECT count(*) FROM evaluation_report er JOIN project p ON p.id = er.project_id
     WHERE p.user_id = @user_id AND er.status = 'ready' AND p.kind = 'project'
       AND er.created_at >= @week_start AND er.created_at < @week_end)::int AS reports,
  COUNT(DISTINCT (ev.course_id, ev.payload->>'ordinal'))
    FILTER (WHERE ev.type = 'step_viewed')::int AS course_steps
FROM event ev
WHERE ev.user_id = @user_id
  AND ev.created_at >= @week_start AND ev.created_at < @week_end;

