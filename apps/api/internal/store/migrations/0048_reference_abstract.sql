-- +goose Up
-- #4 · Persist the Crossref bibliographic metadata a DOI resolves to. Today the
-- abstract + journal are fetched (materialize/doi.go) but discarded on the
-- success path (only surfaced in the 422 failure details). Persist them so the
-- reading room + library can show the abstract as context and the journal in the
-- annotated bib. author/year/url already live on the reference row. Added LAST
-- on the row so every reference SELECT's column order stays a pure append (sqlc
-- Scan order === SELECT order); NOT NULL DEFAULT '' keeps existing rows valid.
ALTER TABLE reference ADD COLUMN abstract text NOT NULL DEFAULT '';
ALTER TABLE reference ADD COLUMN journal text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE reference DROP COLUMN journal;
ALTER TABLE reference DROP COLUMN abstract;
