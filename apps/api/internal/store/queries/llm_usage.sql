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
