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

-- A3 growth history: every report the caller owns, across all three scopes,
-- newest-first, one row per scope (reports are one-time; DISTINCT ON is
-- defensive — if two ever share a scope, the latest wins). Owner-filtered
-- through each scope's own join, so the returned ids are guaranteed owned and
-- the embedded report needs no second per-row auth. Labels: project.title /
-- course.title (+ session phase as sublabel) / chat_thread.title.

-- name: ListGrowthHistory :many
SELECT surface, scope_id, label, sublabel, created_at, scores, narrative
FROM (
  (SELECT DISTINCT ON (e.project_id)
     'project'::text AS surface, e.project_id AS scope_id,
     p.title AS label, NULL::text AS sublabel,
     e.created_at AS created_at, e.scores AS scores, e.narrative AS narrative
   FROM evaluations e JOIN project p ON p.id = e.project_id
   WHERE e.project_id IS NOT NULL AND p.user_id = @user_id
   ORDER BY e.project_id, e.created_at DESC)
  UNION ALL
  (SELECT DISTINCT ON (e.session_id)
     'course'::text, e.session_id,
     c.title, cs.phase,
     e.created_at, e.scores, e.narrative
   FROM evaluations e
     JOIN course_session cs ON cs.id = e.session_id
     JOIN course c ON c.id = cs.course_id
   WHERE e.session_id IS NOT NULL AND cs.user_id = @user_id
   ORDER BY e.session_id, e.created_at DESC)
  UNION ALL
  (SELECT DISTINCT ON (e.thread_id)
     'chat'::text, e.thread_id,
     t.title, NULL::text,
     e.created_at, e.scores, e.narrative
   FROM evaluations e JOIN chat_thread t ON t.id = e.thread_id
   WHERE e.thread_id IS NOT NULL AND t.user_id = @user_id
   ORDER BY e.thread_id, e.created_at DESC)
) rows
ORDER BY created_at DESC;

-- C ability model: the latest evaluation per scope the caller owns across all
-- three scopes, oldest-first, for cross-session aggregation. One report per
-- session is the product invariant (terminal reports are one-time; chat opt-in
-- is UI-gated to one), but the generate endpoints INSERT with no upsert guard,
-- so the invariant is client-side-only. DISTINCT ON makes this query defend it
-- itself: identical to a raw UNION ALL when no scope has a duplicate, and if one
-- ever does, the latest row wins — so evidenceCount / recency weight /
-- totalSessions never inflate. Mirrors ListGrowthHistory's latest-per-scope
-- shape (it projects surface/label too; this projects only scores/created_at).

-- name: ListEvaluationsByUser :many
SELECT scores, created_at FROM (
  (SELECT DISTINCT ON (e.project_id)
     e.scores AS scores, e.created_at AS created_at
   FROM evaluations e JOIN project p ON p.id = e.project_id
   WHERE e.project_id IS NOT NULL AND p.user_id = @user_id
   ORDER BY e.project_id, e.created_at DESC)
  UNION ALL
  (SELECT DISTINCT ON (e.session_id)
     e.scores, e.created_at
   FROM evaluations e JOIN course_session cs ON cs.id = e.session_id
   WHERE e.session_id IS NOT NULL AND cs.user_id = @user_id
   ORDER BY e.session_id, e.created_at DESC)
  UNION ALL
  (SELECT DISTINCT ON (e.thread_id)
     e.scores, e.created_at
   FROM evaluations e JOIN chat_thread t ON t.id = e.thread_id
   WHERE e.thread_id IS NOT NULL AND t.user_id = @user_id
   ORDER BY e.thread_id, e.created_at DESC)
) rows
ORDER BY created_at ASC;
