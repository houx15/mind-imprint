-- 成果与工具。名字仍然全部带 Pbl 前缀。

-- name: CreatePblArtifact :one
INSERT INTO pbl_artifact (atom_id, session_id, kind, title, payload, guessed, admits)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListPblArtifacts :many
SELECT * FROM pbl_artifact WHERE atom_id = $1 ORDER BY created_at;

-- name: GetPblArtifact :one
SELECT a.*, at.user_id
FROM pbl_artifact a JOIN atom at ON at.id = a.atom_id
WHERE a.id = $1;

-- name: SettlePblArtifact :one
-- 没有理由就不落地。why 由 Go 校验（trim 之后非空），这里只落库。
UPDATE pbl_artifact
SET verdict = $2, why = $3, settled_at = now()
WHERE id = $1 AND settled_at IS NULL
RETURNING *;

-- name: SummonPblTool :one
INSERT INTO pbl_tool_instance (atom_id, session_id, tool, reason, kind)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListOpenPblTools :many
-- 还没了结的：刚递出的，和她答应了正在做的。
SELECT * FROM pbl_tool_instance
WHERE atom_id = $1 AND status IN ('summoned', 'accepted')
ORDER BY created_at;

-- name: ListPblTools :many
SELECT * FROM pbl_tool_instance WHERE atom_id = $1 ORDER BY created_at;

-- name: GetPblTool :one
SELECT t.*, a.user_id
FROM pbl_tool_instance t JOIN atom a ON a.id = t.atom_id
WHERE t.id = $1;

-- name: AcceptPblTool :one
-- 她答应了。对于 thinking 工具这几乎是一瞬间的事；对于 world 工具，她可能
-- 就此离开几天——所以这个中间状态必须存得下来，否则「在做」和「放弃了」
-- 只能靠猜。
UPDATE pbl_tool_instance
SET status = 'accepted', accepted_at = now()
WHERE id = $1 AND status = 'summoned'
RETURNING *;

-- name: ResolvePblTool :one
-- declined 也是一个结果：她可以不打开（铁律②），而这件事本身要留痕。
UPDATE pbl_tool_instance
SET status = $2, result = $3, student_note = $4, resolved_at = now()
WHERE id = $1 AND status IN ('summoned', 'accepted')
RETURNING *;

-- 服务端撤掉的那件工具（migration 0134）。
--
-- 闸撤掉一件工具之后，学生和印记都得知道。这两条查询是那条回路的两端：
-- 撤的时候记一行，下一轮建上下文的时候读最近那一行。
-- name: RecordPblToolDrop :exec
INSERT INTO pbl_tool_drop (atom_id, tool, needs) VALUES ($1, $2, $3);

-- name: LatestPblToolDrop :one
SELECT * FROM pbl_tool_drop WHERE atom_id = $1 ORDER BY created_at DESC LIMIT 1;
