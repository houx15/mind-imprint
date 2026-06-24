-- name: CreateTask :one
INSERT INTO tasks (user_id, title, seed)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetTask :one
SELECT * FROM tasks WHERE id = $1;

-- name: ListTasksByUser :many
SELECT * FROM tasks
WHERE user_id = $1
ORDER BY last_active_at DESC;
