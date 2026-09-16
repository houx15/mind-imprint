-- name: CreatePblCodeVersion :one
INSERT INTO pbl_code_version(atom_id,brief_revision,brief,html,parent_version_id,feedback) VALUES($1,$2,$3,$4,$5,$6) RETURNING *;
-- name: ListPblCodeVersions :many
SELECT id,atom_id,brief_revision,brief,created_at,parent_version_id,feedback FROM pbl_code_version WHERE atom_id=$1 ORDER BY created_at DESC,id DESC LIMIT 50;
-- name: GetPblCodeVersion :one
SELECT * FROM pbl_code_version WHERE atom_id=$1 AND id=$2;
