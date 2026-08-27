-- Writing's own atom-kind tables (0099): writing / writing_outline /
-- writing_snippet / writing_draft. Named writing_atom.sql, not writing.sql —
-- queries/writing.sql already exists for the unrelated pro-product edit
-- buffer + draft snapshot feature (project_id-scoped, Slice 8/S5). The two
-- "writing"s share a word, not a table or a domain.

-- name: CreateWriting :one
INSERT INTO writing (atom_id, title, lang) VALUES ($1, $2, $3) RETURNING *;

-- name: GetWriting :one
SELECT * FROM writing WHERE atom_id = $1;

-- name: ListWritingsByUser :many
-- The list the 写作 tab shows. Joins atom for ownership + creation order,
-- same shape as ListReadingsByUser — atom_created_at rides along so the API
-- layer can fill writingDTO.createdAt without an N+1 GetAtom per row (the
-- writing table itself has no created_at column; only atom does).
SELECT w.*, a.created_at AS atom_created_at
FROM writing w
JOIN atom a ON a.id = w.atom_id
WHERE a.user_id = $1 AND a.kind = 'writing'
ORDER BY a.created_at DESC;

-- name: RenameWriting :exec
UPDATE writing SET title = $2, updated_at = now() WHERE atom_id = $1;

-- name: SetWritingStage :one
-- No gate here on purpose (§6.2.3): stages are a map shown to the student,
-- not a checkpoint the server enforces, so any legal value is accepted in any
-- order. The CHECK constraint is the only guard.
UPDATE writing SET stage = $2, updated_at = now() WHERE atom_id = $1 RETURNING *;

-- name: SetWritingTargetWords :exec
UPDATE writing SET target_words = $2, updated_at = now() WHERE atom_id = $1;

-- name: SetWritingFinished :exec
-- Guarded on status, like SetReadingFinished: a second /finish is a genuine
-- no-op so finished_at never drifts after the fact (铁律④ makes it evidence).
-- Deliberately does NOT force stage to 'finished' — a student who skipped
-- straight to a finished draft without ever touching 提纲/片段 keeps that
-- fact recorded in stage for the report, rather than having it silently
-- overwritten at finish time.
UPDATE writing SET status = 'finished', finished_at = now(), updated_at = now()
WHERE atom_id = $1 AND status <> 'finished';

-- name: ReplaceWritingOutline :many
-- Full replace, not a diff: the whole outline is written as one shape each
-- PUT (学生可改 the derived outline wholesale), so delete-then-insert keeps
-- position contiguous and never leaves a stale tail row behind. The DELETE
-- and INSERT run as ONE statement (a data-modifying CTE), so a concurrent
-- reader never observes a momentarily-empty outline between the two.
WITH deleted AS (
  DELETE FROM writing_outline WHERE atom_id = sqlc.arg(atom_id)
)
INSERT INTO writing_outline (atom_id, text, depth, position)
SELECT sqlc.arg(atom_id),
       unnest(sqlc.arg(texts)::text[]),
       unnest(sqlc.arg(depths)::int[]),
       unnest(sqlc.arg(positions)::int[])
RETURNING *;

-- name: ListWritingOutline :many
SELECT * FROM writing_outline WHERE atom_id = $1 ORDER BY position;

-- name: UpsertWritingSnippet :one
-- Keyed by (atom_id, position) — writing the same slot again edits it in
-- place, it never duplicates a row (writing_snippet_atom_position_idx, 0099).
INSERT INTO writing_snippet (atom_id, outline_id, position, text)
VALUES ($1, $2, $3, $4)
ON CONFLICT (atom_id, position) DO UPDATE
  SET outline_id = EXCLUDED.outline_id, text = EXCLUDED.text, updated_at = now()
RETURNING *;

-- name: ListWritingSnippets :many
SELECT * FROM writing_snippet WHERE atom_id = $1 ORDER BY position;

-- name: UpsertWritingDraft :one
INSERT INTO writing_draft (atom_id, body)
VALUES ($1, $2)
ON CONFLICT (atom_id) DO UPDATE SET body = EXCLUDED.body, updated_at = now()
RETURNING *;

-- name: GetWritingDraft :one
SELECT * FROM writing_draft WHERE atom_id = $1;
