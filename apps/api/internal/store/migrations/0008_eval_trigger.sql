-- +goose Up
ALTER TABLE evaluations ADD COLUMN trigger text NOT NULL DEFAULT 'manual';
ALTER TABLE evaluations ADD COLUMN trigger_milestone integer;

-- Pre-existing data may have >1 in-flight eval per task (the old manual path had
-- no in-flight guard). Demote all but the most-recent in-flight row per task so
-- the partial unique index below can build. No-op on clean data.
UPDATE evaluations SET status = 'failed', error = '被单评估在途约束取代'
WHERE status IN ('queued','running')
  AND id NOT IN (
    SELECT DISTINCT ON (task_id) id FROM evaluations
    WHERE status IN ('queued','running')
    ORDER BY task_id, created_at DESC
  );

CREATE UNIQUE INDEX evaluations_one_inflight_per_task
    ON evaluations (task_id) WHERE status IN ('queued','running');

-- +goose Down
DROP INDEX IF EXISTS evaluations_one_inflight_per_task;
ALTER TABLE evaluations DROP COLUMN trigger_milestone;
ALTER TABLE evaluations DROP COLUMN trigger;
