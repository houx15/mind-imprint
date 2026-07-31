-- +goose Up
-- #23 · 片段: the writing room's snippet board. A flat, ordered list of freeform
-- text fragments the student collects while drafting (quotes, a note pulled from
-- a source, a paragraph-in-progress) — distinct from the hierarchical outline and
-- the single 正文 buffer. Whole-set replace like outline_node.
CREATE TABLE snippet (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id uuid NOT NULL REFERENCES project(id) ON DELETE CASCADE,
    text       text NOT NULL DEFAULT '',
    position   int NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX snippet_project_idx ON snippet(project_id);

-- +goose Down
DROP TABLE snippet;
