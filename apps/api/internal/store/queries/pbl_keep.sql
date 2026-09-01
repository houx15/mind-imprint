-- 上线之后（阶段七）。数据、反馈、新想法进来，就地开一轮新的思考。

-- name: CreatePblKeepEntry :one
INSERT INTO pbl_keep_entry (atom_id, kind, body, stage)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListPblKeepEntries :many
SELECT * FROM pbl_keep_entry WHERE atom_id = $1 ORDER BY created_at DESC;

-- name: GetPblKeepEntry :one
SELECT k.*, a.user_id
FROM pbl_keep_entry k JOIN atom a ON a.id = k.atom_id
WHERE k.id = $1;

-- name: LinkPblKeepEntrySession :one
-- 「we can add a new session for this project. (so one project may have
--   several sessions)」——一条数据长出一轮思考，这一步就是维持和归档的分界。
UPDATE pbl_keep_entry SET session_id = $2 WHERE id = $1 RETURNING *;
