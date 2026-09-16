-- PBL 项目。atom 是身份，这里是细节——和 reading.sql / writing_atom.sql 同构。
--
-- 🚨 每个查询名都带 Pbl 前缀。queries/project.sql 是 pro 的项目，两者共用
-- 一个 sqlc 包，重名会直接覆盖掉 pro 的方法。

-- name: CreatePblProject :one
INSERT INTO pbl_project (atom_id, idea, kind, name) VALUES ($1, $2, $3, $4) RETURNING *;

-- name: GetPblProject :one
-- user_id 随行，patch 端点靠它做归属校验，省一次 GetAtom。
SELECT p.*, a.user_id, a.created_at AS atom_created_at, a.last_activity_at
FROM pbl_project p JOIN atom a ON a.id = p.atom_id
WHERE p.atom_id = $1;

-- name: ListPblProjectsByUser :many
-- 看板一次读全部：一个学生的项目是十几个的量级，不分页。按最近活跃降序，
-- 前端再按 status 分列——于是「看板」和「时间线」是同一份数据的两种画法，
-- 换视图不用换请求。
-- 卡片上要显示"现在走到哪一步"，所以顺带把当前计划里第一件还没做完的事捞出来。
-- 用 LATERAL 而不是在 Go 里循环查：一个学生十几个项目，那就是十几次往返。
SELECT p.*, a.created_at AS atom_created_at, a.last_activity_at,
       -- 🚨 COALESCE 不能省：这几个都是 LEFT JOIN 出来的，项目还没有计划时是
       -- NULL，而 sqlc 只看 pbl_plan_step.title 的 NOT NULL，会生成成 string，
       -- 于是"还没有计划"这个最常见的情况一扫描就炸。
       COALESCE(step.title, '')::text AS current_step,
       COALESCE(done.n, 0)::int AS steps_done,
       COALESCE(total.n, 0)::int AS steps_total,
       COALESCE((SELECT v.approved_at IS NULL FROM pbl_plan_version v
                 WHERE v.atom_id=p.atom_id ORDER BY v.version DESC LIMIT 1), false)::boolean AS plan_pending
FROM pbl_project p
JOIN atom a ON a.id = p.atom_id
LEFT JOIN LATERAL (
  SELECT v.id FROM pbl_plan_version v
  WHERE v.atom_id = p.atom_id AND v.approved_at IS NOT NULL
  ORDER BY v.version DESC LIMIT 1
) live ON true
LEFT JOIN LATERAL (
  SELECT s.title FROM pbl_plan_step s
  WHERE s.version_id = live.id AND s.status NOT IN ('done', 'cancelled')
  ORDER BY s.ordinal LIMIT 1
) step ON true
LEFT JOIN LATERAL (
  SELECT count(*) AS n FROM pbl_plan_step s
  WHERE s.version_id = live.id AND s.status = 'done'
) done ON true
LEFT JOIN LATERAL (
  SELECT count(*) AS n FROM pbl_plan_step s WHERE s.version_id = live.id
) total ON true
WHERE a.user_id = $1 AND a.kind = 'project'
ORDER BY a.last_activity_at DESC;

-- name: CountPblProjectsByUser :one
-- spec §4：她还没有项目的时候，第一个项目就是做自己的主页。
SELECT count(*) FROM pbl_project p JOIN atom a ON a.id = p.atom_id
WHERE a.user_id = $1 AND a.kind = 'project';

-- name: UpdatePblProjectMeta :one
UPDATE pbl_project SET name = $2, cover_ground = $3, cover_glyph = $4, updated_at = now()
WHERE atom_id = $1 RETURNING *;

-- name: SetPblBoardAxes :one
UPDATE pbl_project SET board_axes = $2, updated_at = now()
WHERE atom_id = $1 RETURNING *;

-- name: SetPblProjectStatus :one
UPDATE pbl_project SET status = $2, updated_at = now()
WHERE atom_id = $1 RETURNING *;

-- name: MarkPblProjectAssigned :exec
-- 老师布置的项目：idea 是老师的驱动问题，不是她的原话。建项目的同一个事务里写。
UPDATE pbl_project SET assigned = true, assigned_brief = $2 WHERE atom_id = $1;

-- name: StampPblProjectFinished :exec
-- 第一次进入回顾或保留时写入完成时间；之后再改状态不覆盖。
-- pbl_project has no id column: its primary key is atom_id, which is also the project id in routes.
UPDATE pbl_project SET finished_at = now() WHERE atom_id = $1 AND finished_at IS NULL;
