-- Course Runtime Slice 8: CourseSession (§16) snapshot persistence. One row per
-- (user, course); the runtime holds authoritative state client-side and PUTs the
-- whole blob. Owner scoping is by user_id — a student can only ever read or write
-- their own session (get-or-create keys by user_id, so a second student always
-- gets their own new row, never someone else's).

-- name: GetCourseSessionBySlug :one
SELECT cs.id, cs.session, cs.status
FROM course_session cs JOIN course c ON c.id = cs.course_id
WHERE cs.user_id = $1 AND c.slug = $2;

-- name: CreateCourseSession :one
INSERT INTO course_session (user_id, course_id, session, status)
VALUES ($1, $2, $3, $4)
RETURNING id, session, status;

-- name: SaveCourseSession :exec
UPDATE course_session SET session = $3, status = $4, updated_at = now()
WHERE user_id = $1 AND course_id = $2;
