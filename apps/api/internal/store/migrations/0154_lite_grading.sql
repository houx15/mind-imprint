-- +goose Up
-- 老师的 AI 批改（lite）。一行对应学生写作的一个提交版本。
-- ai：通过检查的模型结果，写入后不修改。content：老师编辑、发送的内容，初值是 ai 的副本。
-- 学生只读 status = 'sent' 的行。
-- user_id / class_id：学生与批改时所在的班级；教师接口按 class_id 判断归属。
CREATE TABLE lite_grading (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  atom_id         uuid NOT NULL REFERENCES atom(id) ON DELETE CASCADE,
  version_id      uuid NOT NULL UNIQUE REFERENCES writing_version(id) ON DELETE CASCADE,
  user_id         uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  class_id        uuid NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
  assignment_id   uuid REFERENCES lite_assignment(id) ON DELETE SET NULL,
  rubric          jsonb NOT NULL,
  status          text NOT NULL DEFAULT 'queued'
                  CHECK (status IN ('queued', 'running', 'draft', 'failed', 'sent')),
  ai              jsonb,
  content         jsonb,
  error           text,
  requested_by    uuid NOT NULL REFERENCES users(id),
  reviewed_at     timestamptz,
  sent_at         timestamptz,
  student_seen_at timestamptz,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  CHECK (status <> 'sent' OR content IS NOT NULL)
);
CREATE INDEX lite_grading_atom_idx ON lite_grading (atom_id);
CREATE INDEX lite_grading_assignment_idx ON lite_grading (assignment_id) WHERE assignment_id IS NOT NULL;
CREATE INDEX lite_grading_user_sent_idx ON lite_grading (user_id, sent_at DESC) WHERE status = 'sent';

-- +goose Down
DROP TABLE lite_grading;
