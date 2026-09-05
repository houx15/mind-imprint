-- +goose Up
-- 服务端撤掉的那件工具。
--
-- pbl_tool_gate.go 那道闸是对的：印记说要递「审核助手」却没做出要审的成果，
-- 那张卡打开是一块白板，所以撤掉。但撤掉之后**没有人知道**——学生不知道（她
-- 刚读到印记说给了她），印记也不知道（它下一轮的上文里什么都没变），于是它
-- 照样说「卡我给你了」，她满屏幕找一张永远不会出现的卡。
--
-- 2026-09-04 的模拟学生走查里，Marcus 在这个循环里耗掉约 35 步、8 次要老师
-- 直接替他点。这和「印记看不见页面状态」是同一个形状：闸做了正确的事，结果
-- 没有回到对话里。修法也一样——记下来，然后回灌。
--
-- 为什么不复用 pbl_tool_instance 的 status：那张表是**她屏幕上的卡**，前端按
-- 行渲染。一件从没出现过的工具不该在那张表里占一行。
CREATE TABLE pbl_tool_drop (
  id      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  atom_id uuid NOT NULL REFERENCES atom(id) ON DELETE CASCADE,
  -- 被撤掉的工具名。和 pbl_tool_instance.tool 一样是自由字符串。
  tool    text NOT NULL,
  -- 它缺的是哪一样（decision / artifact / substeps / structure）。回灌那句话
  -- 要说得出「缺的是什么」，否则印记只知道被拒了，不知道该补什么。
  needs   text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX pbl_tool_drop_atom_idx ON pbl_tool_drop (atom_id, created_at DESC);

-- +goose Down
DROP TABLE pbl_tool_drop;
