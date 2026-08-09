-- Slice 8 (S5): the silent edit buffer + immutable draft snapshots. Phase B keys
-- both on doc_kind ('proposal'|'essay') so the 立项 proposal and the essay are
-- distinct documents (one live buffer + one snapshot sequence PER doc). The AI
-- has no write path to either (RL-1) — only the owning student's PUT /buffer and
-- POST /snapshots reach these.

-- name: UpsertEditBuffer :exec
INSERT INTO edit_buffer (project_id, doc_kind, content)
VALUES ($1, $2, $3)
ON CONFLICT (project_id, doc_kind)
DO UPDATE SET content = EXCLUDED.content, updated_at = now();

-- name: GetEditBuffer :one
SELECT content FROM edit_buffer WHERE project_id = $1 AND doc_kind = $2;

-- name: InsertDraftSnapshot :one
INSERT INTO draft_snapshot (project_id, doc_kind, seq, content, span_index)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetLatestSnapshot :one
SELECT * FROM draft_snapshot
WHERE project_id = $1 AND doc_kind = $2
ORDER BY seq DESC
LIMIT 1;

-- name: GetSnapshot :one
SELECT * FROM draft_snapshot WHERE id = $1 AND project_id = $2;

-- name: NextSnapshotSeq :one
SELECT COALESCE(MAX(seq), 0) + 1 AS next FROM draft_snapshot WHERE project_id = $1 AND doc_kind = $2;
