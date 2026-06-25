-- name: CreateEvaluation :one
INSERT INTO evaluations (
    task_id, scores, narrative, model, tier,
    prompt_tokens, completion_tokens, cost_estimate, status, completed_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, 'done', now()
)
RETURNING *;

-- name: GetLatestEvaluation :one
SELECT * FROM evaluations
WHERE task_id = $1
ORDER BY created_at DESC
LIMIT 1;
