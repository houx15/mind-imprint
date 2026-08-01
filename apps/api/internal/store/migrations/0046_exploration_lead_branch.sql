-- +goose Up
-- #12 · 分支: a 线索 can hang sub-branches under it. parent_lead_id points at the
-- parent lead (NULL = a top-level thread). ON DELETE CASCADE so pruning/deleting
-- a parent takes its branches with it. The exploration view projects the tree
-- from this column — no separate table.
ALTER TABLE exploration_lead ADD COLUMN parent_lead_id uuid REFERENCES exploration_lead(id) ON DELETE CASCADE;
CREATE INDEX exploration_lead_parent_idx ON exploration_lead (parent_lead_id);

-- +goose Down
DROP INDEX exploration_lead_parent_idx;
ALTER TABLE exploration_lead DROP COLUMN parent_lead_id;
