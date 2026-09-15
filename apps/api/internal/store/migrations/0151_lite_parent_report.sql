-- +goose Up
-- 家长报告：老师为一名学生、一段日期生成。facts 在生成时冻结；draft 是最近一次模型草稿；
-- body 是老师改过的文字；发布后才有 share_token，撤销时置空。
CREATE TABLE lite_parent_report (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id         uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  class_id        uuid NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
  created_by      uuid NOT NULL REFERENCES users(id),
  range_start     date NOT NULL,
  range_end       date NOT NULL,
  facts           jsonb NOT NULL,
  draft           jsonb,
  body            jsonb,
  status          text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','published')),
  share_token     text UNIQUE,
  published_at    timestamptz,
  student_seen_at timestamptz,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX lite_parent_report_user_idx ON lite_parent_report (user_id, created_at DESC);
CREATE INDEX lite_parent_report_class_idx ON lite_parent_report (class_id, created_at DESC);

-- +goose Down
DROP TABLE lite_parent_report;
