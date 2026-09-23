-- 🚨 0190 合并时从 0189 改过号。另一个会话的 0189_lite_assignment_issue
-- 已经在 origin/main 上，而 goose 碰到重复的版本号会直接起不来
-- （memory: wip-recovery-and-migration-renumber-2026-09-16 —— 这是同一类
-- 事故的第三次）。这一条只动 reading_task 的 kind 闭表加 shape，和那一条没有任何依赖，换号安全。
--
-- 读法里多出来的一步：shape（看作者怎么安排这篇）。
--
-- 产品负责人 2026-09-23 第 5 条：
--
--   「when reading a 记叙文, it always focuses on very detailed things and
--     ignores the general structure, the writing format etc. to give
--     guidance.」
--
-- 她是对的，而且原因在清单里量得出来：zh-narrative 那一套七步里，除了
-- sequence 之外**每一步都是句子或段落那一层**的，而且它是九套读法里唯一
-- 既没有 predict 也没有 reflect 的一套 —— 整篇那一层根本没有位置。
--
-- shape 就是那个位置：叙述顺序（顺叙 / 倒叙 / 插叙）、哪一段详写哪几段带过、
-- 全篇靠什么串起来。它和 sequence 是两件事：sequence 排的是**事情**的先后，
-- shape 问的是**作者**为什么这样排。
--
-- Down 把 shape 折回 'reflect'（最近的一种：都是请她就整篇说一句），
-- 而不是删掉那些行 —— 回滚不该让学生的阅读记录少一段。

-- +goose Up
ALTER TABLE reading_task DROP CONSTRAINT reading_task_kind_check;
ALTER TABLE reading_task ADD CONSTRAINT reading_task_kind_check
  CHECK (kind IN ('read','focus_block','lens','reflect','connect','hunt',
                  'predict','label','recall','critique','sequence','shape'));

-- +goose Down
UPDATE reading_task SET kind = 'reflect' WHERE kind = 'shape';
ALTER TABLE reading_task DROP CONSTRAINT reading_task_kind_check;
ALTER TABLE reading_task ADD CONSTRAINT reading_task_kind_check
  CHECK (kind IN ('read','focus_block','lens','reflect','connect','hunt',
                  'predict','label','recall','critique','sequence'));
