-- +goose Up
ALTER TABLE pbl_code_version ADD COLUMN parent_version_id uuid;
ALTER TABLE pbl_code_version ADD COLUMN feedback text NOT NULL DEFAULT '';
ALTER TABLE pbl_code_version ADD CONSTRAINT pbl_code_version_project_id_unique UNIQUE(atom_id,id);
ALTER TABLE pbl_code_version ADD CONSTRAINT pbl_code_version_parent_project FOREIGN KEY(atom_id,parent_version_id) REFERENCES pbl_code_version(atom_id,id);
-- +goose Down
ALTER TABLE pbl_code_version DROP CONSTRAINT pbl_code_version_parent_project;
ALTER TABLE pbl_code_version DROP CONSTRAINT pbl_code_version_project_id_unique;
ALTER TABLE pbl_code_version DROP COLUMN feedback;
ALTER TABLE pbl_code_version DROP COLUMN parent_version_id;
