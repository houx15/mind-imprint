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
