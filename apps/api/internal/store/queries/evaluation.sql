-- Project-scoped assessment persistence (Slice 10). evaluations.project_id
-- exists since migration 0016; migration 0021 makes task_id optional and
-- adds the scope CHECK so a row can be project-only.
--
-- The old dual-axis generation/read pipeline (session/thread scopes, growth
-- history, cross-user evaluation listing) was retired 2026-08-14
-- (retire-old-evaluation-pipeline). InsertProjectEvaluation itself has no
-- production caller anymore either — the finish path now writes the new
-- EvaluationReport (see evaluation_report.sql) — but it is kept here because
-- the teacher roster/detail read-path tests (internal/api/teacher_read_test.go)
-- still use it to seed `evaluations` rows for the KEPT student_evaluation
-- view/queries.

-- name: InsertProjectEvaluation :one
INSERT INTO evaluations (project_id, scores, narrative, model, tier,
  prompt_tokens, completion_tokens, cost_estimate, status)
VALUES (@project_id, @scores, @narrative, @model, @tier,
  @prompt_tokens, @completion_tokens, @cost_estimate, 'done')
RETURNING *;
