-- +goose Up
-- D2: give the event stream a course scope, so a course page-turn can be
-- recorded as a timestamped event. Until now course progress lived ONLY in
-- course_progress.completed_ordinals — a mutable int array with no timestamp —
-- so "course steps completed this week" was unqueryable. Additive, mirroring
-- 0024's session scope and 0025's thread scope.
ALTER TABLE event ADD COLUMN course_id uuid REFERENCES course(id) ON DELETE CASCADE;
CREATE INDEX event_course_created_idx ON event (course_id, created_at);

-- Widen the scope check. Still NOT VALID for the same reason 0024/0025 gave:
-- pre-existing unscoped rows have nothing to backfill from and stay
-- grandfathered, while every new insert is enforced.
ALTER TABLE event DROP CONSTRAINT event_scope_ck;
ALTER TABLE event ADD CONSTRAINT event_scope_ck
  CHECK (num_nonnulls(project_id, session_id, thread_id, course_id) >= 1) NOT VALID;

-- +goose Down
-- Course-only-scoped rows have no other scope; the restored narrower CHECK
-- would reject them and abort the whole Down exactly where a step_viewed
-- exists. Delete them explicitly (mirrors 0025's Down for thread-scoped rows).
ALTER TABLE event DROP CONSTRAINT IF EXISTS event_scope_ck;
DELETE FROM event
 WHERE course_id IS NOT NULL AND project_id IS NULL AND session_id IS NULL AND thread_id IS NULL;
ALTER TABLE event ADD CONSTRAINT event_scope_ck
  CHECK (num_nonnulls(project_id, session_id, thread_id) >= 1) NOT VALID;
DROP INDEX IF EXISTS event_course_created_idx;
ALTER TABLE event DROP COLUMN IF EXISTS course_id;
