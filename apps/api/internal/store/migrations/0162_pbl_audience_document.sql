-- +goose Up
-- Drafts remain separate from confirmed personas and do not enter coach context.
CREATE TABLE pbl_audience_document (
  atom_id uuid PRIMARY KEY REFERENCES atom(id) ON DELETE CASCADE,
  document jsonb NOT NULL DEFAULT '{"boards":[],"activeBoardId":"","step":"roles"}'::jsonb,
  revision integer NOT NULL DEFAULT 0,
  updated_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE pbl_audience_document;
