-- 0158 —— 计划里的一条材料可以带上它的出处。
--
-- # 它补的是什么
--
-- 规划这一步一直在问同一件事：「有没有你自己见过、经历过的事？」
-- 产品负责人 2026-09-16：
--
--   > in writing, currently we focus too much on personal experience. but we
--   > can also let students to search for other materials, give back the
--   > supporting materials and ai give feedbacks.
--
-- 她说得准。一个十五岁的学生要写「上学时间该不该推迟」，她自己的经历只够撑
-- 一条理由；剩下的那几条要靠她去找 —— 一份睡眠研究、一条本地新闻、一次问卷。
-- 那些材料和她自己的经历一样是材料，而且**它们是要被查的**：出处是谁、原文
-- 到底说了什么、从「相关」跳到「因果」中间少了哪一步。
--
-- 印记 要能查它，就得知道它是从哪来的。所以这一列。
--
-- # 为什么是一列，不是一张表
--
-- 一条材料**已经是**思维导图上的一个节点（depth 2，挂在某条分论点下面）——
-- 它有位置、能拖、能删、会进段落引导。另起一张表就要再做一遍这些，而且两份
-- 「材料」会立刻开始漂。这一列只回答那个节点回答不了的那一个问题：它从哪来。
--
-- 空串 = 这是她自己的经历，或者她没写出处。**不区分这两种**：她见过的事，
-- 出处就是她自己，写一个「本人」进去是多余的。

-- +goose Up
ALTER TABLE writing_outline ADD COLUMN source text NOT NULL DEFAULT '';

COMMENT ON COLUMN writing_outline.source IS
  '这条材料从哪来（链接、刊名、报道名、访谈对象）。空串 = 她自己的经历，或者没写。';

-- +goose Down
ALTER TABLE writing_outline DROP COLUMN source;
