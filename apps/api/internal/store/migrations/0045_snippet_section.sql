-- +goose Up
-- #5 · nest 片段 under an outline section (or a 线索). `section` holds the label a
-- snippet is filed under — the TEXT of a top-level outline heading, or a 线索's
-- text (both are just string labels; outline node ids are re-minted on every PUT
-- so a stable id can't be referenced). NULL = 未归类 (uncategorized). Whole-set
-- replace like the rest of the snippet board carries it.
ALTER TABLE snippet ADD COLUMN section text;

-- +goose Down
ALTER TABLE snippet DROP COLUMN section;
