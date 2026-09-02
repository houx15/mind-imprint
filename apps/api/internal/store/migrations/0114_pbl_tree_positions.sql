-- +goose Up
-- 结构图改成一张真的思维导图（产品负责人 2026-09-02）：
--   「it should be a mindmap, and can directly drag and move and press delete
--    to delete. we don't need so many buttons.」
--
-- 拖动要留得住，所以节点得有自己的位置。
--
-- 🚨 默认 0,0 表示「还没摆过」，界面按层级自动铺一遍；她一拖，位置就变成她的，
-- 之后不再重排。自动布局每次打开都重算的话，她昨天摆好的图今天就变了样——而
-- 一张图的形状本身就是她的思考痕迹。
ALTER TABLE pbl_tree_node ADD COLUMN x real NOT NULL DEFAULT 0;
ALTER TABLE pbl_tree_node ADD COLUMN y real NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE pbl_tree_node DROP COLUMN y;
ALTER TABLE pbl_tree_node DROP COLUMN x;
