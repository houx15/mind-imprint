-- +goose Up
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE schools (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE classes (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id  uuid NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    name       text NOT NULL,
    join_code  text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX classes_join_code_key ON classes (join_code);
CREATE INDEX classes_school_id_idx ON classes (school_id);

CREATE TABLE users (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email             text NOT NULL,
    email_verified_at timestamptz,
    password_hash     text NOT NULL,
    role              text NOT NULL DEFAULT 'student' CHECK (role IN ('student','teacher','admin')),
    school_id         uuid NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    display_name      text NOT NULL,
    avatar_color      text NOT NULL,
    created_at        timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX users_email_key ON users (email);
CREATE INDEX users_school_id_idx ON users (school_id);

CREATE TABLE enrollments (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    class_id      uuid NOT NULL REFERENCES classes(id) ON DELETE RESTRICT,
    role_in_class text NOT NULL DEFAULT 'student' CHECK (role_in_class IN ('student','teacher')),
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX enrollments_user_id_idx ON enrollments (user_id);
CREATE INDEX enrollments_class_id_idx ON enrollments (class_id);
CREATE UNIQUE INDEX enrollments_user_class_key ON enrollments (user_id, class_id);

CREATE TABLE tasks (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id        uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title          text NOT NULL,
    seed           text,
    status         text NOT NULL DEFAULT 'active' CHECK (status IN ('active','evaluated')),
    created_at     timestamptz NOT NULL DEFAULT now(),
    last_active_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX tasks_user_last_active_idx ON tasks (user_id, last_active_at DESC);

CREATE TABLE messages (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id           uuid NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    role              text NOT NULL CHECK (role IN ('user','assistant','system','summary')),
    content           text NOT NULL,
    tool_call         jsonb,
    provider          text,
    model             text,
    tier              text,
    prompt_tokens     integer,
    completion_tokens integer,
    cost_estimate     numeric(12,6),
    created_at        timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX messages_task_created_id_idx ON messages (task_id, created_at, id);

CREATE TABLE card_instances (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    card_id        text NOT NULL,
    task_id        uuid NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    parent_node_id uuid,
    status         text NOT NULL CHECK (status IN ('proposed','active','completed','skipped')),
    field_values   jsonb NOT NULL DEFAULT '{}',
    event_trace    jsonb NOT NULL DEFAULT '[]',
    rubric_tags    text[] NOT NULL DEFAULT '{}',
    created_at     timestamptz NOT NULL DEFAULT now(),
    completed_at   timestamptz
);
CREATE INDEX card_instances_task_created_idx ON card_instances (task_id, created_at);
CREATE INDEX card_instances_parent_idx ON card_instances (parent_node_id);

CREATE TABLE evaluations (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id           uuid NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    scores            jsonb NOT NULL,
    narrative         text NOT NULL,
    model             text NOT NULL,
    tier              text NOT NULL,
    prompt_tokens     integer,
    completion_tokens integer,
    cost_estimate     numeric(12,6),
    status            text NOT NULL CHECK (status IN ('queued','running','done','failed')),
    error             text,
    created_at        timestamptz NOT NULL DEFAULT now(),
    completed_at      timestamptz
);
CREATE INDEX evaluations_task_created_idx ON evaluations (task_id, created_at DESC);

CREATE VIEW llm_usage AS
  SELECT m.id, t.user_id, u.school_id, 'chat'::text AS kind,
         m.provider, m.model, m.tier,
         m.prompt_tokens, m.completion_tokens, m.cost_estimate, m.created_at
    FROM messages m JOIN tasks t ON t.id = m.task_id JOIN users u ON u.id = t.user_id
   WHERE m.role = 'assistant' AND m.model IS NOT NULL
  UNION ALL
  SELECT e.id, t.user_id, u.school_id, 'eval'::text AS kind,
         NULL, e.model, e.tier,
         e.prompt_tokens, e.completion_tokens, e.cost_estimate, e.created_at
    FROM evaluations e JOIN tasks t ON t.id = e.task_id JOIN users u ON u.id = t.user_id;

-- +goose Down
DROP VIEW IF EXISTS llm_usage;
DROP TABLE IF EXISTS evaluations;
DROP TABLE IF EXISTS card_instances;
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS tasks;
DROP TABLE IF EXISTS enrollments;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS classes;
DROP TABLE IF EXISTS schools;
