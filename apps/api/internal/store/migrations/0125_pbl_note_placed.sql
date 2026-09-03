-- +goose Up
-- 一条便签放进了结构里的哪一块。
--
-- 🚨 「结构审查」问她三件事，第一件是「这个分法盖全了吗」——而那一直是一个
-- 没法回答的问题：她只能盯着一棵提纲想「大概全了吧」。
--
-- 把她自己攒下来的材料（观察、原话、推论、问题）一条一条拖进节点里，「盖全了
-- 吗」就变成一个看得见的答案：**放不进去的那几条就是没盖到的地方**。这比任何
-- 自评都硬——那几条是她亲手收集的，不是印记编出来考她的。
--
-- ON DELETE SET NULL：结构改来改去是常事，删掉一个节点不该把她的便签一起带走。
-- 便签活在板上，放进结构只是它的一个去向。
ALTER TABLE pbl_note
  ADD COLUMN tree_node_id uuid REFERENCES pbl_tree_node(id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE pbl_note DROP COLUMN tree_node_id;
