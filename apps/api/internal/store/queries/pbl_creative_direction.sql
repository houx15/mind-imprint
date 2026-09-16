-- name: GetPblCreativeDirection :one
SELECT * FROM pbl_creative_direction WHERE atom_id = $1;

-- name: EnsurePblCreativeDirection :exec
INSERT INTO pbl_creative_direction (atom_id) VALUES ($1) ON CONFLICT DO NOTHING;

-- name: SavePblCreativeDirection :one
UPDATE pbl_creative_direction SET document = $2, revision = revision + 1, updated_at = now()
WHERE atom_id = $1 AND revision = $3 RETURNING *;
