-- name: GetProjectProposal :one
SELECT * FROM project_proposal WHERE project_id = $1;

-- name: UpsertProjectProposal :one
INSERT INTO project_proposal (project_id, objective, reason, activities, resources, updated_at)
VALUES ($1, $2, $3, $4, $5, now())
ON CONFLICT (project_id) DO UPDATE SET
    objective  = EXCLUDED.objective,
    reason     = EXCLUDED.reason,
    activities = EXCLUDED.activities,
    resources  = EXCLUDED.resources,
    updated_at = now()
RETURNING *;

-- name: ListPlanItems :many
SELECT * FROM plan_item
WHERE project_id = $1
ORDER BY stage, position, start_day, created_at;

-- name: CreatePlanItem :one
INSERT INTO plan_item (project_id, title, tag, col, stage, ref_material_id, start_day, days, position)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: GetPlanItem :one
SELECT * FROM plan_item WHERE id = $1 AND project_id = $2;

-- name: UpdatePlanItem :one
-- Sets ALL columns by id + project_id; the handler merges partial patches over
-- the current row before calling this, so every field is always supplied.
UPDATE plan_item SET
    title           = $3,
    tag             = $4,
    col             = $5,
    stage           = $6,
    ref_material_id = $7,
    start_day       = $8,
    days            = $9,
    position        = $10,
    updated_at      = now()
WHERE id = $1 AND project_id = $2
RETURNING *;

-- name: DeletePlanItem :exec
DELETE FROM plan_item WHERE id = $1 AND project_id = $2;

-- name: ListActivityLog :many
SELECT * FROM activity_log_entry
WHERE project_id = $1
ORDER BY entry_date, created_at;

-- name: CreateActivityLogEntry :one
INSERT INTO activity_log_entry (project_id, entry_date, text, source)
VALUES ($1, $2, $3, $4)
RETURNING *;
