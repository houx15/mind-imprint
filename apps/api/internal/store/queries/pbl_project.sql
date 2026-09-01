-- PBL 项目。atom 是身份，这里是细节——和 reading.sql / writing_atom.sql 同构。
--
-- 🚨 每个查询名都带 Pbl 前缀。queries/project.sql 是 pro 的项目，两者共用
-- 一个 sqlc 包，重名会直接覆盖掉 pro 的方法。

-- name: CreatePblProject :one
INSERT INTO pbl_project (atom_id, idea, kind) VALUES ($1, $2, $3) RETURNING *;

-- name: GetPblProject :one
-- user_id 随行，patch 端点靠它做归属校验，省一次 GetAtom。
SELECT p.*, a.user_id, a.created_at AS atom_created_at, a.last_activity_at
FROM pbl_project p JOIN atom a ON a.id = p.atom_id
WHERE p.atom_id = $1;

-- name: ListPblProjectsByUser :many
-- 看板一次读全部：一个学生的项目是十几个的量级，不分页。按最近活跃降序，
-- 前端再按 status 分列——于是「看板」和「时间线」是同一份数据的两种画法，
-- 换视图不用换请求。
SELECT p.*, a.created_at AS atom_created_at, a.last_activity_at
FROM pbl_project p JOIN atom a ON a.id = p.atom_id
WHERE a.user_id = $1 AND a.kind = 'project'
ORDER BY a.last_activity_at DESC;

-- name: CountPblProjectsByUser :one
-- spec §4：她还没有项目的时候，第一个项目就是做自己的主页。
SELECT count(*) FROM pbl_project p JOIN atom a ON a.id = p.atom_id
WHERE a.user_id = $1 AND a.kind = 'project';

-- name: UpdatePblProjectMeta :one
UPDATE pbl_project SET name = $2, cover_ground = $3, cover_glyph = $4, updated_at = now()
WHERE atom_id = $1 RETURNING *;

-- name: SetPblProjectStatus :one
UPDATE pbl_project SET status = $2, updated_at = now()
WHERE atom_id = $1 RETURNING *;
