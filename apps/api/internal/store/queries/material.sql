-- name: CreateMaterial :one
INSERT INTO material (task_id, kind, source, title, source_url, blocks)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListMaterialsByTask :many
SELECT * FROM material
WHERE task_id = $1
ORDER BY created_at;

-- name: GetMaterial :one
SELECT * FROM material WHERE id = $1;

-- name: UpdateMaterialScratch :one
UPDATE material SET scratch = $3
WHERE id = $1 AND task_id = $2
RETURNING *;

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
