-- +goose Up
-- P2 (agentic writing studio) · the 5th 提案要点 section: 可能的反例/张力. Unlike the
-- four required kick-off dimensions, this one is OPTIONAL and does NOT gate plan
-- generation — it's where the student notes tensions/counter-evidence her argument
-- must answer (验收主动脉: 撞反例 → 让步段). Nullable-safe: NOT NULL DEFAULT ''
-- so every existing project_proposal row stays valid. Reversible below.
ALTER TABLE project_proposal ADD COLUMN counterpoints text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE project_proposal DROP COLUMN counterpoints;
