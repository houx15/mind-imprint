-- name: GetMaterial :one
-- Still used by agentstore.go (the new project turn loop's material context),
-- not just the retired task-scoped material handlers.
SELECT * FROM material WHERE id = $1;

-- Project-scoped reads/writes (Slice 3): the classifier's surface_card
-- predicate reads a project's source materials. task_id is nullable as of
-- 0020 (legacy FK from the deleted task surface); project-scoped ingestion
-- (6b) passes NULL, old fixtures may still supply it.

-- name: CreateProjectMaterial :one
INSERT INTO material (task_id, project_id, kind, source, title, source_url, blocks)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListMaterialsByProject :many
SELECT * FROM material
WHERE project_id = $1
ORDER BY created_at;

-- Thread-scoped materials (Slice 11): task_id + project_id NULL, thread_id set.

-- name: CreateThreadMaterial :one
INSERT INTO material (thread_id, kind, source, title, source_url, blocks)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListMaterialsByThread :many
SELECT * FROM material WHERE thread_id = $1 ORDER BY created_at;

-- Course session scope (Slice 12): task_id/project_id/thread_id NULL,
-- session_id set. No source_url — a course anchor material is an authored
-- claim, not a fetched/pasted link (kind/source stay within the existing
-- ('article','draft') / ('fetched','pasted') CHECKs; see course_step.go).

-- name: CreateSessionMaterial :one
INSERT INTO material (session_id, kind, source, title, blocks)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListMaterialsBySession :many
SELECT * FROM material WHERE session_id = $1 ORDER BY created_at, id;
