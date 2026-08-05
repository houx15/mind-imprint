-- +goose Up
-- B1 · the two-level exploration graph's edge table: a labeled, directed
-- relationship between two top-level question leads (子问题/支持/反驳-张力/
-- 细化/依赖-前提 — closed vocabulary, 铁律②'s no-jargon rule keeps warren/
-- rabbit-hole out of the label set too). status mirrors 铁律① — an
-- AI-proposed edge starts "proposed" and only becomes "confirmed" once the
-- student explicitly adopts it (B2 wires the POST that flips it); a
-- student-drawn edge can be inserted directly as "confirmed". Both endpoints
-- cascade off exploration_lead so pruning/deleting a lead takes its edges
-- with it. UNIQUE(from_lead_id, to_lead_id) keeps the graph a simple graph
-- (no duplicate parallel edges between the same ordered pair).
CREATE TABLE question_edge (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id     uuid NOT NULL REFERENCES project(id) ON DELETE CASCADE,
    from_lead_id   uuid NOT NULL REFERENCES exploration_lead(id) ON DELETE CASCADE,
    to_lead_id     uuid NOT NULL REFERENCES exploration_lead(id) ON DELETE CASCADE,
    label          text NOT NULL
                       CHECK (label IN ('子问题','支持','反驳/张力','细化','依赖/前提')),
    status         text NOT NULL DEFAULT 'confirmed'
                       CHECK (status IN ('proposed','confirmed')),
    created_at     timestamptz NOT NULL DEFAULT now(),
    UNIQUE (from_lead_id, to_lead_id)
);
CREATE INDEX question_edge_project_idx ON question_edge (project_id);

-- +goose Down
DROP TABLE question_edge;
