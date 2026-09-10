-- 读法里多出来的三步：预测 / 标注论证 / 复述。
--
-- 三步都补的是我们整套读法里**根本没有的位置**（见
-- docs/2026-09-10-reading-guidance-redesign.md 那份对七个开源阅读 skill 的
-- 调研），不是把已有的步骤换个说法：
--
--   predict  只看标题，先猜这篇要解决什么问题。它的关键在那句禁止——
--            「正文先别读」。它把「读」从接收变成验证。
--   label    给几句话各自贴一个角色（主张/证据/限制/背景/对比）。这是一次
--            不问「你懂了吗」的理解检查：贴不出来就是没读懂。
--   recall   合上文章，凭记忆说一遍。假不了，而且很便宜。
--
-- 🚨 枚举本身是 0103 立的 CHECK 约束，所以加一种 kind 是一次迁移，不是改一行
-- Go。这一条正是它存在的价值：`reading_routines.go` 里加了三个常量、Go 编译
-- 通过、测试里那十几条也照常跑，而线上第一次排读法就会 500 ——
-- 数据库替我们把「两处枚举没对齐」拦了下来。
--
-- Down 把三种新 kind 折回 'read'，而不是删掉那些行：一条被删掉的步骤会让她的
-- 阅读记录少一段，而回滚不该让学生丢东西。

-- +goose Up
ALTER TABLE reading_task DROP CONSTRAINT reading_task_kind_check;
ALTER TABLE reading_task ADD CONSTRAINT reading_task_kind_check
  CHECK (kind IN ('read','focus_block','lens','reflect','connect','hunt',
                  'predict','label','recall'));

-- +goose Down
UPDATE reading_task SET kind = 'read' WHERE kind IN ('predict','label','recall');
ALTER TABLE reading_task DROP CONSTRAINT reading_task_kind_check;
ALTER TABLE reading_task ADD CONSTRAINT reading_task_kind_check
  CHECK (kind IN ('read','focus_block','lens','reflect','connect','hunt'));
