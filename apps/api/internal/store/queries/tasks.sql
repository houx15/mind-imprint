-- name: CreateTask :one
INSERT INTO tasks (user_id, title, seed)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetTask :one
-- Still used by loadOwnedTask (api/tasks.go) for the surviving
-- /api/v1/tasks/{id}/evaluate + /evaluation routes, and directly by fixtures.
SELECT * FROM tasks WHERE id = $1;

-- name: MarkTaskEvaluated :exec
UPDATE tasks SET status = 'evaluated' WHERE id = $1;
