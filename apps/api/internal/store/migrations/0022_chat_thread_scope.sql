-- +goose Up
-- Slice 11: give Chat's card runtime a thread-scoped home. Additive, mirrors
-- 0021's evaluations scope pattern. A material / card_instance belongs to
-- exactly one owner: a project, a chat thread, or a legacy task. intervention
-- is intentionally NOT touched (DEC-11.2: the coach reply is a chat_message,
-- not an anchored intervention; the CRAAP card completes without one).
ALTER TABLE material       ADD COLUMN thread_id uuid REFERENCES chat_thread(id) ON DELETE CASCADE;
ALTER TABLE card_instances ADD COLUMN thread_id uuid REFERENCES chat_thread(id) ON DELETE CASCADE;

CREATE INDEX material_thread_created_idx       ON material (thread_id, created_at);
CREATE INDEX card_instances_thread_created_idx ON card_instances (thread_id, created_at);

-- Scope discipline: at least one owner set. Existing rows (project_id or
-- task_id set, thread_id NULL) satisfy these unchanged; a chat row sets
-- thread_id only.
ALTER TABLE material       ADD CONSTRAINT material_scope_ck
  CHECK (num_nonnulls(task_id, project_id, thread_id) >= 1);
ALTER TABLE card_instances ADD CONSTRAINT card_instances_scope_ck
  CHECK (num_nonnulls(task_id, project_id, thread_id) >= 1);

-- +goose Down
ALTER TABLE material       DROP CONSTRAINT IF EXISTS material_scope_ck;
ALTER TABLE card_instances DROP CONSTRAINT IF EXISTS card_instances_scope_ck;
DROP INDEX IF EXISTS material_thread_created_idx;
DROP INDEX IF EXISTS card_instances_thread_created_idx;
ALTER TABLE material       DROP COLUMN IF EXISTS thread_id;
ALTER TABLE card_instances DROP COLUMN IF EXISTS thread_id;
