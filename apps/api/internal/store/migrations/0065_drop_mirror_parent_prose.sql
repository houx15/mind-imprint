-- +goose Up
DROP TABLE IF EXISTS project_mirror_prose;
DROP TABLE IF EXISTS parent_report_prose;

-- +goose Down
-- (no-op: these tables are retired; recreate from history if ever needed)
