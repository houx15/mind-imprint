-- PBL session：自由深挖和四种思考模式共用的那张表。
-- 名字全部带 Pbl 前缀——和 pro 共用一个 sqlc 包，重名会直接覆盖掉对方的方法。

-- name: CreatePblSession :one
INSERT INTO pbl_session (atom_id, kind, parent_id, depth, anchor_kind, anchor_ref, question)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetPblSession :one
SELECT s.*, a.user_id
FROM pbl_session s JOIN atom a ON a.id = s.atom_id
WHERE s.id = $1;

-- name: ListPblSessions :many
-- 一个项目的全部 session，含未关闭的。前端按 parent_id 还原成树。
SELECT * FROM pbl_session WHERE atom_id = $1 ORDER BY created_at;

-- name: ClosePblSession :one
-- 写回内容够不够，由 Go 按 kind 判断（pbl.ValidateClose）——SQL 表达不了
-- 「这个 kind 至少要产出什么」。这里只负责落库。
UPDATE pbl_session
SET takeaway = $2, writeback = $3, closed_at = now()
WHERE id = $1 AND closed_at IS NULL
RETURNING *;

-- name: ListPblSessionWriteBacks :many
-- 主线（或某个父 session）能看到的：已关闭子 session 的写回，而不是它们的
-- 每一轮对话。这就是 spec §10.3 的上下文规则落到查询上的样子。
SELECT * FROM pbl_session
WHERE atom_id = $1
  AND parent_id IS NOT DISTINCT FROM sqlc.narg('parent_id')::uuid
  AND closed_at IS NOT NULL
ORDER BY closed_at;

-- name: AppendPblSessionMessage :one
INSERT INTO atom_message (atom_id, seq, role, content, payload, session_id)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListPblSessionMessages :many
-- 一个 session 自己的对话。seq 仍在原子的同一个空间里，所以过滤之后仍然有序
-- （中间的空档是别处的消息）。
SELECT * FROM atom_message
WHERE atom_id = $1 AND session_id = $2
ORDER BY seq;

-- name: ListPblMainThread :many
-- 项目主线。session_id IS NULL 是它的全部定义——和 0102 的 block_id 同构。
SELECT * FROM atom_message
WHERE atom_id = $1 AND session_id IS NULL AND block_id IS NULL
ORDER BY seq;
