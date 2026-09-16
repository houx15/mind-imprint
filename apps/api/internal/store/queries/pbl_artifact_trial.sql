-- name: EnsurePblArtifactTrialDraft :exec
INSERT INTO pbl_artifact_trial_draft (artifact_id) VALUES ($1) ON CONFLICT DO NOTHING;

-- name: GetPblArtifactTrialDraft :one
SELECT * FROM pbl_artifact_trial_draft WHERE artifact_id = $1;

-- name: SavePblArtifactTrialDraft :one
UPDATE pbl_artifact_trial_draft SET document = $2, revision = revision + 1, updated_at = now()
WHERE artifact_id = $1 AND revision = $3 RETURNING *;

-- name: SubmitPblArtifactTrialDraft :one
UPDATE pbl_artifact_trial_draft
SET document = $2, submitted_revision = revision, submitted_entry_id = $3, revision = revision + 1, updated_at = now()
WHERE artifact_id = $1 AND revision = $4 RETURNING *;

-- name: GetPblTrialSubmittedEntry :one
SELECT * FROM pbl_keep_entry WHERE id = $1 AND atom_id = $2;
