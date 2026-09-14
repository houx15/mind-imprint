-- +goose Up
-- 教师布置的作业（lite）。学生那一项在她点「开始」时才创建，
-- recipient.atom_id 在那一刻写入；状态不存，由截止时间与完成时间推出。
CREATE TABLE lite_assignment (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  class_id     uuid NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
  created_by   uuid NOT NULL REFERENCES users(id),
  kind         text NOT NULL CHECK (kind IN ('reading','writing','project')),
  title        text NOT NULL,
  instructions text NOT NULL DEFAULT '',
  payload      jsonb NOT NULL,
  due_at       timestamptz NOT NULL,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  archived_at  timestamptz
);
CREATE INDEX lite_assignment_class_idx ON lite_assignment (class_id, due_at DESC);

CREATE TABLE lite_assignment_recipient (
  assignment_id uuid NOT NULL REFERENCES lite_assignment(id) ON DELETE CASCADE,
  user_id       uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  seen_at       timestamptz,
  atom_id       uuid UNIQUE REFERENCES atom(id) ON DELETE SET NULL,
  started_at    timestamptz,
  PRIMARY KEY (assignment_id, user_id)
);
CREATE INDEX lite_assignment_recipient_user_idx ON lite_assignment_recipient (user_id);

-- 项目「做完」的时刻：第一次进入回顾或保留时写入，之后不再改。
ALTER TABLE pbl_project ADD COLUMN finished_at timestamptz;

-- +goose Down
ALTER TABLE pbl_project DROP COLUMN finished_at;
DROP TABLE lite_assignment_recipient;
DROP TABLE lite_assignment;
