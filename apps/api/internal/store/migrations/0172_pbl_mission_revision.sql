-- +goose Up
ALTER TABLE pbl_mission_item ADD COLUMN superseded_at timestamptz;

-- +goose Down
ALTER TABLE pbl_mission_item DROP COLUMN superseded_at;
