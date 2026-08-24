-- Course Runtime Slice 8: CourseSession (§16) snapshot persistence. One row per
-- (user, course); the runtime holds authoritative state client-side and PUTs the
-- whole blob. Owner scoping is by user_id — a student can only ever read or write
-- their own session (get-or-create keys by user_id, so a second student always
-- gets their own new row, never someone else's).

-- name: GetCourseSessionBySlug :one
-- The CURRENT attempt: newest by created_at. Since 0077 a (user, course) pair can
-- hold multiple attempts (relearns), so resume/get-or-create reads the latest.
SELECT cs.id, cs.session, cs.status
FROM course_session cs JOIN course c ON c.id = cs.course_id
WHERE cs.user_id = $1 AND c.slug = $2
ORDER BY cs.created_at DESC
LIMIT 1;

-- name: GetCourseSessionForReport :one
-- One SPECIFIC past attempt, for its frozen report — owner- and course-scoped so
-- a student can only ever read their own attempt of the course the report is for.
-- No row (unknown/foreign attempt id) → pgx.ErrNoRows → 404 at the handler.
SELECT cs.session, cs.status
FROM course_session cs JOIN course c ON c.id = cs.course_id
WHERE cs.id = $1 AND cs.user_id = $2 AND c.slug = $3;

-- name: CreateCourseSession :one
-- A fresh attempt. Called on first entry (get-or-create) AND on relearn (restart
-- mints a brand-new attempt beside the kept finished one).
INSERT INTO course_session (user_id, course_id, session, status)
VALUES ($1, $2, $3, $4)
RETURNING id, session, status;

-- name: SaveCourseSession :exec
-- Snapshot-write the CURRENT attempt (the newest row) only — never every attempt.
-- completed_at is FROZEN: written once, the first time this attempt reaches
-- 'completed', and left untouched by any later save so the history date stays put.
UPDATE course_session SET session = $3, status = $4, updated_at = now(),
    completed_at = CASE WHEN $4 = 'completed' AND completed_at IS NULL THEN now() ELSE completed_at END
WHERE id = (
    SELECT cs.id FROM course_session cs
    WHERE cs.user_id = $1 AND cs.course_id = $2
    ORDER BY cs.created_at DESC
    LIMIT 1
);

-- name: DeleteCourseSession :exec
-- Legacy restart safety-net: drop ALL of a student's attempts for a course.
-- (2.0 restart no longer calls this — it keeps finished attempts; see
-- DeleteIncompleteLatestCourseSession.) Idempotent.
DELETE FROM course_session WHERE user_id = $1 AND course_id = $2;

-- name: DeleteIncompleteLatestCourseSession :exec
-- Relearn housekeeping: if the current (newest) attempt never finished, drop it
-- before minting the fresh one, so abandoned partial attempts don't accrete in
-- the history. A completed current attempt is KEPT (the AND status guard fails).
DELETE FROM course_session
WHERE id = (
    SELECT cs.id FROM course_session cs
    WHERE cs.user_id = $1 AND cs.course_id = $2
    ORDER BY cs.created_at DESC
    LIMIT 1
) AND status <> 'completed';
