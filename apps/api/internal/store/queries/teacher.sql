-- Teacher read-path (Spec D1). Every query is class-scoped: it JOINs enrollments
-- with role_in_class='student' so a teacher can only read members of the class
-- the handler already authorised via assertTeacherOwnsClass. No writes.

-- name: ListClassRosterReport :many
-- One row per student: latest report scores across all scopes (for D/A badge
-- derivation, NULL when unrated) + this-week activity. Mirrors GetClassRoster's
-- enrollment scoping (org.sql).
SELECT
  u.id, u.display_name, u.avatar_color,
  ev.scores AS latest_project_scores,
  COALESCE(act.active_days, 0)::int AS active_days,
  COALESCE(act.turns, 0)::int       AS turns,
  (ev.scores IS NOT NULL)           AS has_report
FROM enrollments e
JOIN users u ON u.id = e.user_id
LEFT JOIN LATERAL (
  SELECT se.scores
  FROM student_evaluation se
  WHERE se.user_id = u.id
  ORDER BY se.created_at DESC
  LIMIT 1
) ev ON true
LEFT JOIN LATERAL (
  SELECT
    COUNT(DISTINCT (ev4.created_at AT TIME ZONE 'UTC')::date) AS active_days,
    COUNT(*) FILTER (WHERE ev4.type IN ('prompt_sent','course_message')) AS turns
  FROM event ev4
  WHERE ev4.user_id = u.id
    AND ev4.created_at >= @week_start AND ev4.created_at < @week_end
) act ON true
WHERE e.class_id = @class_id AND e.role_in_class = 'student'
ORDER BY u.display_name;

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
-- Every report one class member owns, across all three scopes, newest-first,
-- one row per scope. Teacher variant of ListGrowthHistory: same shape, but the
-- owner filter is replaced by "this user AND a student member of this class".
SELECT surface, scope_id, label, sublabel, created_at
FROM (
  (SELECT DISTINCT ON (e.project_id)
     'project'::text AS surface, e.project_id AS scope_id,
     p.title AS label, NULL::text AS sublabel, e.created_at AS created_at
   FROM evaluations e JOIN project p ON p.id = e.project_id
   WHERE e.project_id IS NOT NULL AND p.user_id = @user_id
   ORDER BY e.project_id, e.created_at DESC)
  UNION ALL
  (SELECT DISTINCT ON (e.session_id)
     'course'::text, e.session_id, c.title, cs.phase, e.created_at
   FROM evaluations e
     JOIN course_session cs ON cs.id = e.session_id
     JOIN course c ON c.id = cs.course_id
   WHERE e.session_id IS NOT NULL AND cs.user_id = @user_id
   ORDER BY e.session_id, e.created_at DESC)
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
-- for D/A head-badge derivation on the student-detail page. Mirrors the lateral
-- inside ListClassRosterReport so the roster badge and the head badge can never
-- disagree.
SELECT se.scores
FROM student_evaluation se
WHERE se.user_id = @user_id
ORDER BY se.created_at DESC
LIMIT 1;

-- name: GetStudentProjectEvaluationForTeacher :one
-- Read one project report + its RQ context, guarded by student ownership.
-- researchQuestion: prefer the plan node's research_question, else project.title.
SELECT e.scores, e.created_at, p.title AS project_title,
       (SELECT gn.body FROM graph_node gn
          WHERE gn.project_id = p.id AND gn.type = 'research_question'
          ORDER BY gn.created_at, gn.id LIMIT 1) AS rq_body
FROM evaluations e
JOIN project p ON p.id = e.project_id
WHERE e.project_id = @scope_id AND p.user_id = @user_id
ORDER BY e.created_at DESC
LIMIT 1;

-- name: GetStudentSessionEvaluationForTeacher :one
SELECT e.scores, e.created_at, c.title AS course_title
FROM evaluations e
JOIN course_session cs ON cs.id = e.session_id
JOIN course c ON c.id = cs.course_id
WHERE e.session_id = @scope_id AND cs.user_id = @user_id
ORDER BY e.created_at DESC
LIMIT 1;

-- name: GetStudentThreadEvaluationForTeacher :one
SELECT e.scores, e.created_at, t.title AS thread_title
FROM evaluations e
JOIN chat_thread t ON t.id = e.thread_id
WHERE e.thread_id = @scope_id AND t.user_id = @user_id
ORDER BY e.created_at DESC
LIMIT 1;
