-- course_session / course_message queries (Slice 12, Course policy).
-- A session is the runtime unit: one per (user, course), holding the phase.
-- course_progress stays the page-position unit and is queried separately —
-- it is also the server-side truth the steps_viewed floor reads (DEC-12.2).

-- name: CreateCourseSession :one
INSERT INTO course_session (user_id, course_id, skill_id, phase)
VALUES ($1, $2, $3, $4)
ON CONFLICT (user_id, course_id) DO UPDATE SET updated_at = now()
RETURNING *;

-- name: GetCourseSession :one
SELECT * FROM course_session WHERE id = $1;

-- name: GetCourseSessionByUserCourse :one
SELECT * FROM course_session WHERE user_id = $1 AND course_id = $2;

-- name: SetCourseSessionPhase :one
UPDATE course_session SET phase = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: SetCourseSessionStatus :one
UPDATE course_session SET status = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: CreateCourseMessage :one
INSERT INTO course_message (session_id, phase, role, content)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListMessagesBySession :many
SELECT * FROM course_message WHERE session_id = $1 ORDER BY created_at, id;

-- name: CountStudentTurnsInPhase :one
SELECT count(*) FROM course_message
WHERE session_id = $1 AND phase = $2 AND role = 'student';
