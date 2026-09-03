-- name: CreatePblPersona :one
INSERT INTO pbl_persona (atom_id, label, why_knows, wants, feeling, keywords)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListPblPersonas :many
SELECT * FROM pbl_persona WHERE atom_id = $1 ORDER BY created_at;

-- name: GetPblPersona :one
SELECT * FROM pbl_persona WHERE id = $2 AND atom_id = $1;

-- name: SetPblPersonaPortrait :one
UPDATE pbl_persona SET portrait_key = $3 WHERE id = $2 AND atom_id = $1 RETURNING *;

-- name: SetPblPersonaKeywords :one
UPDATE pbl_persona SET keywords = $3 WHERE id = $2 AND atom_id = $1 RETURNING *;

-- name: ClearPblPersonaChosen :exec
-- 先清空再置一个：唯一索引 pbl_persona_one_chosen 不允许两个同时为真，所以这
-- 两步必须在同一个事务里。
UPDATE pbl_persona SET chosen = false WHERE atom_id = $1 AND chosen;

-- name: ChoosePblPersona :one
UPDATE pbl_persona SET chosen = true WHERE id = $2 AND atom_id = $1 RETURNING *;

-- name: DeletePblPersonasByAtom :exec
-- 「都不像」——整批重来。
DELETE FROM pbl_persona WHERE atom_id = $1;
