-- +goose Up
ALTER TABLE messages ADD COLUMN source text;
-- +goose Down
ALTER TABLE messages DROP COLUMN source;
