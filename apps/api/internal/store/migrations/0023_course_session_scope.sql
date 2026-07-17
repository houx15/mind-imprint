-- +goose Up
-- Slice 12: give Course's dialogue + card runtime a session-scoped home.
-- Additive, mirroring 0022's thread scope. A material / card_instance belongs
-- to exactly one owner: a project, a chat thread, a course session, or a
-- legacy task. course_step/course_progress are NOT touched — they are authored
-- content + page position; the phase layer lives here (DEC-12.1).
CREATE TABLE course_session (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL REFERENCES users(id)  ON DELETE CASCADE,
    course_id  uuid NOT NULL REFERENCES course(id) ON DELETE CASCADE,
    skill_id   text NOT NULL,
    phase      text NOT NULL,
    status     text NOT NULL DEFAULT 'active' CHECK (status IN ('active','finished')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, course_id)
);

CREATE TABLE course_message (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id uuid NOT NULL REFERENCES course_session(id) ON DELETE CASCADE,
    phase      text NOT NULL,
    role       text NOT NULL CHECK (role IN ('student','assistant')),
    content    text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX course_message_session_created_idx ON course_message (session_id, created_at);

ALTER TABLE material       ADD COLUMN session_id uuid REFERENCES course_session(id) ON DELETE CASCADE;
ALTER TABLE card_instances ADD COLUMN session_id uuid REFERENCES course_session(id) ON DELETE CASCADE;
CREATE INDEX material_session_created_idx       ON material (session_id, created_at);
CREATE INDEX card_instances_session_created_idx ON card_instances (session_id, created_at);

ALTER TABLE material       DROP CONSTRAINT material_scope_ck;
ALTER TABLE card_instances DROP CONSTRAINT card_instances_scope_ck;
ALTER TABLE material       ADD CONSTRAINT material_scope_ck
  CHECK (num_nonnulls(task_id, project_id, thread_id, session_id) >= 1);
ALTER TABLE card_instances ADD CONSTRAINT card_instances_scope_ck
  CHECK (num_nonnulls(task_id, project_id, thread_id, session_id) >= 1);

-- +goose Down
ALTER TABLE material       DROP CONSTRAINT IF EXISTS material_scope_ck;
ALTER TABLE card_instances DROP CONSTRAINT IF EXISTS card_instances_scope_ck;
ALTER TABLE material       ADD CONSTRAINT material_scope_ck
  CHECK (num_nonnulls(task_id, project_id, thread_id) >= 1);
ALTER TABLE card_instances ADD CONSTRAINT card_instances_scope_ck
  CHECK (num_nonnulls(task_id, project_id, thread_id) >= 1);
DROP INDEX IF EXISTS material_session_created_idx;
DROP INDEX IF EXISTS card_instances_session_created_idx;
ALTER TABLE material       DROP COLUMN IF EXISTS session_id;
ALTER TABLE card_instances DROP COLUMN IF EXISTS session_id;
DROP TABLE IF EXISTS course_message;
DROP TABLE IF EXISTS course_session;
