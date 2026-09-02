-- +goose Up
-- 决策这件事的形态改了（产品负责人 2026-09-02）。
--
-- 原文：
--   「it happens when AI proposes several thing to decide... what we need to do,
--    is to present several options as cards, with title, description, and
--    students can select one to confirm. but when confirm, they need to answer
--    two small questions: why this, and why not others.」
--
-- 也就是说：**选项是印记提的**，她做的是"选"和"说清为什么"。我原来那一版让她
-- 自己加选项、自己定标准、给每个选项填"赢在哪疼在哪"——那是在让她替印记把活
-- 干了，而且把一个当场的判断拖成一张表格。
--
-- 两处改动：
--   ① 选项要有一段说明，光一个标题她判断不了。
--   ② 多存一句「为什么不选别的」。这一句才是这件工具真正教的东西——选中一个
--      不难，说得出为什么放掉另外两个，才说明她真的比较过。
ALTER TABLE pbl_decision_option ADD COLUMN description text NOT NULL DEFAULT '';
ALTER TABLE pbl_decision ADD COLUMN why_not text NOT NULL DEFAULT '';

-- pbl_decision_criterion 不再有人写，但表留着：已经存进去的是过程记录的一部分，
-- 删表等于篡改历史。flip 同理，列还在，界面上不再问。

-- +goose Down
ALTER TABLE pbl_decision DROP COLUMN why_not;
ALTER TABLE pbl_decision_option DROP COLUMN description;
