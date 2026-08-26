-- D2 weekly-report reads. Class-scoped like teacher.sql: the handler has
-- already authorised the class via assertTeacherOwnsClass, and every student
-- query JOINs enrollments with role_in_class='student'. No writes except the
-- prose row, which holds no student data beyond names already on screen.

-- name: GetClassWeeklyProse :one
SELECT comment, depth_note, autonomy_note, cards, created_at
FROM class_weekly_prose
WHERE class_id = @class_id AND week_start = @week_start;

-- name: InsertClassWeeklyProse :exec
-- First-open-wins (DEC-2): a concurrent second generation is discarded, and
-- the caller re-reads to return the winner's row.
INSERT INTO class_weekly_prose (class_id, week_start, comment, depth_note, autonomy_note, cards)
VALUES (@class_id, @week_start, @comment, @depth_note, @autonomy_note, @cards)
ON CONFLICT (class_id, week_start) DO NOTHING;

-- name: AppendClassWeeklyProseCards :exec
-- The top-up (DEC-6): appends newly-composed card prose to an existing row.
-- comment/depth_note/autonomy_note are never touched — nothing already written
-- is ever rewritten.
UPDATE class_weekly_prose
SET cards = cards || @cards::jsonb, updated_at = now()
WHERE class_id = @class_id AND week_start = @week_start;

-- name: GetClassWeekStats :one
-- The four stat cards for one half-open window. 对话轮次 口径 is D1's, verbatim:
-- prompt_sent (studio AND chat) + course_message. Course steps count DISTINCT
-- (student, course, ordinal) so a re-render cannot inflate the number.
WITH members AS (
  SELECT u.id
  FROM enrollments e JOIN users u ON u.id = e.user_id
  WHERE e.class_id = @class_id AND e.role_in_class = 'student'
)
SELECT
  (SELECT count(*) FROM members)::int AS class_size,
  (SELECT count(DISTINCT ev.user_id) FROM event ev JOIN members m ON m.id = ev.user_id
     WHERE ev.created_at >= @week_start AND ev.created_at < @week_end)::int AS active_students,
  (SELECT count(*) FROM event ev JOIN members m ON m.id = ev.user_id
     WHERE ev.created_at >= @week_start AND ev.created_at < @week_end
       AND ev.type IN ('prompt_sent','course_message'))::int AS turns,
  (SELECT count(DISTINCT (ev.user_id, ev.course_id, ev.payload->>'ordinal'))
     FROM event ev JOIN members m ON m.id = ev.user_id
     WHERE ev.created_at >= @week_start AND ev.created_at < @week_end
       AND ev.type = 'step_viewed')::int AS course_steps,
  (SELECT count(*) FROM evaluation_report er JOIN project p ON p.id = er.project_id
     JOIN members m ON m.id = p.user_id
     WHERE er.status = 'ready' AND er.created_at >= @week_start AND er.created_at < @week_end AND p.kind = 'project')::int AS reports;

-- name: ListClassStudentWeekActivity :many
-- Per-student activity for a COMPLETED week + the full prior week, plus report
-- counts (this week / before this week) and the latest ready report's project id
-- for the card link. All no-LLM; no student_evaluation.
--
-- latest_report_project_id is joined via a plain (non-LATERAL) LEFT JOIN to a
-- CTE, not a LATERAL "ON true" subselect or a scalar subquery in the SELECT
-- list: sqlc's nullability inference does not propagate "this join might not
-- match" through those two shapes (it kept typing the column as the
-- underlying NOT NULL evaluation_report.project_id, which pgx then fails to
-- scan when a student has no ready report). A CTE joined by an ordinary
-- LEFT JOIN ... ON condition is the shape sqlc reliably marks nullable
-- (-> pgtype.UUID).
WITH latest_reports AS (
  SELECT DISTINCT ON (p.user_id) p.user_id, er.project_id,
         er.report->'abstract'->>'overview' AS overview
  FROM evaluation_report er JOIN project p ON p.id = er.project_id
  WHERE er.status = 'ready' AND p.kind = 'project'
  ORDER BY p.user_id, er.created_at DESC
)
SELECT
  u.id AS user_id, u.display_name, u.avatar_color,
  COALESCE(cur.active_days, 0)::int AS active_days,
  COALESCE(cur.turns, 0)::int       AS turns,
  COALESCE(prv.active_days, 0)::int AS prev_active_days,
  COALESCE(rep.n, 0)::int           AS reports_this_week,
  COALESCE(prior.n, 0)::int         AS prior_reports,
  lr.project_id                     AS latest_report_project_id,
  lr.overview                       AS latest_report_overview
FROM enrollments e
JOIN users u ON u.id = e.user_id
LEFT JOIN LATERAL (
  SELECT COUNT(DISTINCT (ev.created_at AT TIME ZONE 'UTC')::date) AS active_days,
         COUNT(*) FILTER (WHERE ev.type IN ('prompt_sent','course_message')) AS turns
  FROM event ev
  WHERE ev.user_id = u.id AND ev.created_at >= @week_start AND ev.created_at < @week_end
) cur ON true
LEFT JOIN LATERAL (
  SELECT COUNT(DISTINCT (ev.created_at AT TIME ZONE 'UTC')::date) AS active_days
  FROM event ev
  WHERE ev.user_id = u.id AND ev.created_at >= @prev_start AND ev.created_at < @prev_end
) prv ON true
LEFT JOIN LATERAL (
  SELECT count(*) AS n FROM evaluation_report er JOIN project p ON p.id = er.project_id
  WHERE p.user_id = u.id AND er.status = 'ready' AND p.kind = 'project' AND er.created_at >= @week_start AND er.created_at < @week_end
) rep ON true
LEFT JOIN LATERAL (
  SELECT count(*) AS n FROM evaluation_report er JOIN project p ON p.id = er.project_id
  WHERE p.user_id = u.id AND er.status = 'ready' AND p.kind = 'project' AND er.created_at < @week_start
) prior ON true
LEFT JOIN latest_reports lr ON lr.user_id = u.id
WHERE e.class_id = @class_id AND e.role_in_class = 'student'
ORDER BY u.display_name;
