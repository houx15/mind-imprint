-- +goose Up
ALTER TABLE pbl_decision
  ADD COLUMN content_version integer NOT NULL DEFAULT 0,
  ADD COLUMN revision_history jsonb NOT NULL DEFAULT '[]';

-- +goose Down
ALTER TABLE pbl_decision DROP COLUMN revision_history, DROP COLUMN content_version;
