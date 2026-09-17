-- 读法里多出来的一步：critique（你怎么看）。
--
-- 产品负责人 2026-09-17 第三轮走查逐字：
--
--	we can invite students to give some comments on this, like do they agree
--	with author's view, do they think the evidence is enough, or do they think
--	if there is another possiblity. this is very important for developing
--	critical thinking.
--	and split them. first is analyze what author written. then is students'
--	self critical thinking.
--
-- 关键在那个 split：拆作者的论证（label，一块板）和她自己怎么看（critique）
-- 是**两件事**，挤在一步里她只会挑更容易的那件做。
--
-- 同一次走查里 透镜 被判出局（「it is really not applicable in many papers.
-- and difficult for students to understand. the above mentioned critical
-- thinking can be a better replacement of lens.」）—— 所以 critique 不是多加
-- 一步，是**顶掉 lens 那一步**。
--
-- 🚨 'lens' 留在 CHECK 里，不删：已经存在的阅读记录里有 lens 的行，删掉约束里
-- 的那一项会让那些行连读都读不出来。读法库里不再排它，和数据库允不允许它是
-- 两件事。
--
-- Down 把 critique 折回 'reflect'（最近的一种：都是请她说一句自己的话），
-- 而不是删掉那些行 —— 回滚不该让学生的阅读记录少一段。

-- +goose Up
ALTER TABLE reading_task DROP CONSTRAINT reading_task_kind_check;
ALTER TABLE reading_task ADD CONSTRAINT reading_task_kind_check
  CHECK (kind IN ('read','focus_block','lens','reflect','connect','hunt',
                  'predict','label','recall','critique'));

-- +goose Down
UPDATE reading_task SET kind = 'reflect' WHERE kind = 'critique';
ALTER TABLE reading_task DROP CONSTRAINT reading_task_kind_check;
ALTER TABLE reading_task ADD CONSTRAINT reading_task_kind_check
  CHECK (kind IN ('read','focus_block','lens','reflect','connect','hunt',
                  'predict','label','recall'));
