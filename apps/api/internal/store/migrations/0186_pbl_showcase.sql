-- +goose Up
CREATE TABLE pbl_showcase (
  user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  draft jsonb NOT NULL DEFAULT '{}'::jsonb,
  published_config jsonb,
  revision integer NOT NULL DEFAULT 0 CHECK (revision >= 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  published_at timestamptz
);

-- +goose Down
DROP TABLE pbl_showcase;
