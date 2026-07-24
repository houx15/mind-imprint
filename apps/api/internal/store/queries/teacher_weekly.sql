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
  (SELECT count(*) FROM student_evaluation se JOIN members m ON m.id = se.user_id
     WHERE se.created_at >= @week_start AND se.created_at < @week_end)::int AS reports;

-- name: ListClassStudentWindowUsage :many
-- Per-student usage for THIS window and the same elapsed offset LAST week, plus
-- how many reports landed this week. Bucketed via `AT TIME ZONE 'UTC'` for the
-- same reason teacher.sql is: a 7×24h window must never span 8 UTC dates.
SELECT
  u.id AS user_id, u.display_name, u.avatar_color,
  COALESCE(cur.active_days, 0)::int AS active_days,
  COALESCE(cur.turns, 0)::int       AS turns,
  COALESCE(prv.active_days, 0)::int AS prev_active_days,
  COALESCE(prv.turns, 0)::int       AS prev_turns,
  COALESCE(rep.n, 0)::int           AS reports_this_week
FROM enrollments e
JOIN users u ON u.id = e.user_id
LEFT JOIN LATERAL (
  SELECT COUNT(DISTINCT (ev.created_at AT TIME ZONE 'UTC')::date) AS active_days,
         COUNT(*) FILTER (WHERE ev.type IN ('prompt_sent','course_message')) AS turns
  FROM event ev
  WHERE ev.user_id = u.id AND ev.created_at >= @week_start AND ev.created_at < @week_end
) cur ON true
LEFT JOIN LATERAL (
  SELECT COUNT(DISTINCT (ev.created_at AT TIME ZONE 'UTC')::date) AS active_days,
         COUNT(*) FILTER (WHERE ev.type IN ('prompt_sent','course_message')) AS turns
  FROM event ev
  WHERE ev.user_id = u.id AND ev.created_at >= @prev_start AND ev.created_at < @prev_end
) prv ON true
LEFT JOIN LATERAL (
  SELECT count(*) AS n FROM student_evaluation se
  WHERE se.user_id = u.id AND se.created_at >= @week_start AND se.created_at < @week_end
) rep ON true
WHERE e.class_id = @class_id AND e.role_in_class = 'student'
ORDER BY u.display_name;

-- name: ListClassRecentReports :many
-- Each student's two newest reports across all scopes: rn=1 is 最新, rn=2 is
-- 上一次 (the baseline for 深度升档 / 更愿意自己想 / the A-axis delta). Students
-- with no report contribute no rows — 敢于空白, not a zero.
SELECT se.user_id, se.scores, se.created_at, se.surface, se.scope_id, se.rn::int AS rn
FROM enrollments e
JOIN LATERAL (
  SELECT s.user_id, s.scores, s.created_at, s.surface, s.scope_id,
         row_number() OVER (ORDER BY s.created_at DESC) AS rn
  FROM student_evaluation s
  WHERE s.user_id = e.user_id
  ORDER BY s.created_at DESC
  LIMIT 2
) se ON true
WHERE e.class_id = @class_id AND e.role_in_class = 'student'
ORDER BY se.user_id, se.rn;
