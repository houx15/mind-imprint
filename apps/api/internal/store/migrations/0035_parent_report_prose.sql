-- +goose Up
-- E1: the ONLY stored part of the parent report. Badges/states are computed
-- live on every read; only the LLM-gentled words are persisted, once per
-- (student, surface, scope). The primary key IS the first-open-wins lock:
-- generation inserts with ON CONFLICT DO NOTHING.
CREATE TABLE parent_report_prose (
    student_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    surface         text NOT NULL,
    scope_id        text NOT NULL,
    prose           jsonb NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (student_user_id, surface, scope_id)
);

-- +goose Down
DROP TABLE IF EXISTS parent_report_prose;
