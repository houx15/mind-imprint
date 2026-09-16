-- name: GetPblAudienceDocument :one
SELECT * FROM pbl_audience_document WHERE atom_id = $1;

-- name: EnsurePblAudienceDocument :exec
INSERT INTO pbl_audience_document (atom_id) VALUES ($1) ON CONFLICT DO NOTHING;

-- name: SavePblAudienceDocument :one
UPDATE pbl_audience_document SET document = $2, revision = revision + 1, updated_at = now()
WHERE atom_id = $1 AND revision = $3 RETURNING *;
