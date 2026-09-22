-- +goose Up
-- Students close a plan step by attaching the thing they actually produced.
-- Text and links cover work completed in this product or in an external tool;
-- the append-only rows retain earlier submissions when a step is revised.
CREATE TABLE pbl_step_submission (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  step_id      uuid NOT NULL REFERENCES pbl_plan_step(id) ON DELETE CASCADE,
  note         text NOT NULL DEFAULT '',
  url          text NOT NULL DEFAULT '',
  confirmed_at timestamptz NOT NULL DEFAULT now(),
  CHECK (length(btrim(note)) > 0 OR length(btrim(url)) > 0)
);

CREATE INDEX pbl_step_submission_step_idx
  ON pbl_step_submission (step_id, confirmed_at DESC);

-- +goose Down
DROP TABLE pbl_step_submission;
