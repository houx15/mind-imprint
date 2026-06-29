-- +goose Up
ALTER TABLE evaluations ADD COLUMN trigger text NOT NULL DEFAULT 'manual';
ALTER TABLE evaluations ADD COLUMN trigger_milestone integer;
CREATE UNIQUE INDEX evaluations_one_inflight_per_task
    ON evaluations (task_id) WHERE status IN ('queued','running');

-- +goose Down
DROP INDEX IF EXISTS evaluations_one_inflight_per_task;
ALTER TABLE evaluations DROP COLUMN trigger_milestone;
ALTER TABLE evaluations DROP COLUMN trigger;
