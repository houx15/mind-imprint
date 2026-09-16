-- +goose Up
CREATE TABLE pbl_site_publication (
 user_id uuid PRIMARY KEY REFERENCES pbl_site(user_id) ON DELETE CASCADE,
 atom_id uuid NOT NULL,
 version_id uuid NOT NULL,
 published_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY(atom_id,version_id) REFERENCES pbl_code_version(atom_id,id)
);
-- +goose Down
DROP TABLE pbl_site_publication;
