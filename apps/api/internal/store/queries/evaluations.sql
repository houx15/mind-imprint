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

-- name: EnqueueEvaluation :one
INSERT INTO evaluations (task_id, scores, narrative, model, tier, status, trigger)
VALUES ($1, '[]'::jsonb, '', '', '', 'queued', 'manual')
RETURNING *;

-- name: MarkEvaluationRunning :exec
UPDATE evaluations SET status = 'running'
WHERE id = $1 AND status = 'queued';

-- name: FinishEvaluation :exec
UPDATE evaluations SET
    scores = $2, narrative = $3, signals = $4, rubric_version = $5,
    model = $6, tier = $7, prompt_tokens = $8, completion_tokens = $9,
    cost_estimate = $10, status = 'done', completed_at = now()
WHERE id = $1;

-- name: FailEvaluation :exec
UPDATE evaluations SET status = 'failed', error = $2, completed_at = now()
WHERE id = $1;
