-- +goose Up
-- 三样「她自己判断过」的痕迹。都是一列，都默认空/零，老数据不用动。
--
-- 共同的理由（铁律④ 过程即数据）：这三处她本来就在做判断，只是那个判断以前
-- 没有地方存，于是既回不到印记那里，复盘时也无从对照。

-- 1 · 复盘：她现在还这么想吗。
--
-- 🚨 空白框里承认「当时没想清楚」要写一段话，成本太高，她于是写「挺好的」。
-- 一个可点的态度只要一下，诚实因此变便宜。空 = 她没表态，那也是信号。
ALTER TABLE pbl_review ADD COLUMN stance text NOT NULL DEFAULT '';

-- 2 · 理性决策：她把几条路排成的顺序。
--
-- 只挑一个不需要把它们放在一起比；排成一列才需要。0 = 没排过。
ALTER TABLE pbl_decision_option ADD COLUMN student_rank integer NOT NULL DEFAULT 0;

-- 3 · 分工：这一件是她自己补上的。
--
-- 审一份方案不等于逐格同意，先要问它漏了什么。漏掉的那一件被她补上，是这件
-- 工具最强的一种信号——而没有这一列，回灌时它和印记自己提的那几件长得一样。
ALTER TABLE pbl_substep ADD COLUMN added_by_student boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE pbl_review DROP COLUMN stance;
ALTER TABLE pbl_decision_option DROP COLUMN student_rank;
ALTER TABLE pbl_substep DROP COLUMN added_by_student;
