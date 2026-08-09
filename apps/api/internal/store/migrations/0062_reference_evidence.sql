-- Slice 4a · the warren graph IS the 证据地图. A reference (paper) gathered under
-- a sub-question carries its evidence facets: a triage (must-read/to-decide before
-- reading), the nature of its bearing on the sub-question (支持/反驳), a structured
-- per-paper note (key argument / key finding / where it can appear in the essay),
-- and an archive flag for "interesting but not really related". All NOT NULL
-- DEFAULT '' / false so every existing/new row lands cleanly with no backfill.

-- +goose Up
ALTER TABLE reference
    ADD COLUMN triage             text    NOT NULL DEFAULT '',
    ADD COLUMN evidence_nature    text    NOT NULL DEFAULT '',
    ADD COLUMN evidence_argument  text    NOT NULL DEFAULT '',
    ADD COLUMN evidence_finding   text    NOT NULL DEFAULT '',
    ADD COLUMN evidence_placement text    NOT NULL DEFAULT '',
    ADD COLUMN archived           boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE reference
    DROP COLUMN triage,
    DROP COLUMN evidence_nature,
    DROP COLUMN evidence_argument,
    DROP COLUMN evidence_finding,
    DROP COLUMN evidence_placement,
    DROP COLUMN archived;
