-- +goose Up
-- 行文那一步（同事 2026-09-20 的意见 4）：
--
--	「前期逻辑讨论的部分需要增加一个对于行文方式的思考和梳理部分，
--	  要在开始写之前先想好整个文章组织框架（并非填充内容）如何搭建，
--	  现在只有文本内容的引导。」
--
-- 这一步**不新建表**：它就是那张结构图，重新排个序、每一块标一个方法。
-- 顺序用已有的 position（拖动那套 0182 那一轮已经建好），这里只加两样东西。

-- 这一块打算用哪一个论证方法（vocab 的 id，''=还没定）。
-- 她在行文那一步给每条分论点标一个；段落那一步的引导据此说「这一段你打算用
-- 举例论证」，而不是每次重新猜一个。
ALTER TABLE writing_outline ADD COLUMN method text NOT NULL DEFAULT '';

-- writing.structure_key 改作「整篇的论证结构」（vocab 里 struct_* 的 id）。
--
-- 🚨 这一列自 2026-08-27 模板选择器被删之后**没有任何代码写过它** —— 它只被
-- 读出来、原样回给前端。留在库里的是那之前存的骨架 id，和新的闭表没有交集；
-- 不清掉的话，行文那一步一进去就会显示一个不存在的选择，而她没有办法把它取消。
UPDATE writing SET structure_key = '' WHERE structure_key <> '';

-- 🚨 stage 上有一条 0099 写下的 CHECK，取值里没有 'flow' —— 不放开它，
-- 她点进行文那一步的第一次保存就是一个数据库层的约束冲突，而那会以一个
-- 读不懂的 500 出现在她面前。
--
-- 约束是匿名的（0099 写在列定义上），所以按 Postgres 的默认命名去找。
ALTER TABLE writing DROP CONSTRAINT IF EXISTS writing_stage_check;
ALTER TABLE writing ADD CONSTRAINT writing_stage_check
  CHECK (stage IN ('ideate','outline','flow','snippets','draft','finished'));

-- +goose Down
ALTER TABLE writing_outline DROP COLUMN method;
ALTER TABLE writing DROP CONSTRAINT IF EXISTS writing_stage_check;
ALTER TABLE writing ADD CONSTRAINT writing_stage_check
  CHECK (stage IN ('ideate','outline','snippets','draft','finished'));
