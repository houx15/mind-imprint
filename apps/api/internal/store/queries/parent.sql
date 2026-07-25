-- name: GetParentReportProse :one
SELECT prose FROM parent_report_prose
WHERE student_user_id = $1 AND surface = $2 AND scope_id = $3;

-- name: InsertParentReportProse :exec
INSERT INTO parent_report_prose (student_user_id, surface, scope_id, prose)
VALUES ($1, $2, $3, $4)
ON CONFLICT (student_user_id, surface, scope_id) DO NOTHING;
