-- +goose Up
-- 教师端（lite）周总结的文字。数字每次现算，只存模型写的字；每个 (学生, 周) 只生成一次。
CREATE TABLE lite_student_weekly_prose (
  user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  week_start date NOT NULL,
  body       jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, week_start)
);
CREATE TABLE lite_class_weekly_prose (
  class_id   uuid NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
  week_start date NOT NULL,
  body       jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (class_id, week_start)
);

-- +goose Down
DROP TABLE lite_class_weekly_prose;
DROP TABLE lite_student_weekly_prose;
