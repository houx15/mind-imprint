-- +goose Up
-- S4 · compaction size-threshold backstop. When the coach's active (non-folded)
-- chat window still exceeds a rune budget after fold-on-solidify, the oldest
-- overflow turns are composed into this rolling per-project digest, then folded
-- out of the window. One evolving row per project (NOT first-open-wins — the
-- digest grows as more turns fold). Read back into buildSpineProjection so the
-- coach never forgets folded history. turns_folded is a running count.
CREATE TABLE conversation_digest (
    project_id   uuid PRIMARY KEY REFERENCES project(id) ON DELETE CASCADE,
    prose        text NOT NULL,
    turns_folded int  NOT NULL DEFAULT 0,
    model        text,
    tier         text,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE conversation_digest;
