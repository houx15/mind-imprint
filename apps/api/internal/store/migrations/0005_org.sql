-- +goose Up
-- Teacher invite codes: an admin mints a single-use, school-scoped code; a teacher
-- self-signs-up with it. Plaintext + distributable (like classes.join_code).
CREATE TABLE teacher_invites (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id   uuid NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    code        text NOT NULL UNIQUE,
    email       text,
    created_by  uuid NOT NULL REFERENCES users(id),
    expires_at  timestamptz NOT NULL,
    consumed_at timestamptz,
    consumed_by uuid REFERENCES users(id),
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_teacher_invites_school ON teacher_invites (school_id);

-- Who created the class (teacher or admin). Nullable: the seeded Demo Class has none.
ALTER TABLE classes ADD COLUMN created_by uuid REFERENCES users(id);

-- +goose Down
ALTER TABLE classes DROP COLUMN IF EXISTS created_by;
DROP TABLE IF EXISTS teacher_invites;
