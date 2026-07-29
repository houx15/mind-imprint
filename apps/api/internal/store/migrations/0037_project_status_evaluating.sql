-- +goose Up
-- BE5: widen project.status to carry the in-flight terminal state. The finish
-- endpoint is now non-blocking — it flips status to 'evaluating', returns 202,
-- and a detached goroutine generates the flagship report, only then marking
-- 'finished' (or rolling back to 'active' on failure). 0026 closed status to
-- ('active','finished'); re-open it to admit 'evaluating' in between. Every
-- existing row is one of the old three values, so the re-added CHECK validates
-- cleanly.
ALTER TABLE project DROP CONSTRAINT IF EXISTS project_status_ck;
ALTER TABLE project ADD CONSTRAINT project_status_ck
  CHECK (status IN ('active', 'evaluating', 'finished'));

-- +goose Down
ALTER TABLE project DROP CONSTRAINT IF EXISTS project_status_ck;
ALTER TABLE project ADD CONSTRAINT project_status_ck
  CHECK (status IN ('active', 'finished'));
