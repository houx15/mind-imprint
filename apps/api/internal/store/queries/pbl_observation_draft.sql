-- name: EnsurePblObservationDraft :exec
INSERT INTO pbl_observation_draft(tool_id) VALUES($1) ON CONFLICT DO NOTHING;
-- name: GetPblObservationDraft :one
SELECT * FROM pbl_observation_draft WHERE tool_id=$1;
-- name: SavePblObservationDraft :one
UPDATE pbl_observation_draft SET document=$2,revision=revision+1,updated_at=now()
WHERE tool_id=$1 AND revision=$3 RETURNING *;
-- name: SubmitPblObservationDraft :one
UPDATE pbl_observation_draft SET document='[]',submitted_revision=revision,submitted_notes=$2,revision=revision+1,updated_at=now()
WHERE tool_id=$1 AND revision=$3 RETURNING *;
