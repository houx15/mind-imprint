-- name: CreateProject :one
INSERT INTO project (user_id, qualification, title, deadline, board_cfg_ver, cover)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetProject :one
SELECT * FROM project WHERE id = $1;

-- name: ListProjectsByUser :many
SELECT * FROM project
WHERE user_id = $1
ORDER BY last_active_at DESC;

-- name: CountLLMCallsByUserProject :many
-- Per-project AI-call totals for the caller's whole project list, in ONE grouped
-- pass (not N per-project reads). llm_call carries user_id + project_id directly
-- (project_id NULL for course/chat calls, excluded here); indexed on both.
SELECT project_id, COUNT(*)::int AS n
FROM llm_call
WHERE user_id = $1 AND project_id IS NOT NULL
GROUP BY project_id;

-- name: CountActivityLogByUserProject :many
-- Per-project activity-log totals for the caller's whole project list, in ONE
-- grouped pass. activity_log_entry has no user_id, so join project to scope to
-- the caller; activity_log_entry is indexed on project_id.
SELECT a.project_id, COUNT(*)::int AS n
FROM activity_log_entry a
JOIN project p ON p.id = a.project_id
WHERE p.user_id = $1
GROUP BY a.project_id;

-- name: TouchProject :exec
UPDATE project SET last_active_at = now() WHERE id = $1;

-- name: SetProjectFinished :exec
-- A3 terminal: the first and only writer of project.status='finished'.
UPDATE project SET status = 'finished', last_active_at = now() WHERE id = $1;

-- name: SetProjectEvaluating :exec
-- BE5: the non-blocking finish flips status to 'evaluating' before spawning the
-- detached report goroutine; a second finish while 'evaluating' is refused.
UPDATE project SET status = 'evaluating', last_active_at = now() WHERE id = $1;

-- name: SetProjectActive :exec
-- BE5: the finish goroutine rolls status back to 'active' when report
-- generation fails or is rejected, so the terminal stays retryable.
UPDATE project SET status = 'active', last_active_at = now() WHERE id = $1;

-- The 完成写作 milestone moved to the per-document writing_finish table
-- (Phase B, writing_finish.sql); project.writing_finished_at is dropped in
-- migration 0061.

-- name: SetProjectTitle :exec
-- Student renames their own project (ownership is enforced in the handler).
UPDATE project SET title = $2, last_active_at = now() WHERE id = $1;

-- name: GetStudioState :one
SELECT studio_state FROM project WHERE id = $1;

-- name: SetStudioState :exec
UPDATE project SET studio_state = $2, last_active_at = now() WHERE id = $1;
