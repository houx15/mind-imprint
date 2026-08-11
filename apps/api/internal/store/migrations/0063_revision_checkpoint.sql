-- Revision recording (Mechanism 1): full-content snapshots of the writing text
-- artifacts at ask-feedback / finish / advance moments. Diffed by the evaluator
-- later to derive "what changed" and "what changed after AI feedback".
-- +goose Up
CREATE TABLE revision_checkpoint (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id    uuid NOT NULL REFERENCES project(id) ON DELETE CASCADE,
  artifact_type text NOT NULL,   -- draft | outline | snippets | proposal | claim
  trigger       text NOT NULL,   -- ask_feedback | finish | advance
  content       jsonb NOT NULL,
  content_hash  text NOT NULL,
  feedback_ref  uuid,
  created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX revision_checkpoint_project_artifact_idx
  ON revision_checkpoint (project_id, artifact_type, created_at);

-- +goose Down
DROP TABLE revision_checkpoint;
