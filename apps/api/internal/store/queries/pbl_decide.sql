-- 做一个决定（阶段三）。
--
-- 摆出选项 → 说清这里什么重要 → 每个选项赢在哪、疼在哪 → 选，说为什么，
-- 并写下什么会让自己改主意。

-- 已有的 RecordPblDecision（pbl_plan.sql）是一次写完的记录：对话里顺手做的
-- 决定，写下来就完了。这里开的是**还没做出来**的决定——它要摊开选项、要她说
-- 清什么重要，然后才落地。两条路都留着，因为不是每个决定都值得摊开。

-- name: CreatePblDecision :one
INSERT INTO pbl_decision (atom_id, session_id, subject, choice, why, gave_up)
VALUES ($1, $2, $3, '', '', '')
RETURNING *;

-- name: GetPblDecision :one
SELECT d.*, a.user_id
FROM pbl_decision d JOIN atom a ON a.id = d.atom_id
WHERE d.id = $1;

-- name: SettlePblDecision :one
-- choice / why / flip 三样都由 Go 校验非空后才到这里。flip 是关键的一格：
-- 写得出「什么会让我改主意」，这个决定才是可复盘的。
UPDATE pbl_decision
SET choice = $2, why = $3, gave_up = $4, flip = $5, settled_at = now()
WHERE id = $1 AND settled_at IS NULL
RETURNING *;

-- ── 选项 ───────────────────────────────────────────────────────────────

-- name: CreatePblDecisionOption :one
INSERT INTO pbl_decision_option (decision_id, label, wins, hurts, author, ordinal)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListPblDecisionOptions :many
SELECT * FROM pbl_decision_option WHERE decision_id = $1 ORDER BY ordinal, created_at;

-- name: UpdatePblDecisionOption :one
UPDATE pbl_decision_option SET label = $2, wins = $3, hurts = $4 WHERE id = $1 RETURNING *;

-- name: GetPblDecisionOption :one
SELECT o.*, a.user_id, d.atom_id, d.settled_at
FROM pbl_decision_option o
JOIN pbl_decision d ON d.id = o.decision_id
JOIN atom a ON a.id = d.atom_id
WHERE o.id = $1;

-- ── 这里什么重要 ───────────────────────────────────────────────────────

-- name: CreatePblDecisionCriterion :one
INSERT INTO pbl_decision_criterion (decision_id, label, author, ordinal)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListPblDecisionCriteria :many
SELECT * FROM pbl_decision_criterion WHERE decision_id = $1 ORDER BY ordinal, created_at;

-- name: DeletePblDecisionCriterion :exec
DELETE FROM pbl_decision_criterion WHERE id = $1;

-- name: GetPblDecisionCriterion :one
SELECT c.*, a.user_id, d.atom_id, d.settled_at
FROM pbl_decision_criterion c
JOIN pbl_decision d ON d.id = c.decision_id
JOIN atom a ON a.id = d.atom_id
WHERE c.id = $1;
