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

-- Read library · collections (self-referential tree). ---------------------

-- name: ListCollections :many
SELECT * FROM collection
WHERE project_id = $1
ORDER BY position, created_at;

-- name: CreateCollection :one
INSERT INTO collection (project_id, name, parent_id, position)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetCollection :one
SELECT * FROM collection WHERE id = $1 AND project_id = $2;

-- name: UpdateCollection :one
-- Sets ALL columns by id + project_id; the handler merges partial patches over
-- the current row before calling this (UpdatePlanItem's pattern).
UPDATE collection SET
    name      = $3,
    parent_id = $4,
    position  = $5
WHERE id = $1 AND project_id = $2
RETURNING *;

-- name: DeleteCollection :exec
-- Children cascade (parent_id FK ON DELETE CASCADE); a deleted collection's
-- references have their collection_id nulled (reference.collection_id FK ON
-- DELETE SET NULL).
DELETE FROM collection WHERE id = $1 AND project_id = $2;

-- Read library · references. -----------------------------------------------

-- name: ListReferences :many
SELECT * FROM reference
WHERE project_id = $1
ORDER BY position, created_at;

-- name: CreateReference :one
INSERT INTO reference (
    project_id, title, classification, author, credentials, year, url,
    tags, collection_id, credibility, evaluation, decision, pending, search_hints
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
RETURNING *;

-- name: GetReference :one
SELECT * FROM reference WHERE id = $1 AND project_id = $2;

-- name: UpdateReference :one
-- Sets ALL editable columns by id + project_id; the handler merges partial
-- patches over the current row first (UpdatePlanItem's pattern). material_id is
-- NOT set here — SetReferenceMaterial owns that link (enter-reading only).
UPDATE reference SET
    title          = $3,
    classification = $4,
    author         = $5,
    credentials    = $6,
    year           = $7,
    url            = $8,
    tags           = $9,
    collection_id  = $10,
    credibility    = $11,
    evaluation     = $12,
    decision       = $13,
    pending        = $14,
    search_hints   = $15,
    updated_at     = now()
WHERE id = $1 AND project_id = $2
RETURNING *;

-- name: DeleteReference :exec
DELETE FROM reference WHERE id = $1 AND project_id = $2;

-- name: SetReferenceMaterial :one
-- Binds a reference to the material fetched/created for it on enter-reading.
UPDATE reference SET
    material_id = $3,
    updated_at  = now()
WHERE id = $1 AND project_id = $2
RETURNING *;
