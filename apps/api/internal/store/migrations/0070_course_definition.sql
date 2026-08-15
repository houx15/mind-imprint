-- +goose Up
-- Course Runtime Slice 8: CourseDefinition 2.0 storage. The 2.0 runtime consumes
-- a whole CourseDefinition document ({ schemaVersion, course }) — a shape the
-- pre-rendered `structure`/`render_cache` columns do not hold. A nullable jsonb
-- column carries it alongside the legacy content: a course WITH a definition
-- plays through the new runtime player, one WITHOUT (every legacy authored
-- course) still falls back to the pre-rendered player. Nullable so existing rows
-- and the admin/legacy seed path need no change.
ALTER TABLE course ADD COLUMN course_definition jsonb;

-- +goose Down
ALTER TABLE course DROP COLUMN IF EXISTS course_definition;
