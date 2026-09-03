-- 上线之后（阶段七）。数据、反馈、新想法进来，就地开一轮新的思考。

-- name: CreatePblKeepEntry :one
INSERT INTO pbl_keep_entry (atom_id, kind, body, stage, metric, value, prev, unit, expect)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: ListPblKeepEntries :many
SELECT * FROM pbl_keep_entry WHERE atom_id = $1 ORDER BY created_at DESC;

-- name: GetPblKeepEntry :one
SELECT k.*, a.user_id
FROM pbl_keep_entry k JOIN atom a ON a.id = k.atom_id
WHERE k.id = $1;

-- name: LinkPblKeepEntrySession :one
-- 「we can add a new session for this project. (so one project may have
--   several sessions)」——一条数据长出一轮思考，这一步就是维持和归档的分界。
UPDATE pbl_keep_entry SET session_id = $2 WHERE id = $1 RETURNING *;

-- name: SettlePblKeepPrediction :one
-- 一次改动的预期后来兑现了没有。没兑现才是最值钱的那一次——它说明她原来想错了。
UPDATE pbl_keep_entry SET verdict = $2 WHERE id = $1 AND atom_id = $3 RETURNING *;

-- name: LastPblKeepMetric :one
-- 这个指标上一次是多少。她记新一次时用它自动填 prev——数字的意思在变化里。
SELECT * FROM pbl_keep_entry
WHERE atom_id = $1 AND metric = $2 AND value IS NOT NULL
ORDER BY created_at DESC LIMIT 1;
