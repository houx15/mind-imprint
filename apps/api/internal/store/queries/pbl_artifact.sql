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
INSERT INTO pbl_tool_instance (atom_id, session_id, tool, reason)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListPblTools :many
SELECT * FROM pbl_tool_instance WHERE atom_id = $1 ORDER BY created_at;

-- name: GetPblTool :one
SELECT t.*, a.user_id
FROM pbl_tool_instance t JOIN atom a ON a.id = t.atom_id
WHERE t.id = $1;

-- name: ResolvePblTool :one
-- declined 也是一个结果：她可以不打开（铁律②），而这件事本身要留痕。
UPDATE pbl_tool_instance
SET status = $2, result = $3
WHERE id = $1 AND status = 'summoned'
RETURNING *;
