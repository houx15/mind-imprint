-- name: AppendMessage :one
INSERT INTO messages (
    task_id, role, content, tool_call,
    provider, model, tier, prompt_tokens, completion_tokens, cost_estimate
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
)
RETURNING *;

-- name: ListMessagesByTask :many
SELECT * FROM messages
WHERE task_id = $1
ORDER BY created_at, id;

-- name: CountSubstantiveTurns :one
SELECT count(*) FROM messages
WHERE task_id = $1 AND role = 'user' AND char_length(content) >= 20;
