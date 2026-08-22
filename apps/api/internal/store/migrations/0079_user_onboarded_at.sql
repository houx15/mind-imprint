-- +goose Up
-- When the student finished (or dismissed) the new-user guided tour. NULL = never
-- onboarded → the welcome modal fires. Follows the users.email_verified_at precedent
-- (a nullable timestamptz on the users row).
ALTER TABLE users ADD COLUMN onboarded_at timestamptz;

-- +goose Down
ALTER TABLE users DROP COLUMN onboarded_at;
