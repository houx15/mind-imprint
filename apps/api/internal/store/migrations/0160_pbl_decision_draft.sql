-- +goose Up
ALTER TABLE pbl_decision
  ADD COLUMN draft jsonb NOT NULL DEFAULT '{}'::jsonb,
  ADD COLUMN draft_revision integer NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE pbl_decision DROP COLUMN draft, DROP COLUMN draft_revision;
