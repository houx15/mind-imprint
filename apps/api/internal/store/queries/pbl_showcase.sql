-- name: GetPblShowcase :one
SELECT * FROM pbl_showcase WHERE user_id=$1;

-- name: EnsurePblShowcase :one
INSERT INTO pbl_showcase(user_id) VALUES($1)
ON CONFLICT(user_id) DO UPDATE SET user_id=excluded.user_id
RETURNING *;

-- name: SavePblShowcase :one
UPDATE pbl_showcase SET draft=$3, revision=revision+1, updated_at=now()
WHERE user_id=$1 AND revision=$2 RETURNING *;

-- name: PublishPblShowcase :one
UPDATE pbl_showcase SET published_config=draft, published_at=now(), updated_at=now()
WHERE user_id=$1 AND revision=$2 RETURNING *;

-- name: UnpublishPblShowcase :one
UPDATE pbl_showcase SET published_at=NULL, updated_at=now()
WHERE user_id=$1 RETURNING *;

-- name: ListShowcaseWorks :many
SELECT a.id AS atom_id, 'writing'::text AS kind, w.title,
       COALESCE(NULLIF(left(d.body, 240), ''), '')::text AS summary,
       COALESCE(rep.share_token, '')::text AS share_token
FROM writing w JOIN atom a ON a.id=w.atom_id
LEFT JOIN writing_draft d ON d.atom_id=a.id
LEFT JOIN atom_report rep ON rep.atom_id=a.id
WHERE a.user_id=$1 AND w.status='finished'
UNION ALL
SELECT a.id, 'reading'::text, r.title,
       COALESCE(t.text, '')::text, COALESCE(rep.share_token, '')::text
FROM reading r JOIN atom a ON a.id=r.atom_id
LEFT JOIN reading_takeaway t ON t.atom_id=a.id
LEFT JOIN atom_report rep ON rep.atom_id=a.id
WHERE a.user_id=$1 AND r.status='finished'
UNION ALL
SELECT a.id, 'project'::text, COALESCE(NULLIF(p.name,''), left(p.idea,80)),
       left(p.idea,240), ''::text
FROM pbl_project p JOIN atom a ON a.id=p.atom_id
WHERE a.user_id=$1 AND p.status IN ('keeping','review') AND p.kind <> 'website'
ORDER BY kind, title;
