-- name: CreateProject :one
INSERT INTO project (user_id, qualification, title, deadline, board_cfg_ver)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetProject :one
SELECT * FROM project WHERE id = $1;

-- name: ListProjectsByUser :many
SELECT * FROM project
WHERE user_id = $1
ORDER BY last_active_at DESC;

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
