-- Project-scoped card_instance lifecycle (Slice 3, agent-spec §3). The
-- table is `card_instances` (Slice-0's task-scoped table, extended by
-- migration 0016 with project_id/contract_ref/framework_fill); task_id went
-- nullable in migration 0020 alongside material.task_id (Slice 6b's
-- project-scoped source-log ingestion creates materials with no task) —
-- the runtime resolves it from the target material's own (possibly NULL)
-- task_id (agentstore.go), never asking the pure agent code to know about
-- tasks.
-- The old task-scoped queries in cards.sql (CreateCardInstance, SetCardActive,
-- SubmitCard, SkipCard, SetCardAnchors) are otherwise unused — there is no
-- legacy path any more (Slice 5d retired the task-based surface) — they stay
-- alive only because their own tests (store/sqlc_lifecycle_test.go,
-- api/anchors_store_test.go) still exercise them directly against the table.

-- name: CreateProjectCardInstance :one
INSERT INTO card_instances (task_id, project_id, card_id, contract_ref, status)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: SetCardInstanceAnchors :one
UPDATE card_instances SET anchors = $3
WHERE id = $1 AND project_id = $2
RETURNING *;

-- name: SetCardInstanceFramework :one
UPDATE card_instances SET framework_fill = $3
WHERE id = $1 AND project_id = $2
RETURNING *;

-- name: SetCardInstanceStatus :one
UPDATE card_instances SET status = $3
WHERE id = $1 AND project_id = $2
RETURNING *;

-- name: SubmitProjectCardInstance :one
UPDATE card_instances SET field_values = $3, event_trace = $4
WHERE id = $1 AND project_id = $2
RETURNING *;

-- name: GetCardInstance :one
SELECT * FROM card_instances WHERE id = $1;

-- name: ListCardInstancesByProject :many
SELECT * FROM card_instances
WHERE project_id = $1
ORDER BY created_at, id;

-- Thread-scoped card_instances (Slice 11): thread_id set, task_id/project_id NULL.

-- name: CreateThreadCardInstance :one
INSERT INTO card_instances (thread_id, card_id, status)
VALUES ($1, $2, $3)
RETURNING *;

-- name: ListCardInstancesByThread :many
SELECT * FROM card_instances WHERE thread_id = $1 ORDER BY created_at, id;

-- name: SubmitThreadCardInstance :one
UPDATE card_instances SET field_values = $3, event_trace = $4, status = $5
WHERE id = $1 AND thread_id = $2
RETURNING *;

-- name: SetThreadCardInstanceStatus :one
UPDATE card_instances SET status = $3
WHERE id = $1 AND thread_id = $2
RETURNING *;

-- Course session scope (Slice 12): mirrors the thread-scoped queries above
-- exactly (card_instances has no material_id/tool_id column — the offer's
-- material link is carried in Go's CardOffer struct, not the row).

-- name: CreateSessionCardInstance :one
INSERT INTO card_instances (session_id, card_id, status)
VALUES ($1, $2, $3)
RETURNING *;

-- name: ListCardInstancesBySession :many
SELECT * FROM card_instances WHERE session_id = $1 ORDER BY created_at, id;

-- name: SubmitSessionCardInstance :one
UPDATE card_instances
SET field_values = $3, event_trace = $4, status = $5, completed_at = now()
WHERE id = $1 AND session_id = $2
RETURNING *;

-- name: SetSessionCardInstanceStatus :one
UPDATE card_instances SET status = $3
WHERE id = $1 AND session_id = $2
RETURNING *;

-- name: ListCollectedCardsByUser :many
-- Every tool card the caller has COMPLETED at least once, across all three
-- scopes, deduped to one row per card_id with a usage summary. status='completed'
-- only (a skip is a decline, matching collectedCourseSessionCards). Owner-filtered
-- through each scope's own parent join (card_instances has no user_id). No cost,
-- no model — a pure read for the 工具卡 tab. created_at (always non-null) is the
-- usage timestamp; completed_at is only set on the session-scope path, so it is
-- not used here.
SELECT card_id,
       count(*)::int AS uses,
       array_agg(DISTINCT surface ORDER BY surface)::text[] AS surfaces,
       max(created_at) AS last_used
FROM (
  (SELECT ci.card_id AS card_id, 'project'::text AS surface, ci.created_at AS created_at
   FROM card_instances ci JOIN project p ON p.id = ci.project_id
   WHERE ci.project_id IS NOT NULL AND ci.status = 'completed' AND p.user_id = @user_id)
  UNION ALL
  (SELECT ci.card_id, 'course'::text, ci.created_at
   FROM card_instances ci JOIN course_session cs ON cs.id = ci.session_id
   WHERE ci.session_id IS NOT NULL AND ci.status = 'completed' AND cs.user_id = @user_id)
  UNION ALL
  (SELECT ci.card_id, 'chat'::text, ci.created_at
   FROM card_instances ci JOIN chat_thread t ON t.id = ci.thread_id
   WHERE ci.thread_id IS NOT NULL AND ci.status = 'completed' AND t.user_id = @user_id)
) rows
GROUP BY card_id
ORDER BY uses DESC, last_used DESC;

-- name: CountCompletedCardUsesByUser :one
-- How many times this student has COMPLETED this specific card, PROJECT
-- SCOPE ONLY — user-scoped across ALL her projects (a second project must
-- not reset her to novice), but deliberately NOT unioned with the
-- course_session/chat_thread scopes ListCollectedCardsByUser above unions.
--
-- Whole-branch review IMPORTANT 2 (N3c): this feeds the guidance fade
-- (agent/guidance.go), which removes AI scaffolding (the question, then the
-- located span) the more completions it counts. Only the project scope's
-- "completed" is gated on the card's own completion predicate
-- (projectcards.go's CompleteCard runs it before flipping status) — it
-- genuinely reflects work the card was for. chat.go and course_session.go
-- both write status='completed' UNCONDITIONALLY on submit, with no
-- predicate and no anchors at all (chat/course cards are thin by design):
-- a student can submit an anchor-less chat CRAAP sheet twice and have her
-- FIRST-EVER Studio CRAAP card surface at max scaffold removed, having never
-- once done the card properly. Counting those two ungated surfaces here
-- would make the ladder's premise false, so this query counts project-scope
-- completions only — see the spec's own §3 for the full reasoning. A skip
-- is a decline and does not count either way.
SELECT count(*)::int FROM (
  SELECT ci.id FROM card_instances ci JOIN project p ON p.id = ci.project_id
  WHERE ci.project_id IS NOT NULL AND ci.status = 'completed'
    AND p.user_id = @user_id AND ci.card_id = @card_id
) rows;
