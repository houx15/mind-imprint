-- +goose Up
-- 轻量版的模型调用记在 llm_call：project_id 为 NULL（与 course/chat 调用一致），
-- surface='lite'，并用 atom_id 指回具体的阅读/写作。llm_usage 视图按 user_id /
-- tier / tokens / cost 聚合、不读 project_id，所以学校维度的成本汇总无需改动。
ALTER TABLE llm_call ADD COLUMN atom_id uuid REFERENCES atom(id) ON DELETE SET NULL;
CREATE INDEX llm_call_atom_created_idx ON llm_call (atom_id, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS llm_call_atom_created_idx;
ALTER TABLE llm_call DROP COLUMN atom_id;
