-- name: CreateTask :one
INSERT INTO tasks (user_id, title, seed)
VALUES ($1, $2, $3)
RETURNING *;
