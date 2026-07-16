-- +goose Up
-- Slice 10 T2: project-scoped assessment storage.
--
-- evaluations.project_id already exists (added nullable, FK project(id)
-- ON DELETE CASCADE, by migration 0016_refactor2_foundations.sql, alongside
-- rubric/leaps). This migration finishes the scoping: task_id becomes
-- optional and a CHECK enforces every row is bound to at least one of
-- task_id/project_id — no unscoped evaluation rows.
ALTER TABLE evaluations ALTER COLUMN task_id DROP NOT NULL;
ALTER TABLE evaluations ADD CONSTRAINT evaluations_scope_ck
  CHECK (task_id IS NOT NULL OR project_id IS NOT NULL);
CREATE INDEX evaluations_project_idx ON evaluations (project_id, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS evaluations_project_idx;
ALTER TABLE evaluations DROP CONSTRAINT IF EXISTS evaluations_scope_ck;
ALTER TABLE evaluations ALTER COLUMN task_id SET NOT NULL;
