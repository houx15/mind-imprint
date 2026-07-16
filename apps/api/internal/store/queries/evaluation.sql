-- Project-scoped assessment persistence (Slice 10). evaluations.project_id
-- exists since migration 0016; migration 0021 makes task_id optional and
-- adds the scope CHECK so a row can be project-only.

-- name: InsertProjectEvaluation :one
INSERT INTO evaluations (project_id, scores, narrative, model, tier,
  prompt_tokens, completion_tokens, cost_estimate, status)
VALUES (@project_id, @scores, @narrative, @model, @tier,
  @prompt_tokens, @completion_tokens, @cost_estimate, 'done')
RETURNING *;

-- name: GetLatestProjectEvaluation :one
SELECT * FROM evaluations
WHERE project_id = @project_id
ORDER BY created_at DESC
LIMIT 1;
