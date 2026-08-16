-- Course v2 (migration 0050): the phase-gated runtime (course_session/
-- course_message/course_step/course_step_render) is retired. `course` now
-- holds the externally-authored structure + published render cache verbatim
-- (+ attached tool card ids), addressed by a stable slug; course.id stays a
-- uuid so the pre-existing event.course_id FK (0031) survives. course_progress
-- stays the page-position unit, now with started_at/completed_at bookkeeping.

-- name: ListCourseRows :many
-- Preview courses are visible only when include_preview is true (the caller is
-- an admin). Students (false) see 'published' only.
SELECT slug, branch, title, blurb, time_label, card_ids, step_count, status, cover
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
INSERT INTO course (slug, branch, title, blurb, time_label, card_ids, step_count, structure, render_cache, course_definition, status, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,0,'{}','{}',$7,'preview', now())
ON CONFLICT (slug) DO UPDATE SET
  branch = EXCLUDED.branch, title = EXCLUDED.title, blurb = EXCLUDED.blurb,
  time_label = EXCLUDED.time_label, card_ids = EXCLUDED.card_ids,
  course_definition = EXCLUDED.course_definition, updated_at = now()
RETURNING slug, status;

-- name: SetCourseStatusAndCover :exec
-- Empty cover ($3='') preserves the existing cover (NULLIF→NULL→COALESCE) so a
-- re-ship without a cover arg never blanks an already-set cover.
UPDATE course SET status = $2, cover = COALESCE(NULLIF($3, ''), cover), updated_at = now() WHERE slug = $1;

-- name: SetCourseDefinition :exec
-- Course Runtime Slice 8: attach (or replace) one course's CourseDefinition 2.0
-- document. Kept separate from UpsertCourse so the legacy content path (structure
-- /render_cache seed + admin publish) is untouched — only the golden 2.0 seed
-- writes this column.
UPDATE course SET course_definition = $2, updated_at = now() WHERE slug = $1;

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
