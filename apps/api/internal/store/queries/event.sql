-- Append-only event stream (C4). Deliberately no update/delete query.

-- name: AppendEvent :one
INSERT INTO event (project_id, user_id, session_id, thread_id, course_id, surface, type, payload)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: ListEventsByProject :many
SELECT * FROM event
WHERE project_id = $1
ORDER BY created_at, id;

-- name: ListEventsBySession :many
SELECT * FROM event
WHERE session_id = $1
ORDER BY created_at, id;

-- name: ListEventsByThread :many
SELECT * FROM event
WHERE thread_id = $1
ORDER BY created_at, id;

-- name: ListEventsByCourse :many
SELECT * FROM event
WHERE course_id = $1
ORDER BY created_at, id;

-- name: CountEventsByType :one
-- G1 · used to emit a project's framework-finished milestone at most once.
SELECT COUNT(*) FROM event
WHERE project_id = $1 AND type = $2;

-- name: GetFrameworkFinishedAt :one
-- G1 · the 立题完成 milestone timestamp for the report generator (NULL when the
-- project never generated a plan). MIN so a re-generation never moves it.
SELECT MIN(created_at)::timestamptz FROM event
WHERE project_id = $1 AND type = 'milestone:framework_finished';
