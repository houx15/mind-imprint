-- +goose Up
-- 家长报告只由老师生成、编辑、导出，没有家长端：去掉发布、公开链接与学生查看用的列。
-- hidden 记老师从报告里隐藏的金句与关键词，按原文定位：
--   {"moments": [quote…], "keywords": [text…]}
ALTER TABLE lite_parent_report
  DROP COLUMN IF EXISTS status,
  DROP COLUMN IF EXISTS share_token,
  DROP COLUMN IF EXISTS published_at,
  DROP COLUMN IF EXISTS student_seen_at;
ALTER TABLE lite_parent_report
  ADD COLUMN hidden jsonb NOT NULL DEFAULT '{}'::jsonb;

-- +goose Down
-- 四列按 0151 的定义加回（CHECK 与 UNIQUE 一并恢复）；已有的报告都回到 draft。
ALTER TABLE lite_parent_report DROP COLUMN IF EXISTS hidden;
ALTER TABLE lite_parent_report
  ADD COLUMN status          text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','published')),
  ADD COLUMN share_token     text UNIQUE,
  ADD COLUMN published_at    timestamptz,
  ADD COLUMN student_seen_at timestamptz;
