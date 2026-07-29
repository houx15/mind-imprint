-- +goose Up
-- S2 · reading sub-agent contract. The reading room already runs isolated on a
-- material's blocks; these columns give it the two missing halves of the §6
-- contract — a durable brief-in (why read THIS source) and a compact
-- takeaways-out object — plus a phase_tag (which project moment the source
-- serves, feeds S3's rabbit-hole). All additive; NULL = legacy/not-yet.
ALTER TABLE reference ADD COLUMN reading_reason        text;
ALTER TABLE reference ADD COLUMN reading_focus         text;
ALTER TABLE reference ADD COLUMN phase_tag             text;
ALTER TABLE reference ADD COLUMN takeaway              jsonb;
ALTER TABLE reference ADD COLUMN takeaway_finalized_at timestamptz;

-- +goose Down
ALTER TABLE reference DROP COLUMN reading_reason;
ALTER TABLE reference DROP COLUMN reading_focus;
ALTER TABLE reference DROP COLUMN phase_tag;
ALTER TABLE reference DROP COLUMN takeaway;
ALTER TABLE reference DROP COLUMN takeaway_finalized_at;
