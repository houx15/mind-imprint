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

-- name: SetCardActive :one
UPDATE card_instances
SET status = 'active'
WHERE id = $1 AND task_id = $2
RETURNING *;

-- name: SubmitCard :one
UPDATE card_instances
SET field_values = $3, event_trace = $4, status = 'completed', completed_at = now()
WHERE id = $1 AND task_id = $2
RETURNING *;

-- name: SkipCard :one
UPDATE card_instances
SET event_trace = $3, status = 'skipped'
WHERE id = $1 AND task_id = $2
RETURNING *;

-- name: CountCompletedCards :one
SELECT count(*) FROM card_instances
WHERE task_id = $1 AND status = 'completed';
