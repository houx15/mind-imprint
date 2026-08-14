-- name: InsertIntervention :one
INSERT INTO intervention (project_id, card_instance_id, type, anchor, criterion, body, level, output_check_verdict)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: ListInterventionsByProject :many
SELECT * FROM intervention
WHERE project_id = $1
ORDER BY created_at, id;

-- name: ListReviewItemsByProject :many
SELECT * FROM intervention
WHERE project_id = $1 AND type = 'review_item'
ORDER BY created_at;

-- name: ListProposalAnnotations :many
SELECT * FROM intervention
WHERE project_id = $1 AND type = 'proposal_annotation'
ORDER BY created_at, id;

-- name: DeleteProposalAnnotations :exec
DELETE FROM intervention
WHERE project_id = $1 AND type = 'proposal_annotation';

-- name: ListAnnotationsByType :many
SELECT * FROM intervention
WHERE project_id = $1 AND type = $2
ORDER BY created_at, id;

-- name: DeleteAnnotationsByType :exec
DELETE FROM intervention
WHERE project_id = $1 AND type = $2;

-- name: CountAnnotationsByProject :one
-- G2 · counters.aiCommentCount = every AI writing 批注 (proposal + essay).
SELECT COUNT(*) FROM intervention
WHERE project_id = $1 AND type IN ('proposal_annotation', 'essay_annotation');
