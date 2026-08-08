-- chat_thread/chat_message queries (Slice 5c, agent conversational loop).
-- chat_message has no project_id of its own — it joins through
-- chat_thread.seeded_project_id, which is nullable (pgtype.UUID param).

-- name: GetThreadByProject :one
SELECT * FROM chat_thread WHERE seeded_project_id = $1 LIMIT 1;

-- name: CreateThread :one
INSERT INTO chat_thread (user_id, seeded_project_id) VALUES ($1, $2)
RETURNING *;

-- name: CreateChatMessage :one
INSERT INTO chat_message (thread_id, role, content, modality)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListChatMessagesByProject :many
SELECT cm.* FROM chat_message cm
JOIN chat_thread ct ON cm.thread_id = ct.id
WHERE ct.seeded_project_id = $1
ORDER BY cm.created_at, cm.id;

-- S1 · one continuous per-project session. The four-room coach persists both
-- sides to this thread, surface-tagged; folded turns stay in the thread (shown
-- on reload) but drop out of the coach's active context window.

-- name: CreateProjectCoachMessage :one
-- Persist one surface-tagged coach turn (role user|assistant) to the project's
-- thread. Mirrors CreateChatMessage but carries the active surface. attachments
-- reuses the existing jsonb column to carry a card-turn's structured reference
-- ({"card":{"cardId","fieldValues"}}), so a reloaded thread re-renders a
-- completed card as a clickable chip; '[]' for a plain turn. stage carries the
-- studio_state.stage in effect at persist time (nullable — "" -> NULL from Go).
INSERT INTO chat_message (thread_id, role, content, modality, surface, attachments, stage)
VALUES ($1, $2, $3, 'text', $4, $5, sqlc.narg('stage'))
RETURNING *;

-- name: ListActiveChatMessagesByProject :many
-- The coach's CONTEXT window: every non-folded turn on the project's thread,
-- both roles, oldest→newest. Continuity ignores surface (whole thread); Go caps
-- to the last N.
SELECT cm.* FROM chat_message cm
JOIN chat_thread ct ON cm.thread_id = ct.id
WHERE ct.seeded_project_id = $1 AND cm.folded_at IS NULL
ORDER BY cm.created_at, cm.id;

-- name: ListChatMessagesByProjectSurface :many
-- The DISPLAY slice for one room: that surface's turns (folded included — a
-- folded turn is still part of the visible conversation).
SELECT cm.* FROM chat_message cm
JOIN chat_thread ct ON cm.thread_id = ct.id
WHERE ct.seeded_project_id = $1 AND cm.surface = $2
ORDER BY cm.created_at, cm.id;

-- name: ListChatMessagesPageLatest :many
-- Task 4 pagination: the newest page of a surface slice, newest-first (caller
-- fetches limit+1 to detect hasMore, then reverses to oldest→newest for
-- display). Folded turns are included — a folded turn is still part of the
-- visible conversation, it has just left the coach's active context window.
SELECT cm.* FROM chat_message cm
JOIN chat_thread ct ON cm.thread_id = ct.id
WHERE ct.seeded_project_id = $1 AND cm.surface = $2
ORDER BY cm.created_at DESC, cm.id DESC
LIMIT $3;

-- name: ListChatMessagesPageBefore :many
-- Task 4 pagination: the next older page, keyed off the composite cursor
-- (created_at, id) of the oldest row already shown. (created_at, id) is a
-- stable total order — id breaks created_at ties deterministically, so
-- paging never skips or duplicates a row even though ids are random UUIDs.
SELECT cm.* FROM chat_message cm
JOIN chat_thread ct ON cm.thread_id = ct.id
WHERE ct.seeded_project_id = $1 AND cm.surface = $2
  AND (cm.created_at, cm.id) < (sqlc.arg(before_created_at)::timestamptz, sqlc.arg(before_id)::uuid)
ORDER BY cm.created_at DESC, cm.id DESC
LIMIT sqlc.arg(page_limit)::int;

-- name: FoldChatSurface :exec
-- Lever 1 (compaction): fold every live turn on the named surfaces into the
-- spine the moment an artifact solidifies (proposal finalized / plan generated).
-- The turns stay in the thread; they just leave the active context window.
UPDATE chat_message
SET folded_at = now()
WHERE thread_id = (SELECT id FROM chat_thread WHERE seeded_project_id = $1 LIMIT 1)
  AND surface = ANY($2::text[])
  AND folded_at IS NULL;

-- name: SelectOldestActiveChatMessages :many
-- S4 compaction backstop's overflow: every non-folded turn on the project's
-- thread, oldest→newest, EXCLUDING the newest $2 (always kept live in the
-- window). These are the turns to compose into the digest, then fold.
SELECT cm.* FROM chat_message cm
JOIN chat_thread ct ON cm.thread_id = ct.id
WHERE ct.seeded_project_id = $1 AND cm.folded_at IS NULL
  AND cm.id NOT IN (
    SELECT cm2.id FROM chat_message cm2
    JOIN chat_thread ct2 ON cm2.thread_id = ct2.id
    WHERE ct2.seeded_project_id = $1 AND cm2.folded_at IS NULL
    ORDER BY cm2.created_at DESC, cm2.id DESC
    LIMIT $2
  )
ORDER BY cm.created_at, cm.id;

-- name: FoldChatMessagesByID :exec
-- S4 age-based fold (surface-agnostic, unlike FoldChatSurface): mark exactly the
-- overflow turns folded once their content is durable in conversation_digest.
UPDATE chat_message SET folded_at = now() WHERE id = ANY($1::uuid[]);

-- name: GetConversationDigest :one
SELECT * FROM conversation_digest WHERE project_id = $1;

-- name: UpsertConversationDigest :exec
-- One evolving digest row per project (grows as more turns fold; NOT
-- first-open-wins). turns_folded is the running count for the projection line.
INSERT INTO conversation_digest (project_id, prose, turns_folded, model, tier, updated_at)
VALUES ($1, $2, $3, $4, $5, now())
ON CONFLICT (project_id) DO UPDATE
  SET prose = EXCLUDED.prose, turns_folded = EXCLUDED.turns_folded,
      model = EXCLUDED.model, tier = EXCLUDED.tier, updated_at = now();

-- Standalone Chat surface (Slice 11): threads owned by a user, not a project.
-- The existing CreateChatMessage above is already thread-keyed and is reused.

-- name: ListThreadsByUser :many
SELECT * FROM chat_thread WHERE user_id = $1 ORDER BY created_at DESC;

-- name: CreateStandaloneThread :one
INSERT INTO chat_thread (user_id, title) VALUES ($1, $2) RETURNING *;

-- name: GetThread :one
SELECT * FROM chat_thread WHERE id = $1;

-- name: ListMessagesByThread :many
SELECT * FROM chat_message WHERE thread_id = $1 ORDER BY created_at, id;
