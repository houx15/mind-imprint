-- Three-key disposition (product spec §7.1 rule 6): accept / reject /
-- rewrite an intervention, with a reason. Reason-length enforcement
-- (>=15 chars) lives in the caller (agent.RecordDisposition), not here.

-- name: InsertDisposition :one
INSERT INTO disposition (intervention_id, action, reason)
VALUES ($1, $2, $3)
RETURNING *;
