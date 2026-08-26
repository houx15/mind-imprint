-- name: CreateReading :one
INSERT INTO reading (atom_id, title, lang) VALUES ($1, $2, $3) RETURNING *;

-- name: GetReading :one
SELECT * FROM reading WHERE atom_id = $1;

-- name: ListReadingsByUser :many
-- The list the 阅读 tab shows. Joins atom for ownership + creation order.
--
-- The LEFT JOIN on reading_source is what keeps the landing's first paint to
-- ONE query: hasSource used to be answered by a GetReadingSource per reading
-- (a textbook N+1 — and it pulled each article's whole BODY across the wire
-- only to test the row's existence). Existence is all the DTO needs, so it is
-- computed here.
SELECT r.*,
       a.created_at AS atom_created_at,
       a.last_activity_at,
       (s.atom_id IS NOT NULL)::bool AS has_source
FROM reading r
JOIN atom a ON a.id = r.atom_id
LEFT JOIN reading_source s ON s.atom_id = r.atom_id
WHERE a.user_id = $1 AND a.kind = 'reading'
ORDER BY a.created_at DESC;

-- name: RenameReading :exec
UPDATE reading SET title = $2, updated_at = now() WHERE atom_id = $1;

-- name: SetReadingFinished :exec
-- Guarded on status, so a SECOND POST /finish is a genuine no-op rather than a
-- re-stamp. finished_at is a fact about when she finished; an idempotent
-- endpoint that quietly moves it lets a completion time drift after the fact,
-- and 铁律④ makes that timestamp evidence like everything else on the atom.
UPDATE reading SET status = 'finished', finished_at = now(), updated_at = now()
WHERE atom_id = $1 AND status <> 'finished';

-- name: UpsertReadingSource :one
INSERT INTO reading_source (atom_id, title, body, source_url)
VALUES ($1, $2, $3, $4)
ON CONFLICT (atom_id) DO UPDATE
  SET title = EXCLUDED.title, body = EXCLUDED.body,
      source_url = EXCLUDED.source_url, ingested_at = now()
RETURNING *;

-- name: GetReadingSource :one
SELECT * FROM reading_source WHERE atom_id = $1;

-- name: UpsertReadingBrief :one
INSERT INTO reading_brief (atom_id, phase_tag, reading_reason, reading_focus)
VALUES ($1, $2, $3, $4)
ON CONFLICT (atom_id) DO UPDATE
  SET phase_tag = EXCLUDED.phase_tag,
      reading_reason = EXCLUDED.reading_reason,
      reading_focus = EXCLUDED.reading_focus,
      updated_at = now()
RETURNING *;

-- name: GetReadingBrief :one
SELECT * FROM reading_brief WHERE atom_id = $1;

-- name: UpsertReadingTakeaway :one
INSERT INTO reading_takeaway (atom_id, text)
VALUES ($1, $2)
ON CONFLICT (atom_id) DO UPDATE SET text = EXCLUDED.text, updated_at = now()
RETURNING *;

-- name: GetReadingTakeaway :one
SELECT * FROM reading_takeaway WHERE atom_id = $1;
