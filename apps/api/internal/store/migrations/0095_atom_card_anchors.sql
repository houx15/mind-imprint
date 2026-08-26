-- +goose Up
-- 轻量版工具卡缺的两列，和 pro 的 card_instances 完全同形
-- （0010_card_anchors.sql 的 anchors、0016 的 framework_fill）：
--
--  anchors        —— 卡片挂在哪一句上。summon 时存 AI 的示范锚点
--                    （block_id + 起止 + 原句），提交时换成学生自己选的那一句。
--                    只存 block_id 不够：阅读室要在正文里精确高亮示范句，
--                    并且「不能选中示范句本身」这条守则是按区间重叠判断的。
--  framework_fill —— 选句复核（agent.EvaluateSelection）的结果。过程即数据：
--                    AI 对学生选句的判断必须留痕，而不是只活在浏览器内存里。
--                    提交只改 field_values/event_trace，所以这一列会留存。
ALTER TABLE atom_card ADD COLUMN anchors jsonb NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE atom_card ADD COLUMN framework_fill jsonb NOT NULL DEFAULT '{}'::jsonb;

-- +goose Down
ALTER TABLE atom_card DROP COLUMN framework_fill;
ALTER TABLE atom_card DROP COLUMN anchors;
