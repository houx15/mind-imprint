-- +goose Up
-- 学生的性别，由老师在学生页设置。NULL = 未设置。
--
-- 教师端的 AI（班级摘要、班级对话、家长报告）写到学生时会用代词。没有这一列时，
-- 模型只能猜「他」还是「她」，线上猜错过。未设置时 AI 不用「他」「她」，改写学生姓名。
ALTER TABLE users ADD COLUMN gender text CHECK (gender IN ('female', 'male'));

-- +goose Down
ALTER TABLE users DROP COLUMN gender;
