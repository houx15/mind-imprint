-- name: InsertEvaluationReport :one
INSERT INTO evaluation_report (project_id, version, report)
VALUES (@project_id, @version, @report)
RETURNING *;

-- name: GetLatestEvaluationReport :one
SELECT * FROM evaluation_report
WHERE project_id = @project_id
ORDER BY created_at DESC
LIMIT 1;

-- name: ListEvaluationReports :many
-- Timeline for one student: newest report per finished project they own.
SELECT DISTINCT ON (er.project_id)
  er.project_id, er.created_at,
  p.title, p.qualification
FROM evaluation_report er
JOIN project p ON p.id = er.project_id
WHERE p.user_id = @user_id
ORDER BY er.project_id, er.created_at DESC;
