-- +goose Up
ALTER TABLE evaluation_report ALTER COLUMN report DROP NOT NULL;
ALTER TABLE evaluation_report ADD COLUMN status text NOT NULL DEFAULT 'ready'
  CHECK (status IN ('generating','ready','failed'));
DROP INDEX IF EXISTS evaluation_report_project_created_idx;
ALTER TABLE evaluation_report ADD CONSTRAINT evaluation_report_project_key UNIQUE (project_id);

-- +goose Down
ALTER TABLE evaluation_report DROP CONSTRAINT evaluation_report_project_key;
CREATE INDEX evaluation_report_project_created_idx ON evaluation_report (project_id, created_at DESC);
ALTER TABLE evaluation_report DROP COLUMN status;
-- in-flight rows (generating/failed) have a NULL report; drop them before
-- restoring the NOT NULL constraint so the rollback can't fail on live data.
DELETE FROM evaluation_report WHERE report IS NULL;
ALTER TABLE evaluation_report ALTER COLUMN report SET NOT NULL;
