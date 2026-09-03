-- 两张便签之间的关系。见 migration 0126。

-- name: CreatePblNoteLink :one
-- 同一对纸之间同一种关系连一次就够：她再连一次是想改关系，不是想要两条线。
INSERT INTO pbl_note_link (atom_id, from_id, to_id, relation)
VALUES ($1, $2, $3, $4)
ON CONFLICT (from_id, to_id, relation) DO UPDATE SET relation = EXCLUDED.relation
RETURNING *;

-- name: ListPblNoteLinks :many
SELECT * FROM pbl_note_link WHERE atom_id = $1 ORDER BY created_at;

-- name: DeletePblNoteLink :exec
DELETE FROM pbl_note_link WHERE id = $1 AND atom_id = $2;

-- name: ListPblNoteLinksWithBodies :many
-- 回灌用：印记要读到「『走廊站着 14 个人』和『教室里坐不住』互相矛盾」，
-- 不是两个 uuid。
SELECT l.relation, f.body AS from_body, t.body AS to_body
FROM pbl_note_link l
JOIN pbl_note f ON f.id = l.from_id
JOIN pbl_note t ON t.id = l.to_id
WHERE l.atom_id = $1
ORDER BY l.created_at;
