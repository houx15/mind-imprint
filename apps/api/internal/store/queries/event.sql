-- Append-only event stream (C4). Deliberately no update/delete query.

-- name: AppendEvent :one
INSERT INTO event (project_id, user_id, surface, type, payload)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListEventsByProject :many
SELECT * FROM event
WHERE project_id = $1
ORDER BY created_at, id;
