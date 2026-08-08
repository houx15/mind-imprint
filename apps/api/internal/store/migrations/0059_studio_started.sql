-- +goose Up
-- studio_state gains a `started` flag: false until the student clicks 开始.
-- New projects default to false; existing projects that already have a
-- studio-surface conversation are mid-journey and must NOT be re-gated.
ALTER TABLE project
  ALTER COLUMN studio_state
  SET DEFAULT '{"stage":"topic_discussion","openTool":"chat","widthTier":"chat","reference":[],"updatedAtTurn":0,"started":false}'::jsonb;

-- Ensure the key exists on every row (false where absent)...
UPDATE project
  SET studio_state = jsonb_set(studio_state, '{started}', 'false'::jsonb, true)
  WHERE NOT (studio_state ? 'started');

-- ...then flip to true for projects with an existing studio thread.
UPDATE project p
  SET studio_state = jsonb_set(p.studio_state, '{started}', 'true'::jsonb, true)
  WHERE EXISTS (
    SELECT 1 FROM chat_thread ct
    JOIN chat_message cm ON cm.thread_id = ct.id
    WHERE ct.seeded_project_id = p.id AND cm.surface = 'studio'
  );

-- +goose Down
ALTER TABLE project
  ALTER COLUMN studio_state
  SET DEFAULT '{"stage":"topic_discussion","openTool":"chat","widthTier":"chat","reference":[],"updatedAtTurn":0}'::jsonb;
UPDATE project
  SET studio_state = studio_state - 'started';
