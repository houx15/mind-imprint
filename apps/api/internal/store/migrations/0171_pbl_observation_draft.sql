-- +goose Up
CREATE TABLE pbl_observation_draft (
 tool_id uuid PRIMARY KEY REFERENCES pbl_tool_instance(id) ON DELETE CASCADE,
 document jsonb NOT NULL DEFAULT '[]'::jsonb,
 revision integer NOT NULL DEFAULT 0,
 submitted_revision integer,
 submitted_notes jsonb NOT NULL DEFAULT '[]'::jsonb,
 updated_at timestamptz NOT NULL DEFAULT now()
);
-- +goose Down
DROP TABLE pbl_observation_draft;
