-- +goose Up
ALTER TABLE project ADD COLUMN cover text;

-- +goose Down
ALTER TABLE project DROP COLUMN cover;
