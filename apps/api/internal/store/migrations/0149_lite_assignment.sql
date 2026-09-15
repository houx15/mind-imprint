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

-- 作业来源记在这一项上，不靠 recipient 行反查：作业归档、班级删除、查询失败之后
-- 仍然知道哪段文字是老师写的。老师写的字不存成她说的话。
-- 写作：老师布置的题目。NULL = 她自己开的写作。
ALTER TABLE writing ADD COLUMN assigned_prompt text;
-- 项目：assigned = idea 存的是老师布置的驱动问题；assigned_brief 是老师的补充说明。
ALTER TABLE pbl_project ADD COLUMN assigned boolean NOT NULL DEFAULT false;
ALTER TABLE pbl_project ADD COLUMN assigned_brief text;

-- +goose Down
ALTER TABLE pbl_project DROP COLUMN assigned_brief;
ALTER TABLE pbl_project DROP COLUMN assigned;
ALTER TABLE writing DROP COLUMN assigned_prompt;
ALTER TABLE pbl_project DROP COLUMN finished_at;
DROP TABLE lite_assignment_recipient;
DROP TABLE lite_assignment;
