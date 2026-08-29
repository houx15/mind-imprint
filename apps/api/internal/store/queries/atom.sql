-- name: CreateAtom :one
INSERT INTO atom (kind, user_id) VALUES ($1, $2) RETURNING *;

-- name: GetAtom :one
SELECT * FROM atom WHERE id = $1;

-- name: TouchAtom :one
-- Bumps last_activity_at (0098). Called from the ONE write chokepoint every
-- lite per-id route funnels through (loadOwnedReadingAtom), so "she was here"
-- can never drift out of sync with "she wrote something" the way
-- reading.updated_at did — that column moved only on rename and finish, so an
-- hour of reading left 上次读到 pointing at the day the reading was created.
-- Returns the refreshed row so the caller's atom is never one write stale.
UPDATE atom SET last_activity_at = now() WHERE id = $1 RETURNING *;

-- name: CountAtomEvidence :one
-- How many rows of PROCESS EVIDENCE hang off this atom: cards, margin notes,
-- transcript turns. All three are anchored INTO the article — block ids are
-- positional and anchors carry rune offsets — so replacing the article under
-- them would silently re-point every one at unrelated prose. 铁律④ makes
-- these rows evidence, so putReadingSourceLite refuses the replacement once
-- this is non-zero rather than corrupting them.
SELECT (
    (SELECT count(*) FROM atom_card c WHERE c.atom_id = sqlc.arg(atom_id))
  + (SELECT count(*) FROM atom_annotation an WHERE an.atom_id = sqlc.arg(atom_id))
  + (SELECT count(*) FROM atom_message m WHERE m.atom_id = sqlc.arg(atom_id))
)::bigint AS n;

-- name: AppendAtomMessage :one
-- payload (0106) 是这条消息随身带的结构化东西：AI 侧是它现场写的那张聊天卡片，
-- 学生侧是她在卡片上的回答。绝大多数消息只有 content，payload 传 NULL——它可空
-- 正是为了让「这条消息什么也没带」是默认状态，而不是每个调用方都要构造一个空壳。
INSERT INTO atom_message (atom_id, seq, role, content, payload)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListAtomMessages :many
-- The room's OWN thread only. The `block_id IS NULL` filter is the whole safety
-- property of 0102: seven callers across reading and writing read this query and
-- every one of them means "the main conversation". Making the default safe is why
-- none of them needed editing when block scoping arrived — do not remove it, and
-- do not add a variant that omits it.
SELECT * FROM atom_message WHERE atom_id = $1 AND block_id IS NULL ORDER BY seq;

-- name: ListAtomBlockMessages :many
SELECT * FROM atom_message
WHERE atom_id = $1 AND block_id = $2
ORDER BY seq;

-- name: AppendAtomBlockMessage :one
-- 没有 payload：聊天卡片长在房间自己那条主线程上（block_id IS NULL），段落
-- 子对话不发卡。真需要时再加参数，而不是先摆一个永远传 NULL 的洞在这里。
INSERT INTO atom_message (atom_id, seq, role, content, block_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

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
--
-- origin (0097) is NOT optional at this seam: it is the ONLY record of whether
-- the AI proposed this lens or the student picked it herself out of the 透镜库,
-- and it cannot be reconstructed from anything else on the row. Both call
-- sites pass it explicitly (reading_turn.go → 'router', reading_lens.go →
-- 'student') so a new creation site cannot inherit a silent default.
INSERT INTO atom_card (atom_id, card_id, block_id, status, field_values, event_trace, anchors, origin)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
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

-- name: GetAtomReport :one
SELECT * FROM atom_report WHERE atom_id = $1;

-- name: UpsertAtomReport :one
INSERT INTO atom_report (atom_id, kind, report)
VALUES ($1, $2, $3)
ON CONFLICT (atom_id) DO UPDATE SET report = EXCLUDED.report
RETURNING *;

-- name: SetAtomReportShare :one
-- The ::text cast on the CASE branch is load-bearing, not decoration: without
-- it Postgres cannot infer sqlc.narg(share_token)'s type from an unqualified
-- "$1 IS NULL" test alone (SQLSTATE 42P08, "could not determine data type of
-- parameter $1") even though the SET target above pins the same $1 to text —
-- reproduced directly against postgres:16 with a plain PREPARE (no explicit
-- param types), the exact shape pgx's Parse step uses.
UPDATE atom_report
SET share_token = sqlc.narg(share_token),
    shared_at = CASE WHEN sqlc.narg(share_token)::text IS NULL THEN NULL ELSE now() END
WHERE atom_id = sqlc.arg(atom_id)
RETURNING *;

-- name: GetAtomReportByShareToken :one
SELECT * FROM atom_report WHERE share_token = $1;

-- name: AddAtomActiveSeconds :exec
UPDATE atom SET active_seconds = active_seconds + $2 WHERE id = $1;
