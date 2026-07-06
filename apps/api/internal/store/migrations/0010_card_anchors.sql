-- +goose Up
ALTER TABLE card_instances ADD COLUMN anchors jsonb NOT NULL DEFAULT '[]';

-- +goose Down
ALTER TABLE card_instances DROP COLUMN anchors;
