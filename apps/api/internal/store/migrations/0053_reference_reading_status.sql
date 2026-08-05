-- One shelf, three states (Task A1): the 图书馆 collapses "待读/在读/读完" onto
-- a single status column on reference instead of separate lists/collections.
-- NOT NULL DEFAULT 'to_read' so every existing/new row lands in 待读 without a
-- backfill; CHECK constrains it to the closed three-value set (no freeform).

-- +goose Up
ALTER TABLE reference ADD COLUMN reading_status text NOT NULL DEFAULT 'to_read'
    CHECK (reading_status IN ('to_read','reading','done'));

-- +goose Down
ALTER TABLE reference DROP COLUMN reading_status;
