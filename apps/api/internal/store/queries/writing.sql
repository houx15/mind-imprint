-- Slice 8 (S5): the silent edit buffer (one row per project) + immutable
-- draft snapshots. The buffer is student-owned scratch; a snapshot is an
-- immutable commit. The AI has no write path to either (RL-1) — only the
-- owning student's PUT /buffer and POST /snapshots reach these.

-- name: UpsertEditBuffer :exec
INSERT INTO edit_buffer (project_id, content)
VALUES ($1, $2)
ON CONFLICT (project_id)
DO UPDATE SET content = EXCLUDED.content, updated_at = now();

-- name: GetEditBuffer :one
SELECT content FROM edit_buffer WHERE project_id = $1;

-- name: InsertDraftSnapshot :one
INSERT INTO draft_snapshot (project_id, seq, content, span_index)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetLatestSnapshot :one
SELECT * FROM draft_snapshot
WHERE project_id = $1
ORDER BY seq DESC
LIMIT 1;

-- name: GetSnapshot :one
SELECT * FROM draft_snapshot WHERE id = $1 AND project_id = $2;

-- name: NextSnapshotSeq :one
SELECT COALESCE(MAX(seq), 0) + 1 AS next FROM draft_snapshot WHERE project_id = $1;
