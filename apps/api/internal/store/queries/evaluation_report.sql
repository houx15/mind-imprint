-- name: ClaimEvaluationReportGeneration :one
INSERT INTO evaluation_report (project_id, version, status, report)
VALUES (@project_id, 1, 'generating', NULL)
ON CONFLICT (project_id) DO UPDATE
  SET status = 'generating', report = NULL, created_at = now()
  WHERE evaluation_report.status = 'failed'
     OR (evaluation_report.status = 'generating'
         AND evaluation_report.created_at < now() - interval '30 minutes')
RETURNING id;

-- name: CompleteEvaluationReport :exec
UPDATE evaluation_report SET report = @report, status = 'ready' WHERE project_id = @project_id;

-- name: FailEvaluationReport :exec
UPDATE evaluation_report SET status = 'failed' WHERE project_id = @project_id AND status = 'generating';

-- name: GetEvaluationReport :one
SELECT * FROM evaluation_report WHERE project_id = @project_id;

-- name: ListEvaluationReports :many
-- Timeline for one student: only fully-generated reports.
SELECT er.project_id, er.created_at, p.title, p.qualification
FROM evaluation_report er
JOIN project p ON p.id = er.project_id
WHERE p.user_id = @user_id AND er.status = 'ready'
ORDER BY er.created_at DESC;
