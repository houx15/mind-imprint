-- +goose Up
ALTER TABLE pbl_mission_item ADD COLUMN edited_by_student boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE pbl_mission_item DROP COLUMN edited_by_student;
