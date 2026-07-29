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
-- thread. Mirrors CreateChatMessage but carries the active surface.
INSERT INTO chat_message (thread_id, role, content, modality, surface)
VALUES ($1, $2, $3, 'text', $4)
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

-- name: FoldChatSurface :exec
-- Lever 1 (compaction): fold every live turn on the named surfaces into the
-- spine the moment an artifact solidifies (proposal finalized / plan generated).
-- The turns stay in the thread; they just leave the active context window.
UPDATE chat_message
SET folded_at = now()
WHERE thread_id = (SELECT id FROM chat_thread WHERE seeded_project_id = $1 LIMIT 1)
  AND surface = ANY($2::text[])
  AND folded_at IS NULL;

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
