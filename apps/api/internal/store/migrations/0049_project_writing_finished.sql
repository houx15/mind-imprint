-- +goose Up
-- Slice 5 (#20) · the 完成写作 milestone. A two-stage 写作→回顾 flow: clicking
-- 完成写作 locks the draft read-only and unlocks the 回顾 room. This is a separate
-- timestamp — NOT a new project.status value — so the existing lifecycle enum
-- (active/evaluating/finished) is untouched and every existing row stays valid
-- (nullable, no default → NULL = writing not yet finished). Reversible below.
ALTER TABLE project ADD COLUMN writing_finished_at timestamptz;

-- +goose Down
ALTER TABLE project DROP COLUMN writing_finished_at;
