-- name: GetProjectProposal :one
SELECT * FROM project_proposal WHERE project_id = $1;

-- name: UpsertProjectProposal :one
INSERT INTO project_proposal (project_id, objective, reason, activities, resources, updated_at)
VALUES ($1, $2, $3, $4, $5, now())
ON CONFLICT (project_id) DO UPDATE SET
    objective  = EXCLUDED.objective,
    reason     = EXCLUDED.reason,
    activities = EXCLUDED.activities,
    resources  = EXCLUDED.resources,
    updated_at = now()
RETURNING *;
