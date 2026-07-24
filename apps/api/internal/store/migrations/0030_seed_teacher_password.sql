-- +goose Up
-- Give the seeded teacher persona (吴老师, migration 0029) a real dev login.
-- 0029 left her password_hash as the placeholder 'SEED_NO_LOGIN', which fails
-- the PHC format check, so the demo's teacher persona was unreachable and the
-- role_in_class='teacher' branch of assertTeacherOwnsClass was never
-- exercised outside tests. Reuses the SAME hash value as phoebe's
-- (0004_seed_password.sql) so both dev personas share one documented
-- password, rather than minting a second one to track.
-- Dev login: wu.teacher@demo.mindimprint.local / phoebe-dev-pass.
UPDATE users
SET password_hash = '$argon2id$v=19$m=65536,t=1,p=4$gC7Avd040JYFP+T01dmfYA$EklK2zWmwF/0j+0BjUedJL7IFVACSZytU1Wy2e5g8yw'
WHERE id = '00000000-0000-0000-0000-000000000910';

-- +goose Down
UPDATE users
SET password_hash = 'SEED_NO_LOGIN'
WHERE id = '00000000-0000-0000-0000-000000000910';
