-- +goose Up
-- Backfill step_count for already-published 2.0 courses. A 2.0 course stores its
-- content in course_definition (jsonb: { schemaVersion, course{ parts[]{ slices[] }}})
-- and, until now, the publish path hardcoded step_count = 0 — so every 2.0 course
-- read "0 步" on the catalog and could not compute a completion percentage. One
-- slice = one step; the count is the sum of slices across every part. Legacy
-- courses (course_definition NULL/'{}') keep their own step_count untouched.
-- +goose StatementBegin
UPDATE course c
SET step_count = sub.n
FROM (
  SELECT id,
    (SELECT COALESCE(SUM(jsonb_array_length(part->'slices')), 0)
     FROM jsonb_array_elements(course_definition->'course'->'parts') AS part
     WHERE jsonb_typeof(part->'slices') = 'array') AS n
  FROM course
  WHERE course_definition IS NOT NULL
    AND jsonb_typeof(course_definition->'course'->'parts') = 'array'
) sub
WHERE c.id = sub.id
  AND c.step_count <> sub.n;
-- +goose StatementEnd

-- +goose Down
-- No-op: step_count is a derived denormalization with no prior 2.0 value to
-- restore (it was uniformly 0). Reverting would re-introduce the "0 步" bug.
SELECT 1;
