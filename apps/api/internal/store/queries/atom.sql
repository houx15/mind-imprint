-- name: CreateAtom :one
INSERT INTO atom (kind, user_id) VALUES ($1, $2) RETURNING *;

-- name: GetAtom :one
SELECT * FROM atom WHERE id = $1;

-- name: AppendAtomMessage :one
INSERT INTO atom_message (atom_id, seq, role, content)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListAtomMessages :many
SELECT * FROM atom_message WHERE atom_id = $1 ORDER BY seq;

-- name: NextAtomMessageSeq :one
-- The next free seq for this atom. Callers append inside the same transaction
-- as this read, so the (atom_id, seq) unique index — not this read — is the
-- real guard against a concurrent double-append.
SELECT COALESCE(MAX(seq), 0)::int + 1 AS next FROM atom_message WHERE atom_id = $1;

-- name: CreateAtomCard :one
INSERT INTO atom_card (atom_id, card_id, block_id, status, field_values, event_trace)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetAtomCard :one
SELECT * FROM atom_card WHERE id = $1;

-- name: ListAtomCards :many
SELECT * FROM atom_card WHERE atom_id = $1 ORDER BY created_at;

-- name: UpdateAtomCardStatus :one
UPDATE atom_card SET status = $2 WHERE id = $1 RETURNING *;

-- name: SubmitAtomCard :one
UPDATE atom_card
SET status = 'submitted', field_values = $2, event_trace = $3, submitted_at = now()
WHERE id = $1
RETURNING *;

-- name: CreateAtomAnnotation :one
INSERT INTO atom_annotation (atom_id, block_id, span, quote, note)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListAtomAnnotations :many
SELECT * FROM atom_annotation WHERE atom_id = $1 ORDER BY created_at;
