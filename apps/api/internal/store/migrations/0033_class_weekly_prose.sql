-- +goose Up
-- D2: the ONLY stored part of the weekly report. Numbers are computed live on
-- every read; only the LLM-written words are persisted, once per (class, week).
-- The primary key IS the first-open-wins lock (DEC-2): generation inserts with
-- ON CONFLICT DO NOTHING, so a concurrent second teacher's call is discarded
-- rather than overwriting.
CREATE TABLE class_weekly_prose (
    class_id      uuid NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
    week_start    date NOT NULL,
    comment       text NOT NULL,
    depth_note    text NOT NULL,
    autonomy_note text NOT NULL,
    cards         jsonb NOT NULL DEFAULT '[]',
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (class_id, week_start)
);

-- +goose Down
DROP TABLE IF EXISTS class_weekly_prose;
