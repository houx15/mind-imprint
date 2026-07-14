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

-- name: MarkSourceLateralRead :execrows
-- The source that WAS laterally read (not the source used to do it). tier is
-- overwritten only when the student re-tiered it after checking; an empty
-- tier_after leaves her ingestion-time tier alone. :execrows (not :exec) so
-- the caller (CommitCardMint) can detect a zero-row match — a project/
-- material pair with no source_log_entry at all — and fail loudly instead of
-- silently leaving lateral_read stuck false forever (whole-branch review
-- IMPORTANT 4).
UPDATE source_log_entry
SET lateral_read = true,
    tier = CASE WHEN @tier_after::text = '' THEN tier ELSE @tier_after::text END
WHERE project_id = @project_id AND material_id = @material_id;
