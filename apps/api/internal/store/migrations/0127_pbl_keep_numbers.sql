-- +goose Up
-- 一个数字，和它上一次是多少。
--
-- 🚨 一个数字本身不说明任何事。「这周 23 个人用了」——多还是少？只有和上一次
-- 比才有意思。「数据驱动」这四个字落到一个中学生手里，实际内容就是这一件：
-- 看的是**变化**，不是数值。
--
-- 三列都可空：她可以只记一句话（feedback / thought 那两类本来就没有数字），
-- 也可以第一次记某个指标时没有上一次可比——那时候界面照实说「首次记录」，
-- 而不是假装它是个结果。
ALTER TABLE pbl_keep_entry ADD COLUMN metric text NOT NULL DEFAULT '';
ALTER TABLE pbl_keep_entry ADD COLUMN value numeric;
ALTER TABLE pbl_keep_entry ADD COLUMN prev numeric;
ALTER TABLE pbl_keep_entry ADD COLUMN unit text NOT NULL DEFAULT '';

-- 迭代卡：一次改动 = 一个可以被推翻的预测。
--
-- 🚨 这是「产品迭代」这一步真正的方法。她改一件事的时候心里有个预期（「把心理价
-- 那栏去掉，报名的人会变多」）——把那个预期写下来，下次数据回来就能对照：
-- 兑现了，还是没有。**没兑现才是最值钱的那一次**，因为它说明她原来想错了，
-- 而这正是迭代要教的东西。
--
-- 不写预测也能改东西：expect 空着就是一次普通的改动，不强求（铁律④）。
ALTER TABLE pbl_keep_entry ADD COLUMN expect text NOT NULL DEFAULT '';
-- 这条预测后来兑现了吗：'' 还没看 / 'met' 兑现了 / 'missed' 没兑现。
ALTER TABLE pbl_keep_entry ADD COLUMN verdict text NOT NULL DEFAULT ''
  CHECK (verdict IN ('', 'met', 'missed'));

-- +goose Down
ALTER TABLE pbl_keep_entry DROP COLUMN metric;
ALTER TABLE pbl_keep_entry DROP COLUMN value;
ALTER TABLE pbl_keep_entry DROP COLUMN prev;
ALTER TABLE pbl_keep_entry DROP COLUMN unit;
ALTER TABLE pbl_keep_entry DROP COLUMN expect;
ALTER TABLE pbl_keep_entry DROP COLUMN verdict;
