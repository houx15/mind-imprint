-- Course v2 (migration 0050): the phase-gated runtime (course_session/
-- course_message/course_step/course_step_render) is retired. `course` now
-- holds the externally-authored structure + published render cache verbatim
-- (+ attached tool card ids), addressed by a stable slug; course.id stays a
-- uuid so the pre-existing event.course_id FK (0031) survives. course_progress
-- stays the page-position unit, now with started_at/completed_at bookkeeping.

-- name: ListCourseRows :many
SELECT slug, branch, title, blurb, time_label, card_ids, step_count
FROM course ORDER BY branch, title;

-- name: GetCourseBySlug :one
SELECT id, slug, branch, title, blurb, time_label, card_ids, step_count, structure, render_cache
FROM course WHERE slug = $1;

-- name: UpsertCourse :one
INSERT INTO course (slug, branch, title, blurb, time_label, card_ids, step_count, structure, render_cache, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9, now())
ON CONFLICT (slug) DO UPDATE SET
  branch = EXCLUDED.branch, title = EXCLUDED.title, blurb = EXCLUDED.blurb,
  time_label = EXCLUDED.time_label, card_ids = EXCLUDED.card_ids,
  step_count = EXCLUDED.step_count, structure = EXCLUDED.structure,
  render_cache = EXCLUDED.render_cache, updated_at = now()
RETURNING id, slug;

-- name: GetCourseProgressBySlug :one
SELECT p.course_id, p.current_ordinal, p.completed_ordinals, p.started_at, p.completed_at, p.updated_at
FROM course_progress p JOIN course c ON c.id = p.course_id
WHERE p.user_id = $1 AND c.slug = $2;

-- name: UpsertCourseProgress :one
INSERT INTO course_progress (user_id, course_id, current_ordinal, completed_ordinals, started_at, completed_at, updated_at)
VALUES ($1,$2,$3,$4, COALESCE($5, now()), $6, now())
ON CONFLICT (user_id, course_id) DO UPDATE SET
  current_ordinal = EXCLUDED.current_ordinal,
  completed_ordinals = EXCLUDED.completed_ordinals,
  started_at = COALESCE(course_progress.started_at, EXCLUDED.started_at),
  completed_at = COALESCE(EXCLUDED.completed_at, course_progress.completed_at),
  updated_at = now()
RETURNING course_id, current_ordinal, completed_ordinals, started_at, completed_at, updated_at;
