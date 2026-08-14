-- G3 · source→section citation links (see migration 0068).

-- name: CreateCitation :one
INSERT INTO citation (project_id, reference_id, section)
VALUES ($1, $2, $3)
RETURNING *;

-- name: ListCitationsByProject :many
SELECT * FROM citation
WHERE project_id = $1
ORDER BY created_at, id;
