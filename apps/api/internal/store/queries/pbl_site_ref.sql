-- name: CreatePblSiteRef :one
-- 同一个项目里同一个网址只留一条。她贴第二遍是手滑，不是第二个例子——所以
-- 冲突时更新印记那三句（重贴多半意味着上一次读失败了），保留她自己补的那句。
INSERT INTO pbl_site_ref (atom_id, url, title, what, structure, best)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (atom_id, url) DO UPDATE
SET title = EXCLUDED.title,
    what = EXCLUDED.what,
    structure = EXCLUDED.structure,
    best = EXCLUDED.best
RETURNING *;

-- name: ListPblSiteRefs :many
SELECT * FROM pbl_site_ref WHERE atom_id = $1 ORDER BY created_at;

-- name: SetPblSiteRefSaid :one
-- 她在这一站上补的那一句。
UPDATE pbl_site_ref SET she_said = $3 WHERE id = $2 AND atom_id = $1 RETURNING *;

-- name: DeletePblSiteRef :exec
DELETE FROM pbl_site_ref WHERE id = $2 AND atom_id = $1;

-- name: CountPblSiteRefs :one
SELECT count(*) FROM pbl_site_ref WHERE atom_id = $1;
