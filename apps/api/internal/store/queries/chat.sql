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
