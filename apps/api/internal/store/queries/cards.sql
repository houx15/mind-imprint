-- name: CreateCardInstance :one
INSERT INTO card_instances (card_id, task_id, status)
VALUES ($1, $2, 'proposed')
RETURNING *;

-- name: GetCard :one
SELECT * FROM card_instances WHERE id = $1;

-- name: ListCardsByTask :many
SELECT * FROM card_instances
WHERE task_id = $1
ORDER BY created_at, id;
