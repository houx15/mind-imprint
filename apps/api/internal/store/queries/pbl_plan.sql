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
-- 面板上该显示的那一版：优先已批准的最新一版；一版都没批准过就显示最新的
-- 提案，让她能看见、能审、能按下确认。
--
-- 🚨 只有 GetPblLivePlan 的时候，印记提的计划是**看不见**的：她批准之前它不是
-- 「当前计划」，而面板只问当前计划，于是那一版停在库里，她永远等在「计划待
-- 生成」上，也就永远没有机会批准它。看得见和生效是两件事——生效仍然只认
-- approved_at（spec §12.5），这条查询只管让她看见。
SELECT * FROM pbl_plan_version
WHERE atom_id = $1
ORDER BY (approved_at IS NOT NULL) DESC, version DESC
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
