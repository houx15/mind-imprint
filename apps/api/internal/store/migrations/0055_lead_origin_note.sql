-- +goose Up
-- C1 · a lead can now originate from promoting a reading note into a
-- question (origin="note", carrying sourceReferenceId). Widen the inline
-- CHECK from migration 0040 (('takeaway','manual','guide')) to include it.
ALTER TABLE exploration_lead DROP CONSTRAINT exploration_lead_origin_check;
ALTER TABLE exploration_lead ADD CONSTRAINT exploration_lead_origin_check
    CHECK (origin IN ('takeaway','manual','guide','note'));

-- +goose Down
ALTER TABLE exploration_lead DROP CONSTRAINT exploration_lead_origin_check;
ALTER TABLE exploration_lead ADD CONSTRAINT exploration_lead_origin_check
    CHECK (origin IN ('takeaway','manual','guide'));
