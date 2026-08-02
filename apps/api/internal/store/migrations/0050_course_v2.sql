-- +goose Up
-- Course v2: retire the phase-gated runtime. Drop the authored-step tables and
-- the session/message tables; reshape `course` to hold the externally-authored
-- structure + published render cache verbatim (+ attached tool cards); add
-- time-spent bookkeeping to course_progress. course.id stays uuid so the
-- event.course_id FK (0031) survives; a new slug is the external identifier.

DROP TABLE IF EXISTS course_message;

-- course_session is also referenced by: material.session_id, card_instances.session_id,
-- event.session_id, evaluations.session_id (FKs from 0023/0024) and the
-- `student_evaluation` view (0032, via a `course_session cs` LEFT JOIN for the
-- 'course' scope). CASCADE drops those FK constraints (the columns themselves
-- stay — now orphaned uuids with no referential integrity, acceptable per "no
-- back-compat") and the view; the view is immediately recreated below without
-- the course-session scope so the teacher/report queries that still rely on
-- it (project + chat scopes) keep working.
DROP TABLE IF EXISTS course_session CASCADE;

-- The CASCADE above already drops the view when course_session (created way
-- back at 0023) is still around to cascade from. But some test harnesses
-- goose-Down past 0050 and then goose-Up again WITHOUT crossing 0032 (e.g. a
-- DownTo(33)/Up round trip) — in that path course_session was never
-- recreated (by design, no back-compat) so the CASCADE above is a no-op, and
-- the view recreated by an earlier application of 0050 is still sitting
-- there. DROP IF EXISTS first makes this block replay-safe either way.
DROP VIEW IF EXISTS student_evaluation;

CREATE VIEW student_evaluation AS
SELECT
  e.id,
  COALESCE(p.user_id, t.user_id)::uuid                          AS user_id,
  e.scores,
  e.created_at,
  (CASE WHEN e.project_id IS NOT NULL THEN 'project'
        ELSE 'chat' END)::text                                  AS surface,
  COALESCE(e.project_id, e.thread_id)::uuid                      AS scope_id
FROM evaluations e
LEFT JOIN project     p ON p.id = e.project_id
LEFT JOIN chat_thread t ON t.id = e.thread_id
WHERE COALESCE(p.user_id, t.user_id) IS NOT NULL;

DROP TABLE IF EXISTS course_step_render;
DROP TABLE IF EXISTS course_step;

-- Remove old seeded rows (0012). event.course_id ON DELETE CASCADE cleans up
-- any course-scoped events; course_progress rows cascade too.
DELETE FROM course;

ALTER TABLE course DROP COLUMN IF EXISTS tasks_count;
ALTER TABLE course DROP COLUMN IF EXISTS tools_count;
ALTER TABLE course ADD COLUMN slug         text NOT NULL DEFAULT '';
ALTER TABLE course ADD COLUMN card_ids     text[] NOT NULL DEFAULT '{}';
ALTER TABLE course ADD COLUMN structure    jsonb NOT NULL DEFAULT '{}';
ALTER TABLE course ADD COLUMN render_cache jsonb NOT NULL DEFAULT '{}';
ALTER TABLE course ADD COLUMN step_count   int  NOT NULL DEFAULT 0;
ALTER TABLE course ADD COLUMN updated_at   timestamptz NOT NULL DEFAULT now();
CREATE UNIQUE INDEX course_slug_uidx ON course (slug);

ALTER TABLE course_progress ADD COLUMN started_at   timestamptz;
ALTER TABLE course_progress ADD COLUMN completed_at timestamptz;

-- +goose Down
ALTER TABLE course_progress DROP COLUMN IF EXISTS completed_at;
ALTER TABLE course_progress DROP COLUMN IF EXISTS started_at;
DROP INDEX IF EXISTS course_slug_uidx;
ALTER TABLE course DROP COLUMN IF EXISTS updated_at;
ALTER TABLE course DROP COLUMN IF EXISTS step_count;
ALTER TABLE course DROP COLUMN IF EXISTS render_cache;
ALTER TABLE course DROP COLUMN IF EXISTS structure;
ALTER TABLE course DROP COLUMN IF EXISTS card_ids;
ALTER TABLE course DROP COLUMN IF EXISTS slug;
ALTER TABLE course ADD COLUMN tools_count int NOT NULL DEFAULT 0;
ALTER TABLE course ADD COLUMN tasks_count int NOT NULL DEFAULT 0;
-- NB: the dropped step/session tables are not recreated on Down (no back-compat).
