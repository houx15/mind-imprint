-- +goose Up
-- #8 · the student's own freeform note on a source, written in the reading room
-- (我的笔记). Distinct from `evaluation` (a library-metadata credibility field)
-- and from the card-derived read-only `notes[]` projection — this is the
-- student's private working note, editable any time via PATCH /references/{rid}.
ALTER TABLE reference ADD COLUMN reading_note text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE reference DROP COLUMN reading_note;
