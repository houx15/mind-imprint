-- +goose Up
ALTER TABLE pbl_site ADD CONSTRAINT pbl_site_owner_atom_unique UNIQUE(user_id,atom_id);
ALTER TABLE pbl_site_publication ADD CONSTRAINT pbl_site_publication_owner
 FOREIGN KEY(user_id,atom_id) REFERENCES pbl_site(user_id,atom_id) ON DELETE CASCADE;
-- +goose Down
ALTER TABLE pbl_site_publication DROP CONSTRAINT pbl_site_publication_owner;
ALTER TABLE pbl_site DROP CONSTRAINT pbl_site_owner_atom_unique;
