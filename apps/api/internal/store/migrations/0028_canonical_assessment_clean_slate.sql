-- +goose Up
-- Spec B reshapes the assessment engine: the flat DualAxis report shape
-- (subtotal-bearing depth axis, CrossAxis, Solo, Timeline, KeyEvidence)
-- cannot be upgraded to the canonical shape (6-dim D6 L1-L4 depth + 6-signal
-- A6 0-5 autonomy + 6-lens prompt lens + interaction evidence + generic
-- official-projection slot). The product is not in use, so clear existing
-- evaluation rows rather than attempt a data migration. New rows are
-- canonical-shaped (agent.Report).
DELETE FROM evaluations;

-- +goose Down
-- Irreversible data clear; nothing to restore.
SELECT 1;
