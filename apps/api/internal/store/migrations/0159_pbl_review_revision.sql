-- +goose Up
ALTER TABLE pbl_review ADD COLUMN revision integer NOT NULL DEFAULT 1;
ALTER TABLE pbl_review ADD CONSTRAINT pbl_review_revision_positive CHECK (revision > 0);

-- +goose Down
ALTER TABLE pbl_review DROP COLUMN revision;
