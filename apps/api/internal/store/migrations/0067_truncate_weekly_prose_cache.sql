-- +goose Up
-- class_weekly_prose is a regenerable cache (every number is live; only wording
-- is stored). The fact shape changed from axis to activity metrics AND the window
-- moved to the last completed week, so any pre-existing row narrates the old
-- world. Truncate so no stale axis-worded prose is ever served; it regenerates
-- lazily on next open. depth_note / autonomy_note columns are LEFT in place
-- (harmless, written as '') to avoid a query-shape change on the prose table.
TRUNCATE class_weekly_prose;

-- +goose Down
-- No-op: a cache truncation cannot be un-done, and does not need to be.
SELECT 1;
