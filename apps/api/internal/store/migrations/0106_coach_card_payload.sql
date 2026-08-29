-- +goose Up
-- 聊天卡片挂在提出它的那条消息上（AI 侧是卡片本身，学生侧是她的回答）。
--
-- 为什么不用 atom_card：card_id 必须在 Go 的 cards.ByID 与 TS 的 CARD_REGISTRY
-- 两边都解析得到，而 印记 现场写的卡在两边都没有家；更要命的是
-- atom_card_one_open_idx（0096）是 (atom_id) WHERE status IN ('proposed','active')
-- 的唯一偏索引，聊天卡片一旦以 open 状态落进去，就会把这篇文章的透镜召唤全堵死。
ALTER TABLE atom_message ADD COLUMN payload jsonb;

-- +goose Down
ALTER TABLE atom_message DROP COLUMN payload;
