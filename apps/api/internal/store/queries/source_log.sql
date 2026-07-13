-- The source log (spec §5, product-spec §9/S2). One row per source the student
-- brought in; time_spent_s accumulates across opens. RL-2's ledger.

-- name: CreateSourceLogEntry :one
INSERT INTO source_log_entry (project_id, material_id, url, title, takeaway, tier)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListSourceLogByProject :many
SELECT * FROM source_log_entry
WHERE project_id = $1
ORDER BY opened_at;

-- name: GetSourceLogByMaterial :one
SELECT * FROM source_log_entry WHERE material_id = $1;

-- name: AddSourceTimeSpent :exec
UPDATE source_log_entry
SET time_spent_s = time_spent_s + $2
WHERE material_id = $1;
