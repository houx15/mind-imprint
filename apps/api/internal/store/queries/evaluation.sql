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

-- Course session scope (A1): mirrors the project-scoped pair above. A course
-- session's report is one evaluations row scoped by session_id alone.

-- name: InsertSessionEvaluation :one
INSERT INTO evaluations (session_id, scores, narrative, model, tier,
  prompt_tokens, completion_tokens, cost_estimate, status)
VALUES (@session_id, @scores, @narrative, @model, @tier,
  @prompt_tokens, @completion_tokens, @cost_estimate, 'done')
RETURNING *;

-- name: GetLatestSessionEvaluation :one
SELECT * FROM evaluations
WHERE session_id = @session_id
ORDER BY created_at DESC
LIMIT 1;

-- Chat thread scope (A2): mirrors the session-scoped pair above. A chat
-- thread's report is one evaluations row scoped by thread_id alone.

-- name: InsertThreadEvaluation :one
INSERT INTO evaluations (thread_id, scores, narrative, model, tier,
  prompt_tokens, completion_tokens, cost_estimate, status)
VALUES (@thread_id, @scores, @narrative, @model, @tier,
  @prompt_tokens, @completion_tokens, @cost_estimate, 'done')
RETURNING *;

-- name: GetLatestThreadEvaluation :one
SELECT * FROM evaluations
WHERE thread_id = @thread_id
ORDER BY created_at DESC
LIMIT 1;
