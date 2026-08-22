-- +goose Up
-- Attempt-log model for the 2.0 runtime. A course can now be RELEARNED, and each
-- finished run is kept as its own frozen record — with its own report, which is
-- computed on the fly from that attempt's session blob (there is no report table,
-- so keeping the attempt's session IS keeping its report). Two changes:
--
--   1. Drop UNIQUE(user_id, course_id) so a (user, course) pair can hold multiple
--      attempts. The "current" attempt is the most recent row by created_at; every
--      read that used to key by (user, course) now takes the latest one.
--   2. Add completed_at — the FROZEN completion timestamp, written once the first
--      time a session reaches status='completed' (SaveCourseSession). The learning
--      history shows completed_at for a finished attempt (fixed) and updated_at for
--      one still in progress (bumps as the student works).
--
-- Only ADD COLUMN + DROP CONSTRAINT + CREATE INDEX on existing rows — safe on the
-- live table. The course DEFINITION is untouched (no hash change), so in-progress
-- sessions are NOT reset by this migration.
ALTER TABLE course_session DROP CONSTRAINT IF EXISTS course_session_user_id_course_id_key;
ALTER TABLE course_session ADD COLUMN IF NOT EXISTS completed_at timestamptz;

-- Backfill: give already-finished sessions a stable history date from now on.
-- Their last-activity time is the best completion proxy we have (there was no
-- completion timestamp before this migration).
UPDATE course_session SET completed_at = updated_at WHERE status = 'completed' AND completed_at IS NULL;

-- Latest-attempt lookups (resume get-or-create, catalog ring, report) key by
-- (user, course) newest-first.
CREATE INDEX IF NOT EXISTS course_session_user_course_created_idx
  ON course_session(user_id, course_id, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS course_session_user_course_created_idx;
ALTER TABLE course_session DROP COLUMN IF EXISTS completed_at;
-- NB: re-adding UNIQUE(user_id, course_id) fails if multiple attempts already
-- exist. This Down is only safe to run when at most one attempt per (user,
-- course) remains — i.e. right after Up on data that had no relearns yet.
ALTER TABLE course_session ADD CONSTRAINT course_session_user_id_course_id_key UNIQUE (user_id, course_id);
