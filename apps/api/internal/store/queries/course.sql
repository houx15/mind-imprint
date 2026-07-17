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

-- name: SetCourseCurrentOrdinal :one
-- The resume-position write (PUT /courses/{id}/progress): current_ordinal is
-- UX, never a floor input (Slice-12 whole-branch C1+C3 fix), so this is the
-- ONLY column it touches. completed_ordinals is left untouched on conflict —
-- RecordCourseStepViewed below is its only writer — and defaults to empty on
-- a fresh row (no step has been server-recorded as viewed yet).
INSERT INTO course_progress (user_id, course_id, current_ordinal, completed_ordinals)
VALUES ($1, $2, $3, '{}')
ON CONFLICT (user_id, course_id) DO UPDATE
SET current_ordinal = EXCLUDED.current_ordinal, updated_at = now()
RETURNING *;

-- name: RecordCourseStepViewed :one
-- The `steps_viewed` floor's ONLY input writer (DEC-12.2 / whole-branch
-- C1+C3): called once per real POST .../steps/{ordinal}/render — the moment
-- the student's browser actually opens that page — appending the ordinal to
-- completed_ordinals idempotently (array_agg DISTINCT dedupes a re-render of
-- an already-viewed page). A client can never assert this column: PUT
-- /progress (SetCourseCurrentOrdinal above) does not accept it.
INSERT INTO course_progress (user_id, course_id, current_ordinal, completed_ordinals)
VALUES ($1, $2, sqlc.arg(ordinal), ARRAY[sqlc.arg(ordinal)]::int[])
ON CONFLICT (user_id, course_id) DO UPDATE
SET completed_ordinals = (
        SELECT array_agg(DISTINCT v ORDER BY v)
        FROM unnest(array_append(course_progress.completed_ordinals, sqlc.arg(ordinal)::int)) AS v
    ),
    updated_at = now()
RETURNING *;

-- name: GetCourseStepByOrdinal :one
SELECT * FROM course_step WHERE course_id = $1 AND ordinal = $2;

-- name: GetCourseStepRender :one
SELECT * FROM course_step_render WHERE course_step_id = $1;

-- name: UpsertCourseStepRender :one
INSERT INTO course_step_render (course_step_id, content, source)
VALUES ($1, $2, $3)
ON CONFLICT (course_step_id) DO UPDATE
SET content = EXCLUDED.content, source = EXCLUDED.source, created_at = now()
RETURNING *;
