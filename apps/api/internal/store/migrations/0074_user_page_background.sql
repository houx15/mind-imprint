-- +goose Up
-- The student's chosen page background colorway. Follows the users.card_theme /
-- avatar_color precedent (a per-user display preference on the users row, not a
-- settings table). The frontend overrides the --mk-paper CSS variable per preset
-- (warm paper by default, plus pure white / cream / soft blue / soft green /
-- cool grey). Added LAST on the row; only two narrow queries
-- (GetUserPageBackground / SetUserPageBackground) touch it. It does land in the
-- sqlc User struct (GetUserByID is SELECT *), which is additive and harmless.
ALTER TABLE users ADD COLUMN page_background text NOT NULL DEFAULT 'paper'
    CHECK (page_background IN ('paper', 'white', 'cream', 'blue', 'green', 'slate'));

-- +goose Down
ALTER TABLE users DROP COLUMN page_background;
