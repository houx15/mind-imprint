-- +goose Up
-- A confirmed audience board may contain several distinct roles.
DROP INDEX pbl_persona_one_chosen;

-- +goose Down
-- Preserve data on rollback: multiple selections cannot be silently discarded.
CREATE UNIQUE INDEX pbl_persona_one_chosen ON pbl_persona (atom_id) WHERE chosen;
