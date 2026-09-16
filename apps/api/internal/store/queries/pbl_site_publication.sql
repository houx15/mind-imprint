-- name: GetPblSitePublication :one
SELECT * FROM pbl_site_publication WHERE user_id=$1;

-- name: SetPblSitePublication :exec
INSERT INTO pbl_site_publication(user_id,atom_id,version_id) VALUES($1,$2,$3)
ON CONFLICT(user_id) DO UPDATE SET atom_id=excluded.atom_id, version_id=excluded.version_id, published_at=now();
