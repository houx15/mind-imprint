-- 出门前的观察清单。见 migration 0124。

-- name: CreatePblMissionItem :one
INSERT INTO pbl_mission_item (tool_id, prompt, want_kind, ordinal)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListPblMissionItems :many
SELECT * FROM pbl_mission_item WHERE tool_id = $1 ORDER BY ordinal, created_at;

-- name: GetPblMissionItem :one
-- 归属一路查到人：清单挂在工具上，工具挂在项目上，项目挂在她身上。
SELECT m.*, a.user_id
FROM pbl_mission_item m
JOIN pbl_tool_instance t ON t.id = m.tool_id
JOIN atom a ON a.id = t.atom_id
WHERE m.id = $1;

-- name: TickPblMissionItem :one
-- 她在现场点掉一条。再点一下是取消——现场点错了不该没法反悔。
UPDATE pbl_mission_item SET done_at = $2 WHERE id = $1 AND superseded_at IS NULL RETURNING *;

-- name: LockPblMissionTool :one
SELECT * FROM pbl_tool_instance WHERE id = $1 FOR UPDATE;

-- name: SupersedePblMissionItems :exec
UPDATE pbl_mission_item SET superseded_at = now() WHERE tool_id = $1 AND superseded_at IS NULL;

-- name: SupersedePblMissionItem :exec
UPDATE pbl_mission_item SET superseded_at = now() WHERE id = $1 AND superseded_at IS NULL;

-- name: CreateStudentPblMissionItem :one
INSERT INTO pbl_mission_item (tool_id, prompt, want_kind, ordinal, edited_by_student)
VALUES ($1, $2, $3, $4, true)
RETURNING *;

-- name: ListPblMissionItemsByAtom :many
-- 这个项目所有出门趟次的清单，回灌用：没做到的那几条要说给印记听。
SELECT m.*
FROM pbl_mission_item m
JOIN pbl_tool_instance t ON t.id = m.tool_id
WHERE t.atom_id = $1
ORDER BY m.created_at, m.ordinal;
