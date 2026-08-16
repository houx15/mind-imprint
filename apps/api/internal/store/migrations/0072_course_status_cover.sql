-- +goose Up
-- Course authoring/publish lifecycle: `status` gates catalog/play visibility
-- (rows created via the admin definition-upsert API land as 'preview'; existing
-- and seeded rows default to 'published' so nothing already live is hidden).
-- `cover` is a catalog cover id (e.g. a stock 'img:N'), set at ship time.
ALTER TABLE course
  ADD COLUMN status text NOT NULL DEFAULT 'published'
    CHECK (status IN ('preview','published')),
  ADD COLUMN cover  text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE course DROP COLUMN IF EXISTS cover;
ALTER TABLE course DROP COLUMN IF EXISTS status;
