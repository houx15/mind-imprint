-- +goose Up
-- 工具卡图鉴: the student's chosen cover colorway. Follows the users.avatar_color
-- precedent (a per-user display preference on the users row, not a settings
-- table). Added LAST on the row; deliberately NOT added to the sqlc User struct
-- or any full-row SELECT — only two narrow queries (GetUserCardTheme /
-- SetUserCardTheme) touch it, so every existing users query is unaffected.
ALTER TABLE users ADD COLUMN card_theme text NOT NULL DEFAULT 'light'
    CHECK (card_theme IN ('light', 'cyber-sage', 'cyber-slate', 'cyber-warm'));

-- +goose Down
ALTER TABLE users DROP COLUMN card_theme;
