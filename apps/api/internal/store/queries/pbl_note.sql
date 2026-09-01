-- 便签板。观察、引语、假设、问题、点子共用一张板（阶段一）。

-- name: CreatePblNote :one
INSERT INTO pbl_note (atom_id, kind, body, author, cluster, x, y)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListPblNotes :many
SELECT * FROM pbl_note
WHERE atom_id = $1 AND archived = false
ORDER BY created_at;

-- name: GetPblNote :one
SELECT n.*, a.user_id
FROM pbl_note n JOIN atom a ON a.id = n.atom_id
WHERE n.id = $1;

-- name: UpdatePblNote :one
-- 她改了印记写的便签，edited 就永久为 true——纠正是最强的过程信号之一，
-- 不该因为她后来又改回去而消失。
UPDATE pbl_note
SET body = $2, kind = $3, cluster = $4,
    edited = (edited OR author = 'yinji')
WHERE id = $1
RETURNING *;

-- name: MovePblNote :one
UPDATE pbl_note SET x = $2, y = $3 WHERE id = $1 RETURNING *;

-- name: ArchivePblNote :one
UPDATE pbl_note SET archived = true WHERE id = $1 RETURNING *;

-- name: CountPblNotesByKind :many
SELECT kind, count(*) AS n
FROM pbl_note
WHERE atom_id = $1 AND archived = false
GROUP BY kind;
