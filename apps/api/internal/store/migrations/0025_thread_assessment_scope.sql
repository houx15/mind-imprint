-- +goose Up
-- A2: give the event stream and evaluations a chat-thread scope, so chat
-- evidence stops being unreachable and A1's temporary surface='chat' exemption
-- in event_scope_ck can be closed. Additive, mirroring 0024's session scope.
ALTER TABLE event       ADD COLUMN thread_id uuid REFERENCES chat_thread(id) ON DELETE CASCADE;
ALTER TABLE evaluations ADD COLUMN thread_id uuid REFERENCES chat_thread(id) ON DELETE CASCADE;

CREATE INDEX event_thread_created_idx ON event (thread_id, created_at);
CREATE INDEX evaluations_thread_idx   ON evaluations (thread_id, created_at DESC);

-- evaluations: ordinary widening — every existing row still satisfies it.
ALTER TABLE evaluations DROP CONSTRAINT evaluations_scope_ck;
ALTER TABLE evaluations ADD CONSTRAINT evaluations_scope_ck
  CHECK (num_nonnulls(task_id, project_id, session_id, thread_id) >= 1);

-- event: close A1's surface='chat' exemption. From here a chat event MUST carry
-- thread_id (all three chat writes now do — migration is paired with that code
-- change in Task 2). Still NOT VALID for the same reason A1 gave: the pre-A2
-- unscoped chat/course rows have nothing to backfill FROM and stay
-- grandfathered, while every new insert — chat included — is enforced.
ALTER TABLE event DROP CONSTRAINT event_scope_ck;
ALTER TABLE event ADD CONSTRAINT event_scope_ck
  CHECK (num_nonnulls(project_id, session_id, thread_id) >= 1) NOT VALID;

-- +goose Down
ALTER TABLE event       DROP CONSTRAINT IF EXISTS event_scope_ck;
ALTER TABLE evaluations DROP CONSTRAINT IF EXISTS evaluations_scope_ck;
-- Thread-scoped reports have no other scope; the restored 0024 CHECK would
-- reject them and abort the whole Down exactly where a chat report exists.
-- Delete them explicitly (mirrors 0024's Down for session-scoped rows).
DELETE FROM evaluations
 WHERE thread_id IS NOT NULL AND task_id IS NULL AND project_id IS NULL AND session_id IS NULL;
ALTER TABLE evaluations ADD CONSTRAINT evaluations_scope_ck
  CHECK (num_nonnulls(task_id, project_id, session_id) >= 1);
-- Restore 0024's event_scope_ck INCLUDING the chat exemption — dropping
-- thread_id reverts chat's writes to unscoped, so the arm must return or the
-- restored constraint would reject every new chat insert.
ALTER TABLE event ADD CONSTRAINT event_scope_ck
  CHECK (surface = 'chat' OR num_nonnulls(project_id, session_id) >= 1) NOT VALID;
DROP INDEX IF EXISTS event_thread_created_idx;
DROP INDEX IF EXISTS evaluations_thread_idx;
ALTER TABLE event       DROP COLUMN IF EXISTS thread_id;
ALTER TABLE evaluations DROP COLUMN IF EXISTS thread_id;
