-- name: GetMaterial :one
-- Still used by agentstore.go (the new project turn loop's material context),
-- not just the retired task-scoped material handlers.
SELECT * FROM material WHERE id = $1;

-- Project-scoped reads/writes (Slice 3): the classifier's surface_card
-- predicate reads a project's source materials; task_id stays required
-- (legacy FK, not yet dropped) so fixtures still supply it.

-- name: CreateProjectMaterial :one
INSERT INTO material (task_id, project_id, kind, source, title, source_url, blocks)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListMaterialsByProject :many
SELECT * FROM material
WHERE project_id = $1
ORDER BY created_at;
