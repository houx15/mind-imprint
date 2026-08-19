-- +goose Up
-- Course catalog metadata (spec 2026-08-19): three nullable columns on the
-- existing course row. `category` is one of the 7 controlled slugs (validated
-- app-side, not a DB CHECK, so the vocabulary lives in one place — the TS
-- contract). `introduction` is the schema-driven course intro document
-- (hook/whatYouDo/takeaways/alignment/keywords), stored verbatim as jsonb and
-- border-validated app-side. `featured_rank` drives home-page curation (lower =
-- earlier; NULL = not featured → random fill). All nullable so existing/seeded
-- rows need no backfill to keep rendering.
ALTER TABLE course
  ADD COLUMN category      text,
  ADD COLUMN introduction  jsonb,
  ADD COLUMN featured_rank int;

-- +goose Down
ALTER TABLE course DROP COLUMN IF EXISTS featured_rank;
ALTER TABLE course DROP COLUMN IF EXISTS introduction;
ALTER TABLE course DROP COLUMN IF EXISTS category;
