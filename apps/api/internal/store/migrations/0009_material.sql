-- +goose Up
CREATE TABLE material (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id    uuid NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    kind       text NOT NULL CHECK (kind IN ('article','draft')),
    source     text NOT NULL CHECK (source IN ('fetched','pasted')),
    title      text NOT NULL,
    source_url text,
    blocks     jsonb NOT NULL DEFAULT '[]',
    scratch    text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX material_task_created_idx ON material (task_id, created_at);

-- +goose Down
DROP TABLE material;
