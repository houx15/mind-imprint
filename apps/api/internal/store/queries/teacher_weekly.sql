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
