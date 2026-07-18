-- Append-only event stream (C4). Deliberately no update/delete query.

-- name: AppendEvent :one
INSERT INTO event (project_id, user_id, session_id, thread_id, surface, type, payload)
VALUES ($1, $2, $3, $4, $5, $6, $7)
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
