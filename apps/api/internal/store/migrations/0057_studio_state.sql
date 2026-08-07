-- +goose Up
ALTER TABLE project
  ADD COLUMN studio_state jsonb NOT NULL
  DEFAULT '{"stage":"topic_discussion","openTool":"chat","widthTier":"chat","reference":[],"updatedAtTurn":0}'::jsonb;

-- +goose Down
ALTER TABLE project DROP COLUMN studio_state;
