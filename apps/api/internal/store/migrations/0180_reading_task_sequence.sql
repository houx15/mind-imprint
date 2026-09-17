-- 读法里多出来的一步：sequence（排出事件顺序 / 排出事件时间线）。
--
-- 同事 2026-09-17 的阅读模块 PRD：阅读模块覆盖议论文、说明文、记叙文、新闻报道，
-- 按文章类型选读法和交互卡。新闻报道要「搭建事件时间线」，记叙文要「事件卡
-- 排序；切换发生顺序／讲述顺序」。这一步只出现在报道和记叙的读法里，
-- 它的内容是一块排序板（order_events，见 reading_genre.go）。
--
-- Down 把 sequence 折回 'reflect'（最近的一种：都是请她整理一遍这篇讲了什么），
-- 而不是删掉那些行 —— 回滚不该让学生的阅读记录少一段。

-- +goose Up
ALTER TABLE reading_task DROP CONSTRAINT reading_task_kind_check;
ALTER TABLE reading_task ADD CONSTRAINT reading_task_kind_check
  CHECK (kind IN ('read','focus_block','lens','reflect','connect','hunt',
                  'predict','label','recall','critique','sequence'));

-- +goose Down
UPDATE reading_task SET kind = 'reflect' WHERE kind = 'sequence';
ALTER TABLE reading_task DROP CONSTRAINT reading_task_kind_check;
ALTER TABLE reading_task ADD CONSTRAINT reading_task_kind_check
  CHECK (kind IN ('read','focus_block','lens','reflect','connect','hunt',
                  'predict','label','recall','critique'));
