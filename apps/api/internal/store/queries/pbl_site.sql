-- 她的主页（迁移 0117）。一个学生一行。
--
-- 🚨 每个查询名都带 Pbl 前缀，理由同 pbl_project.sql：pro 与 lite 共用一个 sqlc
-- 包，重名会直接盖掉 pro 的方法。

-- name: GetPblSite :one
SELECT * FROM pbl_site WHERE user_id = $1;

-- name: EnsurePblSite :one
-- 第一次进主页项目时把行建出来。ON CONFLICT DO UPDATE 而不是 DO NOTHING：
-- DO NOTHING 时 RETURNING 一行都不返回，调用方就得再查一次，于是「建好并读出来」
-- 这件本该原子的事变成两次往返、中间有缝。这里 SET atom_id 顺带把主页认到当前
-- 这个项目名下——她重建主页时是新项目、同一个页面。
INSERT INTO pbl_site (user_id, atom_id) VALUES ($1, $2)
ON CONFLICT (user_id) DO UPDATE SET atom_id = EXCLUDED.atom_id, updated_at = now()
RETURNING *;

-- name: SetPblSiteLayout :one
-- 版式 + 她写下的理由。理由为空由服务端拦掉（设计原则：没有理由，什么都不落定），
-- 这里不做校验——约束写在 handler 里才能给她一句话，写在这里只会变成 500。
UPDATE pbl_site SET layout = $2, layout_why = $3, updated_at = now()
WHERE user_id = $1 RETURNING *;

-- name: SetPblSiteContent :one
UPDATE pbl_site SET content = $2, updated_at = now()
WHERE user_id = $1 RETURNING *;

-- name: SetPblSiteShare :one
-- 发布与撤销走同一条语句。撤销 = 两个字段一起置 NULL，下一个请求就查不到了。
UPDATE pbl_site SET share_token = $2, published_at = $3, updated_at = now()
WHERE user_id = $1 RETURNING *;

-- name: GetPblSiteByShareToken :one
-- 公开路由唯一读的那一条。share_token IS NOT NULL 是多余的（NULL = NULL 永远不
-- 成立），但写出来是为了让人一眼看见撤销之后这里必然落空。
SELECT s.*, u.display_name
FROM pbl_site s JOIN users u ON u.id = s.user_id
WHERE s.share_token = $1 AND s.share_token IS NOT NULL;

/* ── 页面上那三张列表的真实来源 ────────────────────────────────────────────
   这三个查询是这一版和原型最根本的差别：原型里 READINGS / WRITINGS / STUDENT
   是三个写死的常量，于是每个学生建出来的都是同一个人的主页。            */

-- name: ListSiteWritingsByUser :many
-- 她真正写完的文章。字数取草稿正文的字符数——中文里字符数就是字数，而且它是
-- 真的，不是估的。没写完的不上主页：主页是给陌生人看的，不是她的工作台。
SELECT a.id AS atom_id, w.title, w.finished_at,
       COALESCE(char_length(d.body), 0)::int AS words
FROM writing w
JOIN atom a ON a.id = w.atom_id
LEFT JOIN writing_draft d ON d.atom_id = w.atom_id
WHERE a.user_id = $1 AND a.kind = 'writing' AND w.status = 'finished'
ORDER BY w.finished_at DESC NULLS LAST
LIMIT 12;

-- name: ListSiteReadingsByUser :many
-- 她读完的东西。source_url 可能没有（她直接粘的正文），所以是 LEFT JOIN + 空串。
SELECT a.id AS atom_id, r.title, r.finished_at,
       COALESCE(s.source_url, '')::text AS source_url
FROM reading r
JOIN atom a ON a.id = r.atom_id
LEFT JOIN reading_source s ON s.atom_id = r.atom_id
WHERE a.user_id = $1 AND a.kind = 'reading' AND r.status = 'finished'
ORDER BY r.finished_at DESC NULLS LAST
LIMIT 12;

-- name: ListSiteProjectsByUser :many
-- 🚨 主页不列自己。主页项目交付的就是这一页，给它一张卡片等于把访客指回他正在
-- 读的这一页。所以 kind <> 'website' —— 这一条在 site.go 里还有一道，因为
-- kind 是 0112 之后由她自己填的，可能是空串。
--
-- 只取做完的（keeping）和在复盘的（review）。talking / running 是还没定型的
-- 想法，把它们摆到主页上，等于替她对外宣布一件她自己都还没想清楚的事。
SELECT a.id AS atom_id, p.name, p.idea, p.kind, p.status,
       a.created_at AS atom_created_at, a.last_activity_at
FROM pbl_project p
JOIN atom a ON a.id = p.atom_id
WHERE a.user_id = $1 AND a.kind = 'project'
  AND p.status IN ('keeping', 'review')
  AND p.kind <> 'website'
ORDER BY a.last_activity_at DESC
LIMIT 12;

-- name: GetPblWebsiteProjectByUser :one
-- 她的主页项目。spec §4 的门槛靠它回答「她是不是已经有一个了」——建第二个主页
-- 项目没有意义，页面是单数的。
SELECT p.*, a.created_at AS atom_created_at, a.last_activity_at
FROM pbl_project p JOIN atom a ON a.id = p.atom_id
WHERE a.user_id = $1 AND a.kind = 'project' AND p.kind = 'website'
ORDER BY a.created_at ASC LIMIT 1;
