-- +goose Up
-- Slice 0 of the whole-product refactor: the project/graph/event foundation.
-- Additive only — old task/message tables and code stay untouched and dormant;
-- they are retired per surface slice as each is replaced (see roadmap).

CREATE TABLE project (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id        uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    qualification  text NOT NULL DEFAULT '',
    title          text NOT NULL,
    deadline       timestamptz,
    board_cfg_ver  integer NOT NULL DEFAULT 1,
    status         text NOT NULL DEFAULT 'active',
    created_at     timestamptz NOT NULL DEFAULT now(),
    last_active_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX project_user_last_active_idx ON project (user_id, last_active_at DESC);

-- Light argument-graph participants. Heavy participants (material, draft_snapshot,
-- card_instances) keep their own tables and are referenced polymorphically by graph_edge.
CREATE TABLE graph_node (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id uuid NOT NULL REFERENCES project(id) ON DELETE CASCADE,
    type       text NOT NULL CHECK (type IN ('claim','evidence','plan','gate_state','note')),
    body       jsonb NOT NULL DEFAULT '{}',
    author     text NOT NULL CHECK (author IN ('student','ai','imported')),
    span_ref   jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX graph_node_project_created_idx ON graph_node (project_id, created_at);

-- Polymorphic edges: (from_kind, from_id) -> (to_kind, to_id) across graph_node and
-- the heavy node kinds, so an edge can link a claim to a card_instance-derived
-- evidence node or a material without a handle row per heavy node.
CREATE TABLE graph_edge (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id uuid NOT NULL REFERENCES project(id) ON DELETE CASCADE,
    type       text NOT NULL,
    from_kind  text NOT NULL CHECK (from_kind IN ('graph_node','material','draft_snapshot','card_instance')),
    from_id    uuid NOT NULL,
    to_kind    text NOT NULL CHECK (to_kind IN ('graph_node','material','draft_snapshot','card_instance')),
    to_id      uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX graph_edge_project_created_idx ON graph_edge (project_id, created_at);
CREATE INDEX graph_edge_from_idx ON graph_edge (from_kind, from_id);
CREATE INDEX graph_edge_to_idx ON graph_edge (to_kind, to_id);

-- Immutable, span-indexed draft snapshots (the "snapshot" material).
CREATE TABLE draft_snapshot (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id uuid NOT NULL REFERENCES project(id) ON DELETE CASCADE,
    seq        integer NOT NULL,
    content    text NOT NULL,
    span_index jsonb NOT NULL DEFAULT '[]',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX draft_snapshot_project_seq_key ON draft_snapshot (project_id, seq);

-- Mutable scratch space — NOT a record, one live buffer per project.
CREATE TABLE edit_buffer (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id uuid NOT NULL REFERENCES project(id) ON DELETE CASCADE,
    content    text NOT NULL DEFAULT '',
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX edit_buffer_project_key ON edit_buffer (project_id);

CREATE TABLE source_log_entry (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id   uuid NOT NULL REFERENCES project(id) ON DELETE CASCADE,
    url          text NOT NULL,
    title        text NOT NULL DEFAULT '',
    time_spent_s integer NOT NULL DEFAULT 0,
    takeaway     text NOT NULL DEFAULT '',
    tier         text,
    lateral_read boolean NOT NULL DEFAULT false,
    opened_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX source_log_entry_project_opened_idx ON source_log_entry (project_id, opened_at);

CREATE TABLE intervention (
    id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id           uuid NOT NULL REFERENCES project(id) ON DELETE CASCADE,
    card_instance_id     uuid REFERENCES card_instances(id) ON DELETE SET NULL,
    type                 text NOT NULL,
    anchor               jsonb NOT NULL DEFAULT '{}',
    criterion            text,
    body                 text NOT NULL DEFAULT '',
    level                text,
    output_check_verdict text,
    created_at           timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX intervention_project_created_idx ON intervention (project_id, created_at);
CREATE INDEX intervention_card_instance_idx ON intervention (card_instance_id);

CREATE TABLE disposition (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    intervention_id uuid NOT NULL REFERENCES intervention(id) ON DELETE CASCADE,
    action          text NOT NULL CHECK (action IN ('accept','reject','rewrite')),
    reason          text NOT NULL DEFAULT '',
    created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX disposition_intervention_idx ON disposition (intervention_id);

-- Shared with Course: per student, per card. Not a project child.
CREATE TABLE card_competence (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    card_id          text NOT NULL,
    scaffold_state   text NOT NULL DEFAULT 'scaffolded',
    unprompted_count integer NOT NULL DEFAULT 0,
    prompted_count   integer NOT NULL DEFAULT 0,
    updated_at       timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX card_competence_user_card_key ON card_competence (user_id, card_id);

CREATE TABLE chat_thread (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title             text NOT NULL DEFAULT '',
    seeded_project_id uuid REFERENCES project(id) ON DELETE SET NULL,
    created_at        timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX chat_thread_user_created_idx ON chat_thread (user_id, created_at DESC);

CREATE TABLE chat_message (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    thread_id       uuid NOT NULL REFERENCES chat_thread(id) ON DELETE CASCADE,
    role            text NOT NULL CHECK (role IN ('user','assistant','system')),
    content         text NOT NULL DEFAULT '',
    modality        text NOT NULL DEFAULT 'text' CHECK (modality IN ('text','voice','file','image')),
    attachments     jsonb NOT NULL DEFAULT '[]',
    quoted_fragment text,
    created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX chat_message_thread_created_idx ON chat_message (thread_id, created_at);

-- Append-only event stream. No UPDATE/DELETE query is authored against this table.
CREATE TABLE event (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id uuid REFERENCES project(id) ON DELETE CASCADE,
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    surface    text NOT NULL CHECK (surface IN ('studio','course','chat')),
    type       text NOT NULL,
    payload    jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX event_project_created_idx ON event (project_id, created_at);

-- Evolve the existing task-scoped tables with a nullable project_id so both worlds
-- can coexist during the migration; task_id/project_id are not mutually exclusive
-- yet, and nothing existing is dropped.
ALTER TABLE material ADD COLUMN project_id uuid REFERENCES project(id) ON DELETE CASCADE;

ALTER TABLE card_instances ADD COLUMN project_id uuid REFERENCES project(id) ON DELETE CASCADE;
ALTER TABLE card_instances ADD COLUMN contract_ref text;
ALTER TABLE card_instances ADD COLUMN framework_fill jsonb NOT NULL DEFAULT '{}';

ALTER TABLE evaluations ADD COLUMN project_id uuid REFERENCES project(id) ON DELETE CASCADE;
ALTER TABLE evaluations ADD COLUMN rubric text;
ALTER TABLE evaluations ADD COLUMN leaps jsonb NOT NULL DEFAULT '[]';

-- +goose Down
ALTER TABLE evaluations DROP COLUMN leaps;
ALTER TABLE evaluations DROP COLUMN rubric;
ALTER TABLE evaluations DROP COLUMN project_id;

ALTER TABLE card_instances DROP COLUMN framework_fill;
ALTER TABLE card_instances DROP COLUMN contract_ref;
ALTER TABLE card_instances DROP COLUMN project_id;

ALTER TABLE material DROP COLUMN project_id;

DROP TABLE IF EXISTS event;
DROP TABLE IF EXISTS chat_message;
DROP TABLE IF EXISTS chat_thread;
DROP TABLE IF EXISTS card_competence;
DROP TABLE IF EXISTS disposition;
DROP TABLE IF EXISTS intervention;
DROP TABLE IF EXISTS source_log_entry;
DROP TABLE IF EXISTS edit_buffer;
DROP TABLE IF EXISTS draft_snapshot;
DROP TABLE IF EXISTS graph_edge;
DROP TABLE IF EXISTS graph_node;
DROP TABLE IF EXISTS project;
