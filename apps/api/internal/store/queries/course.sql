-- Course v2 (migration 0050): the phase-gated runtime (course_session/
-- course_message/course_step/course_step_render) is retired. `course` now
-- holds the externally-authored structure + published render cache verbatim
-- (+ attached tool card ids), addressed by a stable slug; course.id stays a
-- uuid so the pre-existing event.course_id FK (0031) survives. course_progress
-- stays the page-position unit, now with started_at/completed_at bookkeeping.

-- name: ListCourseRows :many
-- Preview courses are visible only when include_preview is true (the caller is
-- an admin). Students (false) see 'published' only.
SELECT slug, branch, title, blurb, time_label, card_ids, step_count, status, cover,
       category, introduction, featured_rank
FROM course
WHERE status = 'published' OR sqlc.arg(include_preview)::bool
ORDER BY branch, title;

-- name: GetCourseBySlug :one
SELECT id, slug, branch, title, blurb, time_label, card_ids, step_count, structure, render_cache, audio_manifest, status, cover
FROM course WHERE slug = $1;

-- name: UpsertCourse :one
INSERT INTO course (slug, branch, title, blurb, time_label, card_ids, step_count, structure, render_cache, audio_manifest, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10, now())
ON CONFLICT (slug) DO UPDATE SET
  branch = EXCLUDED.branch, title = EXCLUDED.title, blurb = EXCLUDED.blurb,
  time_label = EXCLUDED.time_label, card_ids = EXCLUDED.card_ids,
  step_count = EXCLUDED.step_count, structure = EXCLUDED.structure,
  render_cache = EXCLUDED.render_cache, audio_manifest = EXCLUDED.audio_manifest, updated_at = now()
RETURNING id, slug;

-- name: GetCourseProgressBySlug :one
SELECT p.course_id, p.current_ordinal, p.completed_ordinals, p.started_at, p.completed_at, p.updated_at, p.active_seconds
FROM course_progress p JOIN course c ON c.id = p.course_id
WHERE p.user_id = $1 AND c.slug = $2;

-- name: GetCourseSessionProgressBySlug :one
-- Catalog progress for a 2.0 (runtime) course: its progress lives in
-- course_session (sliceStates: sliceId -> { status }), NOT course_progress, so
-- the catalog's completion ring has to read it from here. Returns the count of
-- completed slices — clamped so a finished session reads exactly step_count
-- (100%) even if the last slice's state lagged the closing-scene flip, and never
-- exceeds step_count. `sliceStates` is a contract-guaranteed object; the
-- jsonb_typeof guard keeps a malformed/legacy blob from erroring the count.
SELECT c.id AS course_id,
       cs.status AS status,
       cs.updated_at AS updated_at,
       CASE WHEN cs.status = 'completed' THEN c.step_count
            WHEN jsonb_typeof(cs.session->'sliceStates') = 'object'
              THEN LEAST((SELECT count(*) FROM jsonb_each(cs.session->'sliceStates') ss
                          WHERE ss.value->>'status' = 'completed')::int, c.step_count)
            ELSE 0 END AS completed_slices
FROM course_session cs JOIN course c ON c.id = cs.course_id
WHERE cs.user_id = $1 AND c.slug = $2;

-- name: GetCourseProgressByCourseID :one
-- Task 4 addition: SaveProgress's union-completed-ordinals step is keyed by
-- courseUUID (not slug) — it already holds the course row's id from
-- GetCoursePayload, and a slug round-trip would be a wasted join. Mirrors
-- GetCourseProgressBySlug's column list/order exactly.
SELECT course_id, current_ordinal, completed_ordinals, started_at, completed_at, updated_at, active_seconds
FROM course_progress
WHERE user_id = $1 AND course_id = $2;

-- name: UpsertCourseProgress :one
-- active_seconds is ADDITIVE: the arg is a delta (the active-focus seconds the
-- client accrued since its last flush), added to the stored total on conflict
-- so time accumulates across visits. On first insert the delta IS the total.
INSERT INTO course_progress (user_id, course_id, current_ordinal, completed_ordinals, started_at, completed_at, active_seconds, updated_at)
VALUES (
  sqlc.arg(user_id), sqlc.arg(course_id), sqlc.arg(current_ordinal), sqlc.arg(completed_ordinals),
  COALESCE(sqlc.narg(started_at)::timestamptz, now()), sqlc.arg(completed_at), sqlc.arg(active_seconds_delta), now()
)
ON CONFLICT (user_id, course_id) DO UPDATE SET
  current_ordinal = EXCLUDED.current_ordinal,
  completed_ordinals = EXCLUDED.completed_ordinals,
  started_at = COALESCE(course_progress.started_at, EXCLUDED.started_at),
  completed_at = COALESCE(EXCLUDED.completed_at, course_progress.completed_at),
  active_seconds = course_progress.active_seconds + EXCLUDED.active_seconds,
  updated_at = now()
RETURNING course_id, current_ordinal, completed_ordinals, started_at, completed_at, updated_at, active_seconds;

-- name: GetCourseDefinition :one
-- Course Runtime Slice 8: the stored CourseDefinition 2.0 document for one
-- course, addressed by slug. NULL (a legacy course with no 2.0 definition) is
-- returned as a nil []byte — the handler treats both "unknown slug" (no row) and
-- "no definition" (NULL) as 404, routing that course to the legacy player.
SELECT course_definition, status FROM course WHERE slug = $1;

-- name: GetCourseStatusBySlug :one
SELECT status FROM course WHERE slug = $1;

-- name: UpsertCourseDefinition :one
-- Course authoring: create/modify a 2.0 course. status is set to 'preview' ONLY
-- on insert (EXCLUDED is not applied on conflict), so re-posting a definition
-- never (un)publishes an existing course. structure/render_cache are the empty
-- object for 2.0 courses (they use course_definition, not the legacy blobs).
INSERT INTO course (slug, branch, title, blurb, time_label, card_ids, step_count, structure, render_cache, course_definition, category, introduction, status, updated_at)
VALUES (sqlc.arg(slug), sqlc.arg(branch), sqlc.arg(title), sqlc.arg(blurb), sqlc.arg(time_label), sqlc.arg(card_ids), sqlc.arg(step_count), '{}','{}', sqlc.arg(course_definition), sqlc.arg(category), sqlc.arg(introduction),'preview', now())
ON CONFLICT (slug) DO UPDATE SET
  branch = EXCLUDED.branch, title = EXCLUDED.title, blurb = EXCLUDED.blurb,
  time_label = EXCLUDED.time_label, card_ids = EXCLUDED.card_ids,
  step_count = EXCLUDED.step_count,
  course_definition = EXCLUDED.course_definition,
  category = EXCLUDED.category, introduction = EXCLUDED.introduction, updated_at = now()
RETURNING slug, status;

-- name: SetCourseStatusAndCover :exec
-- Empty cover ($3='') preserves the existing cover (NULLIF→NULL→COALESCE) so a
-- re-ship without a cover arg never blanks an already-set cover.
UPDATE course SET status = sqlc.arg(status), cover = COALESCE(NULLIF(sqlc.arg(cover)::text, ''), cover), updated_at = now() WHERE slug = sqlc.arg(slug);

-- name: SetCourseDefinition :exec
-- Course Runtime Slice 8: attach (or replace) one course's CourseDefinition 2.0
-- document. Kept separate from UpsertCourse so the legacy content path (structure
-- /render_cache seed + admin publish) is untouched — only the golden 2.0 seed
-- writes this column.
UPDATE course SET course_definition = $2, updated_at = now() WHERE slug = $1;

-- name: DeleteCourseProgress :exec
-- Restart (legacy player): drop the resume position + completed steps so the
-- next visit starts at ordinal 0 with nothing marked done. Idempotent.
DELETE FROM course_progress WHERE user_id = $1 AND course_id = $2;

-- name: ListCourseHistory :many
-- Courses this student has TOUCHED, newest activity first, across BOTH runtime
-- (course_session) and legacy (course_progress) storage. `status` is the
-- runtime session status verbatim (created/opening/in-progress/closing/
-- completed), or 'completed'/'in-progress' for a legacy course. The caller
-- enriches title/cover from the course list, so this query stays cover-signing
-- free. completed_count is the completed-step count for BOTH storages: a runtime
-- course counts its completed slices from course_session.sliceStates (clamped to
-- step_count; a finished session reads full step_count), a legacy course reads
-- course_progress.completed_ordinals — so the history list shows a live ring for
-- 2.0 courses too, not a stuck 0.
SELECT c.slug AS slug, cs.status AS status,
       CASE WHEN cs.status = 'completed' THEN c.step_count
            WHEN jsonb_typeof(cs.session->'sliceStates') = 'object'
              THEN LEAST((SELECT count(*) FROM jsonb_each(cs.session->'sliceStates') ss
                          WHERE ss.value->>'status' = 'completed')::int, c.step_count)
            ELSE 0 END AS completed_count,
       cs.updated_at AS updated_at
FROM course_session cs JOIN course c ON c.id = cs.course_id
WHERE cs.user_id = $1
UNION ALL
SELECT c.slug AS slug,
       CASE WHEN cp.completed_at IS NOT NULL THEN 'completed' ELSE 'in-progress' END AS status,
       COALESCE(array_length(cp.completed_ordinals, 1), 0)::int AS completed_count,
       cp.updated_at AS updated_at
FROM course_progress cp JOIN course c ON c.id = cp.course_id
WHERE cp.user_id = $1
ORDER BY updated_at DESC;

-- name: FinishedCourseIDsByUser :many
-- Task 5 addition: cards_catalog.go's proficiency computation ("which
-- courses has this student finished") needs this and it was dropped by
-- Task 3 with no v2 replacement ("no v2 replacement asked for" — it used to
-- read course_session.status='finished', a table migration 0050 removed).
-- Ported to the v2 schema: a course is finished when course_progress.
-- completed_at is set (SaveProgress sets it exactly when the student's
-- current_ordinal reaches the last authored step — see coursestore.go).
SELECT DISTINCT course_id FROM course_progress
WHERE user_id = $1 AND completed_at IS NOT NULL;
