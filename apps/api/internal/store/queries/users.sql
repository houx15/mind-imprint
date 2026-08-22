-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserCardTheme :one
-- The student's chosen 工具卡图鉴 cover colorway (see migration 0047). Narrow
-- read, does not touch the full User row.
SELECT card_theme FROM users WHERE id = @user_id;

-- name: SetUserCardTheme :exec
-- Set the student's cover colorway. The 4-value CHECK is enforced by the column;
-- the handler validates against cards.ValidTheme before calling.
UPDATE users SET card_theme = @card_theme WHERE id = @user_id;

-- name: GetUserPageBackground :one
-- The student's chosen page background colorway (see migration 0074). Narrow
-- read, does not touch the full User row.
SELECT page_background FROM users WHERE id = @user_id;

-- name: SetUserPageBackground :exec
-- Set the student's page background. The 6-value CHECK is enforced by the column;
-- the handler validates against the preset allowlist before calling.
UPDATE users SET page_background = @page_background WHERE id = @user_id;

-- name: SetUserAvatarColor :exec
-- Persists the student's chosen accent preset id into the existing
-- avatar_color column. The handler validates against the 8-preset allowlist
-- before calling; no CHECK constraint on this column.
UPDATE users SET avatar_color = @avatar_color WHERE id = @user_id;

-- name: SetUserOnboardedAt :exec
-- Stamps the moment the student completed/dismissed onboarding. Idempotent enough for
-- our use (re-running just refreshes the timestamp).
UPDATE users SET onboarded_at = now() WHERE id = @user_id;
