-- Task 10 · nav 反馈 button — free-text feedback saved to its own table.

-- name: CreateFeedback :one
INSERT INTO feedback (user_id, text)
VALUES ($1, $2)
RETURNING *;
