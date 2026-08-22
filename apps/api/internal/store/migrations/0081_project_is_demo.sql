-- +goose Up
ALTER TABLE project ADD COLUMN is_demo boolean NOT NULL DEFAULT false;
-- The canonical demo project gets a NEW dedicated id
-- (00000000-0000-0000-0000-000000000200), inserted + flagged is_demo in a
-- later migration (0082) — NOT the pre-existing, heavily-reused test fixture
-- project ...0101 (~10 test files write to it).

-- +goose Down
ALTER TABLE project DROP COLUMN is_demo;
