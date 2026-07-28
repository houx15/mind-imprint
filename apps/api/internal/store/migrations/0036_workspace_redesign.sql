-- +goose Up
-- Writing-studio redesign (slice 1b): the new four-room workspace data model.
-- Additive only — replaces the old station projection's storage per surface as
-- slices 2–5 wire each room. FKs cascade from project(id); enums via CHECK.

-- Project Management · the four kick-off dimensions (replaces old onboarding/framing/perspectives kickoff).
CREATE TABLE project_proposal (
    project_id uuid PRIMARY KEY REFERENCES project(id) ON DELETE CASCADE,
    objective  text NOT NULL DEFAULT '',
    reason     text NOT NULL DEFAULT '',
    activities text NOT NULL DEFAULT '',
    resources  text NOT NULL DEFAULT '',
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- Project Management · plan board / gantt items.
CREATE TABLE plan_item (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id      uuid NOT NULL REFERENCES project(id) ON DELETE CASCADE,
    title           text NOT NULL DEFAULT '',
    tag             text NOT NULL CHECK (tag IN ('read','write','review')),
    col             text NOT NULL CHECK (col IN ('todo','doing','done')),
    stage           text NOT NULL DEFAULT '',
    ref_material_id uuid REFERENCES material(id) ON DELETE SET NULL,
    start_day       int NOT NULL DEFAULT 0,
    days            int NOT NULL DEFAULT 1,
    position        int NOT NULL DEFAULT 0,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX plan_item_project_idx ON plan_item (project_id);

-- Read library · collections (self-referential tree).
CREATE TABLE collection (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id uuid NOT NULL REFERENCES project(id) ON DELETE CASCADE,
    name       text NOT NULL DEFAULT '',
    parent_id  uuid REFERENCES collection(id) ON DELETE CASCADE,
    position   int NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX collection_project_idx ON collection (project_id);

-- Read library · reference rows. Reading "notes" (quote→finding) are projected
-- from card_instances/reading-outcomes anchored to material_id, not stored here.
CREATE TABLE reference (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id     uuid NOT NULL REFERENCES project(id) ON DELETE CASCADE,
    title          text NOT NULL DEFAULT '',
    classification text NOT NULL DEFAULT '',
    author         text NOT NULL DEFAULT '',
    credentials    text NOT NULL DEFAULT '',
    year           text NOT NULL DEFAULT '',
    url            text NOT NULL DEFAULT '',
    tags           jsonb NOT NULL DEFAULT '[]',
    collection_id  uuid REFERENCES collection(id) ON DELETE SET NULL,
    credibility    text CHECK (credibility IN ('strong','mixed','weak')),
    evaluation     text NOT NULL DEFAULT '',
    decision       text CHECK (decision IN ('use','maybe','drop')),
    pending        boolean NOT NULL DEFAULT false,
    search_hints   jsonb NOT NULL DEFAULT '[]',
    material_id    uuid REFERENCES material(id) ON DELETE SET NULL,
    position       int NOT NULL DEFAULT 0,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX reference_project_idx ON reference (project_id);

-- Write · outline nodes (depth-indexed flat list, projected to a tree).
CREATE TABLE outline_node (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id uuid NOT NULL REFERENCES project(id) ON DELETE CASCADE,
    text       text NOT NULL DEFAULT '',
    depth      int NOT NULL DEFAULT 0,
    position   int NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX outline_node_project_idx ON outline_node (project_id);

-- Project Management · activity log (auto rows appended by platform, manual via POST).
CREATE TABLE activity_log_entry (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id uuid NOT NULL REFERENCES project(id) ON DELETE CASCADE,
    entry_date date NOT NULL DEFAULT current_date,
    text       text NOT NULL DEFAULT '',
    source     text NOT NULL CHECK (source IN ('auto','me')),
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX activity_log_entry_project_idx ON activity_log_entry (project_id);

-- Review · the five dimensioned reflection answers.
CREATE TABLE project_reflection (
    project_id uuid PRIMARY KEY REFERENCES project(id) ON DELETE CASCADE,
    answers    jsonb NOT NULL DEFAULT '[]',
    done       boolean NOT NULL DEFAULT false,
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- Review · generated mirror prose, first-open-wins (parent-report pattern).
CREATE TABLE project_mirror_prose (
    project_id     uuid PRIMARY KEY REFERENCES project(id) ON DELETE CASCADE,
    sections       jsonb NOT NULL DEFAULT '[]',
    carry_forwards jsonb NOT NULL DEFAULT '[]',
    model          text NOT NULL DEFAULT '',
    tier           text NOT NULL DEFAULT '',
    created_at     timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS project_mirror_prose;
DROP TABLE IF EXISTS project_reflection;
DROP TABLE IF EXISTS activity_log_entry;
DROP TABLE IF EXISTS outline_node;
DROP TABLE IF EXISTS reference;
DROP TABLE IF EXISTS collection;
DROP TABLE IF EXISTS plan_item;
DROP TABLE IF EXISTS project_proposal;
