-- +goose Up
ALTER TABLE evaluations ADD COLUMN signals jsonb;
ALTER TABLE evaluations ADD COLUMN rubric_version text;

-- +goose Down
ALTER TABLE evaluations DROP COLUMN rubric_version;
ALTER TABLE evaluations DROP COLUMN signals;
