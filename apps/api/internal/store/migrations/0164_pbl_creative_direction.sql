-- +goose Up
CREATE TABLE pbl_creative_direction (
  atom_id uuid PRIMARY KEY REFERENCES atom(id) ON DELETE CASCADE,
  document jsonb NOT NULL DEFAULT '{"stage":"feeling","feeling":"","motifs":[],"suggestions":[]}'::jsonb,
  revision integer NOT NULL DEFAULT 0,
  updated_at timestamptz NOT NULL DEFAULT now()
);
-- +goose Down
DROP TABLE pbl_creative_direction;
