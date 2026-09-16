-- +goose Up
-- Unsubmitted work must not enter keep entries or evidence/coach context.
CREATE TABLE pbl_artifact_trial_draft (
  artifact_id uuid PRIMARY KEY REFERENCES pbl_artifact(id) ON DELETE CASCADE,
  document jsonb NOT NULL DEFAULT '{"mode":"self","version":"","task":"","expected":"","actual":"","next":""}'::jsonb,
  revision integer NOT NULL DEFAULT 0,
  submitted_revision integer,
  submitted_entry_id uuid REFERENCES pbl_keep_entry(id),
  updated_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE pbl_artifact_trial_draft;
