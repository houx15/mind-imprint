-- name: InsertRevisionCheckpoint :one
INSERT INTO revision_checkpoint (project_id, artifact_type, trigger, content, content_hash, feedback_ref)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListRevisionCheckpoints :many
SELECT * FROM revision_checkpoint
WHERE project_id = $1
ORDER BY created_at, artifact_type;
