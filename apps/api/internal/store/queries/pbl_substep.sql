-- 一件任务里的分工（阶段五）。每一格谁做，为什么；她改了也要写为什么。

-- name: CreatePblSubstep :one
INSERT INTO pbl_substep (step_id, ordinal, title, owner, reason, added_by_student)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListPblSubsteps :many
SELECT * FROM pbl_substep WHERE step_id = $1 ORDER BY ordinal, created_at;

-- name: ListPblSubstepsForPlan :many
-- 一次取完整份计划的分工，省得前端按步骤挨个请求。
SELECT s.*
FROM pbl_substep s
JOIN pbl_plan_step st ON st.id = s.step_id
WHERE st.version_id = $1
ORDER BY st.ordinal, s.ordinal;

-- name: GetPblSubstep :one
SELECT s.*, a.user_id, a.id AS atom_id
FROM pbl_substep s
JOIN pbl_plan_step st ON st.id = s.step_id
JOIN pbl_plan_version v ON v.id = st.version_id
JOIN atom a ON a.id = v.atom_id
WHERE s.id = $1;

-- name: ReassignPblSubstep :one
-- 🚨 student_reason 由 Go 校验非空。改一格不写为什么，这张卡就退化成
-- 「全部同意」，也就退化成了 AI 领活。
UPDATE pbl_substep
SET student_owner = $2, student_reason = $3, confirmed_at = now()
WHERE id = $1
RETURNING *;

-- name: ConfirmPblSubstep :one
UPDATE pbl_substep SET confirmed_at = now() WHERE id = $1 RETURNING *;

-- name: SetPblSubstepStatus :one
UPDATE pbl_substep SET status = $2 WHERE id = $1 RETURNING *;
