-- +goose Up
-- S3 · rabbit-hole exploration surface. Leads materialize from reading
-- takeaways (postFinalizeReading) or get added manually/by the exploration
-- guide; the student connects a lead to whichever source answers it, or
-- prunes it. This is the data foundation the exploration tree/branch view and
-- the exploration_guide handler build on.
CREATE TABLE exploration_lead (
    id                      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id              uuid NOT NULL REFERENCES project(id) ON DELETE CASCADE,
    text                    text NOT NULL,
    status                  text NOT NULL DEFAULT 'open'
                                CHECK (status IN ('open','connected','pruned')),
    origin                  text NOT NULL DEFAULT 'takeaway'
                                CHECK (origin IN ('takeaway','manual','guide')),
    source_reference_id     uuid REFERENCES reference(id) ON DELETE SET NULL, -- who surfaced it
    connected_reference_id  uuid REFERENCES reference(id) ON DELETE SET NULL, -- who answers it
    position                int NOT NULL DEFAULT 0,
    created_at              timestamptz NOT NULL DEFAULT now(),
    updated_at              timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX exploration_lead_project_idx ON exploration_lead (project_id);

-- +goose Down
DROP TABLE exploration_lead;
