-- +goose Up
ALTER TABLE project ADD COLUMN is_demo boolean NOT NULL DEFAULT false;
-- Flag the canonical demo project (seeded content lands in 0082).
UPDATE project SET is_demo = true WHERE id = '00000000-0000-0000-0000-000000000101';

-- +goose Down
ALTER TABLE project DROP COLUMN is_demo;
