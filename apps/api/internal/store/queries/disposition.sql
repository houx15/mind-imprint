-- Three-key disposition (product spec §7.1 rule 6): accept / reject /
-- rewrite an intervention, with a reason. Reason-length enforcement
-- (>=15 chars) lives in the caller (agent.RecordDisposition), not here.

-- name: InsertDisposition :one
INSERT INTO disposition (intervention_id, action, reason)
VALUES ($1, $2, $3)
RETURNING *;

-- name: ListDispositionsByProject :many
SELECT d.* FROM disposition d
JOIN intervention i ON i.id = d.intervention_id
WHERE i.project_id = $1
ORDER BY d.created_at;
