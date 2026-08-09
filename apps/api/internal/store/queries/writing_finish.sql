-- Phase B (status-router redesign) · the per-document 完成写作 milestone,
-- replacing the scalar project.writing_finished_at. One finish row per
-- (project, doc_kind). Finishing locks that document read-only; the essay row
-- gates 定稿评估 (finishProject). Reversible (铁律②: 重新打开写作 deletes the row).

-- name: SetWritingFinish :exec
INSERT INTO writing_finish (project_id, doc_kind)
VALUES ($1, $2)
ON CONFLICT (project_id, doc_kind)
DO UPDATE SET finished_at = now();

-- name: ClearWritingFinish :exec
DELETE FROM writing_finish WHERE project_id = $1 AND doc_kind = $2;

-- name: GetWritingFinish :one
SELECT finished_at FROM writing_finish WHERE project_id = $1 AND doc_kind = $2;

-- name: ListWritingFinish :many
SELECT doc_kind, finished_at FROM writing_finish WHERE project_id = $1;
