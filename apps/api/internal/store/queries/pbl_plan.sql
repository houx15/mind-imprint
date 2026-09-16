-- PBL 的计划：有版本的计划、它的步骤、以及待审阅的结构性变更。

-- name: CreatePblPlanVersion :one
INSERT INTO pbl_plan_version (atom_id, version, summary, reason, decided_by)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: NextPblPlanVersion :one
-- 下一个版本号。和 atom_message 的 seq 一样是 read-then-insert，所以调用方
-- 必须先 LockAtom——理由见 queries/atom.sql 的 LockAtom 注释。
SELECT COALESCE(MAX(version), 0)::int + 1 AS next
FROM pbl_plan_version WHERE atom_id = $1;

-- name: GetPblLivePlan :one
-- 当前生效的那一版：已批准的里面版本号最大的一个。
-- 🚨 未批准的版本不是「当前计划」。她批准第一版之前什么都不许跑（spec §12.5），
-- 而 Plan Check 里被否掉的建议根本不会走到建版本这一步。
SELECT * FROM pbl_plan_version
WHERE atom_id = $1 AND approved_at IS NOT NULL
ORDER BY version DESC LIMIT 1;

-- name: GetPblShownPlan :one
-- 展示最新提案，包括已有生效版本之后的新提案。执行仍只读 GetPblLivePlan。
SELECT * FROM pbl_plan_version
WHERE atom_id = $1
ORDER BY version DESC
LIMIT 1;

-- name: ApprovePblPlanVersion :one
UPDATE pbl_plan_version SET approved_at = now()
WHERE id = $1 AND approved_at IS NULL
RETURNING *;

-- name: ListPblPlanVersions :many
-- 全部版本，含未批准的。过程即数据：改过三次的计划比一次没改的更诚实。
SELECT * FROM pbl_plan_version WHERE atom_id = $1 ORDER BY version;

-- name: CreatePblPlanStep :one
INSERT INTO pbl_plan_step (
  version_id, ordinal, title, blurb, goal, you_bring, i_bring, decide, then_bring, status
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: ListPblPlanSteps :many
SELECT * FROM pbl_plan_step WHERE version_id = $1 ORDER BY ordinal;

-- name: SetPblStepStatus :one
UPDATE pbl_plan_step SET status = $2 WHERE id = $1 RETURNING *;

-- name: GetPblPlanStep :one
SELECT s.*, v.atom_id
FROM pbl_plan_step s JOIN pbl_plan_version v ON v.id = s.version_id
WHERE s.id = $1;

-- ── 待审阅的结构性变更 ─────────────────────────────────────────────────────

-- name: StagePblPendingChange :one
-- 🚨 只写进这张表。这个查询没有任何一条路径能碰到 pbl_plan_step——
-- 结构性变更「进不了当前计划」这条不变量，就是靠这里没有那条路径实现的。
INSERT INTO pbl_pending_change (atom_id, kind, diff, evidence)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListPblOpenChanges :many
SELECT * FROM pbl_pending_change
WHERE atom_id = $1 AND resolved_at IS NULL
ORDER BY created_at;

-- name: ResolvePblPendingChange :one
-- kept 也是一个结果，不是没反应——它带着她的理由一起存下来。
UPDATE pbl_pending_change
SET resolution = $2, reason = $3, resolved_at = now()
WHERE id = $1 AND resolved_at IS NULL
RETURNING *;

-- name: GetPblPendingChange :one
SELECT c.*, a.user_id
FROM pbl_pending_change c JOIN atom a ON a.id = c.atom_id
WHERE c.id = $1;

-- ── 决定 ───────────────────────────────────────────────────────────────────

-- name: RecordPblDecision :one
INSERT INTO pbl_decision (atom_id, session_id, subject, choice, why, gave_up)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListPblDecisions :many
SELECT * FROM pbl_decision WHERE atom_id = $1 ORDER BY created_at;
