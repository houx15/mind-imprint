-- +goose Up
-- DualAxis replaces the flat 10-dim report shape. The product is not in use;
-- existing evaluation rows are flat-shaped and cannot be upgraded (one-time,
-- no-regenerate), so clear them. New rows are axis-structured.
DELETE FROM evaluations;

-- +goose Down
-- Irreversible data clear; nothing to restore.
SELECT 1;
