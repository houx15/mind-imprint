-- +goose Up
-- 「上次读到」到底是什么时候。
--
-- 在这一列之前，唯一能回答这个问题的是 reading.updated_at，而它只在两处被
-- 写：重命名和完成。于是一个学生可以读一小时——提问、开卡、划线、写批注——
-- 而列表里那句「上次读到 8 月 20 日」纹丝不动，首页那句「你有 N 篇还没读完」
-- 也永远打开同一篇。
--
-- 放在 atom 上而不是 reading 上，是刻意的：last_activity_at 是**每一种原子**
-- 都有的属性（阅读、写作，以及之后的 AI 聊天 / AI 项目），不是阅读的专属
-- 字段。放在底座上，写作那一形态直接继承，不必再发明第二套自己的。
--
-- 回填用 atom.created_at：对旧行来说这是唯一诚实的下界，且不会把一篇久未
-- 触碰的阅读伪造成「刚刚还在读」。
ALTER TABLE atom
  ADD COLUMN last_activity_at timestamptz NOT NULL DEFAULT now();

UPDATE atom SET last_activity_at = created_at;

-- +goose Down
ALTER TABLE atom DROP COLUMN last_activity_at;
