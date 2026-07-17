-- +goose Up
-- A1: give the event stream and evaluations a course-session scope, so course
-- evidence stops being unreachable. Additive, mirroring 0023's session scope
-- over material/card_instances.
ALTER TABLE event       ADD COLUMN session_id uuid REFERENCES course_session(id) ON DELETE CASCADE;
ALTER TABLE evaluations ADD COLUMN session_id uuid REFERENCES course_session(id) ON DELETE CASCADE;

CREATE INDEX event_session_created_idx ON event (session_id, created_at);
CREATE INDEX evaluations_session_idx   ON evaluations (session_id, created_at DESC);

-- evaluations: ordinary widening — every existing row has task_id or
-- project_id and satisfies this unchanged.
ALTER TABLE evaluations DROP CONSTRAINT evaluations_scope_ck;
ALTER TABLE evaluations ADD CONSTRAINT evaluations_scope_ck
  CHECK (num_nonnulls(task_id, project_id, session_id) >= 1);

-- event: NOT VALID, deliberately. event.user_id is NOT NULL, so ownership is
-- never at risk here — these columns carry attribution, not ownership. Every
-- chat/course event already stored was written by InsertUserEvent, which
-- recorded no scope at all (project_id NULL, and no thread/session column
-- existed to fill): those rows are permanently unattributable and there is
-- nothing to backfill FROM. NOT VALID enforces every new row while
-- grandfathering the old ones, rather than deleting real records or inventing
-- a scope they never had.
--
-- The `surface = 'chat'` arm is an EXPLICIT, TEMPORARY exemption. Chat has no
-- scope column until A2, and all three of its event writes are best-effort
-- (error swallowed to a slog.Warn — chat.go:195, chat.go:268,
-- chat_step.go:181). Without this arm the constraint would reject every chat
-- event and chat would silently stop recording evidence, with no test failing.
-- A constraint cannot be enforced one slice before its writers have a scope to
-- satisfy it. A2 adds thread_id, scopes chat's writes, and MUST delete this
-- arm — the exemption is written into the schema so it stays louder than the
-- hole it stands in for.
ALTER TABLE event ADD CONSTRAINT event_scope_ck
  CHECK (surface = 'chat' OR num_nonnulls(project_id, session_id) >= 1) NOT VALID;

-- +goose Down
ALTER TABLE event       DROP CONSTRAINT IF EXISTS event_scope_ck;
ALTER TABLE evaluations DROP CONSTRAINT IF EXISTS evaluations_scope_ck;
-- Down necessarily discards session-scoped reports: session_id itself is
-- dropped a few lines below, so these rows have no scope to fall back to and
-- the restored 0021 CHECK (task_id OR project_id) would reject them. Delete
-- them explicitly rather than letting ADD CONSTRAINT abort the whole Down —
-- validation runs against existing rows, so with even one course report
-- present the rollback would fail exactly where it is most needed.
DELETE FROM evaluations
 WHERE session_id IS NOT NULL AND task_id IS NULL AND project_id IS NULL;
ALTER TABLE evaluations ADD CONSTRAINT evaluations_scope_ck
  CHECK (task_id IS NOT NULL OR project_id IS NOT NULL);
DROP INDEX IF EXISTS event_session_created_idx;
DROP INDEX IF EXISTS evaluations_session_idx;
ALTER TABLE event       DROP COLUMN IF EXISTS session_id;
ALTER TABLE evaluations DROP COLUMN IF EXISTS session_id;
