-- +goose Up
-- S5 · the student's AI-use statement (回顾 · 复盘我与 AI 的互动). Split-hybrid: the
-- AI assembles the objective interaction record (recomputed live from
-- events/llm_calls, never stored) and seeds a draft; the student AUTHORS these
-- two fields (AI 克制 — the reflection must be the student's own). Student-owned,
-- updatable (re-review supersedes) — NOT first-open-wins. Fed to the assessment
-- sub-agent as evidence for the responsible-AI-use lens.
CREATE TABLE project_ai_use (
    project_id   uuid PRIMARY KEY REFERENCES project(id) ON DELETE CASCADE,
    used_for     text NOT NULL DEFAULT '',
    not_used_for text NOT NULL DEFAULT '',
    updated_at   timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE project_ai_use;
