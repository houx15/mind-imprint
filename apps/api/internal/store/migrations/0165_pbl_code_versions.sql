-- +goose Up
CREATE TABLE pbl_code_version (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 atom_id uuid NOT NULL REFERENCES atom(id) ON DELETE CASCADE,
 brief_revision integer NOT NULL,
 brief jsonb NOT NULL,
 html text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX pbl_code_version_project ON pbl_code_version(atom_id, created_at DESC);
-- +goose Down
DROP TABLE pbl_code_version;
