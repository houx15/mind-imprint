-- 把观察改写成「谁需要什么，为什么」，再变成「我们可以怎样……」（阶段一）。

-- name: CreatePblReframe :one
INSERT INTO pbl_reframe (atom_id, who, needs, why, hmw, supersedes)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListPblReframes :many
SELECT * FROM pbl_reframe WHERE atom_id = $1 ORDER BY created_at;

-- name: GetPblReframe :one
SELECT r.*, a.user_id
FROM pbl_reframe r JOIN atom a ON a.id = r.atom_id
WHERE r.id = $1;

-- name: CurrentPblReframe :one
-- 当前这一版：已确认、且没有被后来的版本取代。
SELECT r.* FROM pbl_reframe r
WHERE r.atom_id = $1
  AND r.confirmed_at IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM pbl_reframe s WHERE s.supersedes = r.id)
ORDER BY r.created_at DESC
LIMIT 1;

-- name: UpdatePblReframe :one
UPDATE pbl_reframe
SET who = $2, needs = $3, why = $4, hmw = $5
WHERE id = $1 AND confirmed_at IS NULL
RETURNING *;

-- name: ConfirmPblReframe :one
-- 确认由她做（「every major update reviewed and confirmed by the student」）。
UPDATE pbl_reframe
SET confirmed_at = now()
WHERE id = $1 AND confirmed_at IS NULL
RETURNING *;
