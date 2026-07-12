-- Project-scoped card_instance lifecycle (Slice 3, agent-spec §3). The
-- table is `card_instances` (Slice-0's task-scoped table, extended by
-- migration 0016 with project_id/contract_ref/framework_fill); task_id
-- stays NOT NULL (legacy FK, not yet dropped) so callers still supply it —
-- the runtime resolves it from the target material's own task_id
-- (agentstore.go), never asking the pure agent code to know about tasks.
-- The old task-scoped queries in cards.sql (CreateCardInstance, SetCardActive,
-- SubmitCard, SkipCard, SetCardAnchors) stay untouched for the legacy path.

-- name: CreateProjectCardInstance :one
INSERT INTO card_instances (task_id, project_id, card_id, contract_ref, status)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: SetCardInstanceAnchors :one
UPDATE card_instances SET anchors = $3
WHERE id = $1 AND project_id = $2
RETURNING *;

-- name: SetCardInstanceFramework :one
UPDATE card_instances SET framework_fill = $3
WHERE id = $1 AND project_id = $2
RETURNING *;

-- name: SetCardInstanceStatus :one
UPDATE card_instances SET status = $3
WHERE id = $1 AND project_id = $2
RETURNING *;

-- name: SubmitProjectCardInstance :one
UPDATE card_instances SET field_values = $3, event_trace = $4
WHERE id = $1 AND project_id = $2
RETURNING *;

-- name: GetCardInstance :one
SELECT * FROM card_instances WHERE id = $1;

-- name: ListCardInstancesByProject :many
SELECT * FROM card_instances
WHERE project_id = $1
ORDER BY created_at, id;
