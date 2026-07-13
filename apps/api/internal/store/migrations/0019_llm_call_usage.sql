-- +goose Up
-- Slice 5d review CRITICAL fix: the new project/graph model (migration 0016)
-- has no row every LLM call maps onto — a silence-legal coach turn persists
-- nothing, and anchor generation writes onto a card, not a message — so
-- usage (provider/model/tier/tokens/cost, AGENTS.md's "记录档位 + token +
-- 成本" hard constraint) gets its own typed table instead of piggybacking on
-- messages/evaluations the way the old task-scoped surface did.
--
-- messages/evaluations stay exactly as they are (0001_init.sql): they are
-- frozen (their writers — agent/turn.go, the rubric evaluator — were deleted
-- in Slice 5d), but their historical rows are real spent money and must not
-- be discarded, so llm_usage keeps unioning them alongside the new arm.
CREATE TABLE llm_call (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    project_id        uuid REFERENCES project(id) ON DELETE SET NULL,  -- NULL for course/chat calls
    surface           text NOT NULL,   -- 'studio' | 'course' | 'chat'
    purpose           text NOT NULL,   -- 'coach' | 'anchors' | 'course_render'
    provider          text NOT NULL DEFAULT '',
    model             text NOT NULL DEFAULT '',
    tier              text NOT NULL DEFAULT '',
    prompt_tokens     integer NOT NULL DEFAULT 0,
    completion_tokens integer NOT NULL DEFAULT 0,
    cost_estimate     numeric(12,6) NOT NULL DEFAULT 0,
    created_at        timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX llm_call_user_created_idx    ON llm_call (user_id, created_at DESC);
CREATE INDEX llm_call_project_created_idx ON llm_call (project_id, created_at DESC);

-- Re-point llm_usage at a three-arm union: the new llm_call table plus the
-- two original arms (messages/evaluations), byte-for-byte unchanged, so
-- GetSchoolUsageByTier (org.sql) — which only reads school_id/tier/
-- prompt_tokens/completion_tokens/cost_estimate — keeps working with no
-- query change.
DROP VIEW IF EXISTS llm_usage;
CREATE VIEW llm_usage AS
  SELECT c.id, c.user_id, u.school_id, c.purpose AS kind,
         c.provider, c.model, c.tier,
         c.prompt_tokens, c.completion_tokens, c.cost_estimate, c.created_at
    FROM llm_call c JOIN users u ON u.id = c.user_id
  UNION ALL
  SELECT m.id, t.user_id, u.school_id, 'chat'::text AS kind,
         m.provider, m.model, m.tier,
         m.prompt_tokens, m.completion_tokens, m.cost_estimate, m.created_at
    FROM messages m JOIN tasks t ON t.id = m.task_id JOIN users u ON u.id = t.user_id
   WHERE m.role = 'assistant' AND m.model IS NOT NULL
  UNION ALL
  SELECT e.id, t.user_id, u.school_id, 'eval'::text AS kind,
         NULL, e.model, e.tier,
         e.prompt_tokens, e.completion_tokens, e.cost_estimate, e.created_at
    FROM evaluations e JOIN tasks t ON t.id = e.task_id JOIN users u ON u.id = t.user_id;

-- +goose Down
DROP VIEW IF EXISTS llm_usage;
CREATE VIEW llm_usage AS
  SELECT m.id, t.user_id, u.school_id, 'chat'::text AS kind,
         m.provider, m.model, m.tier,
         m.prompt_tokens, m.completion_tokens, m.cost_estimate, m.created_at
    FROM messages m JOIN tasks t ON t.id = m.task_id JOIN users u ON u.id = t.user_id
   WHERE m.role = 'assistant' AND m.model IS NOT NULL
  UNION ALL
  SELECT e.id, t.user_id, u.school_id, 'eval'::text AS kind,
         NULL, e.model, e.tier,
         e.prompt_tokens, e.completion_tokens, e.cost_estimate, e.created_at
    FROM evaluations e JOIN tasks t ON t.id = e.task_id JOIN users u ON u.id = t.user_id;
DROP TABLE IF EXISTS llm_call;
