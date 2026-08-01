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

-- name: DeletePlanItemsByProject :exec
-- Regenerating the plan replaces it wholesale (#15): clear the whole board
-- before inserting the freshly generated tasks, in one transaction.
DELETE FROM plan_item WHERE project_id = $1;

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
    reading_note   = $16,
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

-- name: GetReferenceForProject :one
-- Scope a reference id to its project (the IDOR guard for the reading-brief /
-- takeaway endpoints, mirroring patchReference's ownership scoping).
SELECT * FROM reference WHERE id = $1 AND project_id = $2;

-- name: UpdateReadingBrief :one
-- Brief-in: persist why-read-this + optional focus + phase_tag on the source.
-- Editable any time; does not touch the takeaway.
UPDATE reference
SET reading_reason = $3, reading_focus = $4, phase_tag = $5, updated_at = now()
WHERE id = $1 AND project_id = $2
RETURNING *;

-- name: FinalizeReadingTakeaway :one
-- Takeaways-out: land the compact 5-field object and stamp finalized_at.
-- UPDATE (not insert) — re-finalize on a re-read SUPERSEDES (§1, deliberately
-- unlike S1's first-open-wins proposal prose; reading is iterative).
UPDATE reference
SET takeaway = $3, takeaway_finalized_at = now(), updated_at = now()
WHERE id = $1 AND project_id = $2
RETURNING *;

-- Write · outline nodes (depth-indexed flat list, projected to a tree). ------

-- name: ListOutlineNodes :many
SELECT * FROM outline_node
WHERE project_id = $1
ORDER BY position, created_at;

-- name: DeleteAllOutlineNodes :exec
-- Clears the whole outline for a project; PUT /outline replaces the set by
-- deleting then re-inserting the posted array in one transaction.
DELETE FROM outline_node WHERE project_id = $1;

-- name: CreateOutlineNode :one
INSERT INTO outline_node (project_id, text, depth, position)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- Write · snippets (片段: flat, ordered text fragments). -----------------------

-- name: ListSnippets :many
SELECT * FROM snippet
WHERE project_id = $1
ORDER BY position, created_at;

-- name: DeleteAllSnippets :exec
-- PUT /snippets replaces the whole set (delete + re-insert the posted array).
DELETE FROM snippet WHERE project_id = $1;

-- name: CreateSnippet :one
INSERT INTO snippet (project_id, text, position, section)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- Review · the five-dimension reflection doc (answers jsonb string array). ----

-- name: GetProjectAIUse :one
-- S5 · the student's AI-use statement (回顾 · 复盘我与 AI 的互动).
SELECT * FROM project_ai_use WHERE project_id = $1;

-- name: UpsertProjectAIUse :exec
INSERT INTO project_ai_use (project_id, used_for, not_used_for, updated_at)
VALUES ($1, $2, $3, now())
ON CONFLICT (project_id) DO UPDATE SET
    used_for     = EXCLUDED.used_for,
    not_used_for = EXCLUDED.not_used_for,
    updated_at   = now();

-- name: GetProjectReflection :one
SELECT * FROM project_reflection WHERE project_id = $1;

-- name: UpsertProjectReflection :one
INSERT INTO project_reflection (project_id, answers, done, updated_at)
VALUES ($1, $2, $3, now())
ON CONFLICT (project_id) DO UPDATE SET
    answers    = EXCLUDED.answers,
    done       = EXCLUDED.done,
    updated_at = now()
RETURNING *;

-- Review · the 你的思维印记 mirror prose (first-open-wins). -------------------

-- name: GetProjectMirror :one
SELECT * FROM project_mirror_prose WHERE project_id = $1;

-- name: InsertProjectMirror :exec
-- First-open-wins: the first composer to insert a row wins; a concurrent loser's
-- INSERT is a no-op (ON CONFLICT DO NOTHING) and it re-reads the winner's row.
INSERT INTO project_mirror_prose (project_id, sections, carry_forwards, model, tier)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (project_id) DO NOTHING;

-- S1 · summary-on-return prose (first-open-wins, same pattern as the mirror). --

-- name: GetProjectSummaryProse :one
SELECT * FROM project_summary_prose WHERE project_id = $1;

-- name: InsertProjectSummaryProse :exec
-- First composer wins; a concurrent loser's INSERT is a no-op and it re-reads
-- the winner's row. A failed compose is never inserted (retries next open).
INSERT INTO project_summary_prose (project_id, prose, model, tier)
VALUES ($1, $2, $3, $4)
ON CONFLICT (project_id) DO NOTHING;

-- S3 · rabbit-hole exploration leads (branch off reading takeaways). ---------

-- name: ListExplorationLeads :many
SELECT * FROM exploration_lead
WHERE project_id = $1
ORDER BY position, created_at;

-- name: CreateExplorationLead :one
INSERT INTO exploration_lead (
    project_id, text, status, origin, source_reference_id, connected_reference_id, position
) VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetExplorationLeadForProject :one
-- IDOR guard, mirrors GetReferenceForProject.
SELECT * FROM exploration_lead WHERE id = $1 AND project_id = $2;

-- name: UpdateExplorationLead :one
-- Sets text, status, connected_reference_id, position by id + project_id; the
-- handler merges partial patches over the current row first (UpdatePlanItem's
-- pattern).
UPDATE exploration_lead SET
    text                   = $3,
    status                 = $4,
    connected_reference_id = $5,
    position               = $6,
    updated_at             = now()
WHERE id = $1 AND project_id = $2
RETURNING *;

-- name: DeleteExplorationLead :exec
DELETE FROM exploration_lead WHERE id = $1 AND project_id = $2;

-- name: CountExplorationLeadForSource :one
-- The idempotent-materialize dedupe check for postFinalizeReading (D-S3-2):
-- re-finalizing a source must not duplicate a lead whose text already exists
-- for that (project, source).
SELECT count(*) FROM exploration_lead
WHERE project_id = $1 AND source_reference_id = $2 AND text = $3;
