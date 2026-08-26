-- llm_call — the metering table every live LLM call (coach turn, anchor
-- generation, course-step render) records onto (migration 0019, 5d review
-- CRITICAL fix). project_id is NULL for course/chat calls, which have no
-- owning project.

-- name: RecordLLMCall :one
INSERT INTO llm_call (user_id, project_id, surface, purpose, provider, model, tier, prompt_tokens, completion_tokens, cost_estimate)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: ListLLMCallsByProject :many
SELECT * FROM llm_call
WHERE project_id = $1
ORDER BY created_at;

-- name: CountLLMCallsByProjectPurpose :one
SELECT count(*) FROM llm_call
WHERE project_id = $1 AND purpose = $2;

-- name: RecordAtomLLMCall :one
-- The lite edition's metering row (migration 0094). project_id stays NULL —
-- a lite atom has no owning project, exactly as course/chat calls have none —
-- and atom_id points back at the reading/writing the call belongs to. Kept as
-- a SEPARATE statement rather than widening RecordLLMCall so the pro side's
-- ~20 call sites keep their existing params struct untouched.
INSERT INTO llm_call (user_id, atom_id, surface, purpose, provider, model, tier, prompt_tokens, completion_tokens, cost_estimate)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: ListLLMCallsByAtom :many
SELECT * FROM llm_call
WHERE atom_id = $1
ORDER BY created_at;
