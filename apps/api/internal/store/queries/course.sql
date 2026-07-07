-- name: ListCourses :many
SELECT c.*, count(s.id) AS step_count
FROM course c
LEFT JOIN course_step s ON s.course_id = c.id
GROUP BY c.id
ORDER BY c.created_at;

-- name: GetCourse :one
SELECT * FROM course WHERE id = $1;

-- name: CountCourseSteps :one
SELECT count(*) FROM course_step WHERE course_id = $1;

-- name: ListCourseSteps :many
SELECT * FROM course_step WHERE course_id = $1 ORDER BY ordinal;

-- name: GetCourseProgress :one
SELECT * FROM course_progress WHERE user_id = $1 AND course_id = $2;

-- name: UpsertCourseProgress :one
INSERT INTO course_progress (user_id, course_id, current_ordinal, completed_ordinals)
VALUES ($1, $2, $3, $4)
ON CONFLICT (user_id, course_id) DO UPDATE
SET current_ordinal = EXCLUDED.current_ordinal,
    completed_ordinals = EXCLUDED.completed_ordinals,
    updated_at = now()
RETURNING *;
