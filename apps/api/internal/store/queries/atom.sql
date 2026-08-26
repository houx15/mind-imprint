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
-- ON CONFLICT DO NOTHING against atom_card_one_open_idx (0096): if another
-- request opened a lens between this caller's check and this insert, we get
-- pgx.ErrNoRows instead of a second open card. The handlers translate that
-- into "someone just opened one" — the SAME answer their own pre-check gives,
-- so the race and the ordinary case are indistinguishable to the student.
-- A row that is already terminal ('submitted'/'skipped') is not in the index,
-- so it can never conflict.
INSERT INTO atom_card (atom_id, card_id, block_id, status, field_values, event_trace, anchors)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (atom_id) WHERE status IN ('proposed', 'active') DO NOTHING
RETURNING *;

-- name: GetAtomCard :one
SELECT * FROM atom_card WHERE id = $1;

-- name: ListAtomCards :many
SELECT * FROM atom_card WHERE atom_id = $1 ORDER BY created_at;

-- name: UpdateAtomCardStatus :one
UPDATE atom_card SET status = $2 WHERE id = $1 RETURNING *;

-- name: SubmitAtomCard :one
-- anchors is REPLACED here, not merged: the student's own picked sentence
-- supersedes the AI's example the summon hung the card on. framework_fill is
-- untouched, so the selection review recorded by the evaluate endpoint
-- survives the submit (过程即数据).
UPDATE atom_card
SET status = 'submitted', field_values = $2, event_trace = $3, anchors = $4, submitted_at = now()
WHERE id = $1
RETURNING *;

-- name: SetAtomCardFramework :one
-- The selection review (agent.EvaluateSelection) for this card, persisted so
-- the AI's judgment of the student's picked sentence is a recorded fact and
-- not just browser state. Never flips status — evaluate is a read-with-a-model,
-- the confirm step is what submits.
UPDATE atom_card SET framework_fill = $2 WHERE id = $1 RETURNING *;

-- name: CreateAtomAnnotation :one
INSERT INTO atom_annotation (atom_id, block_id, span, quote, note)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListAtomAnnotations :many
SELECT * FROM atom_annotation WHERE atom_id = $1 ORDER BY created_at;
