-- +goose Up
-- Guided-tour P7 Task 3: the demo project's (…0200) warren graph had NO
-- 2nd-layer nodes — all 4 exploration_lead rows seeded by
-- 0082_seed_demo_project_finished.sql:169-182 are ROOTS (parent_lead_id
-- NULL; the branch column was added later by 0046_exploration_lead_branch.sql).
-- The guided tour (Task 9) needs to click a root lead on the map and reveal a
-- real 2nd layer, so this seeds two children under …0292 (the open "既然是
-- 最大碳排放国，为什么还能说治理有决心？" question — the most natural node
-- to drill into: it's still 'open', unconnected, and the essay's own
-- concession section answers exactly this tension). Content stays on the
-- China-sustainability topic and is coherent with the essay/reflection/
-- evaluation report already seeded (0082-0086): one child follows the
-- "invest vs. burn coal in parallel" thread, the other the "annual share vs.
-- cumulative/per-capita" framing thread from risk item "data-scope" in the
-- 0082 evaluation report JSON.
--
-- Separately, the "还需要探索的" (resource-needs) box was EMPTY: that data
-- lives in project.studio_state jsonb (agent.StudioState.ResourceNeeds,
-- apps/api/internal/api/resource_needs.go — there is no dedicated
-- resource_need TABLE), so this seeds it via jsonb_set, mirroring
-- 0084_demo_studio_state_retrospective.sql's jsonb_set-one-key discipline
-- (every other studio_state key untouched). Idempotent: exploration_lead
-- INSERTs use ON CONFLICT (mirrors 0082/0083); the jsonb_set is naturally
-- idempotent (same value on re-run).

-- ── 2nd-layer: two children under the open root …0292 ───────────────────────
INSERT INTO exploration_lead (id, project_id, text, status, origin, source_reference_id, connected_reference_id, position, parent_lead_id) VALUES
  ('00000000-0000-0000-0000-000000000294', '00000000-0000-0000-0000-000000000200',
   '清洁能源投资全球第一，能不能抵消同一时期新核准的燃煤电厂？', 'connected', 'guide',
   NULL, '00000000-0000-0000-0000-000000000264', 0, '00000000-0000-0000-0000-000000000292'),
  ('00000000-0000-0000-0000-000000000295', '00000000-0000-0000-0000-000000000200',
   '「2022 年约占全球排放 31%」是年度份额，还是历史累计或人均排放？口径不同，结论会变吗？', 'open', 'manual',
   NULL, NULL, 1, '00000000-0000-0000-0000-000000000292')
ON CONFLICT (id) DO NOTHING;

-- ── 还需要探索: seed the resource-needs list on studio_state ─────────────────
UPDATE project
SET studio_state = jsonb_set(
  studio_state,
  '{resourceNeeds}',
  '[
    {"id":"00000000-0000-0000-0000-000000000296","text":"中国海外投资（一带一路）项目的环境影响数据","done":false},
    {"id":"00000000-0000-0000-0000-000000000297","text":"反方来源：把「碳排放全球第一」与「可持续」对立起来论证的一手文献","done":false},
    {"id":"00000000-0000-0000-0000-000000000298","text":"人均碳排放与历史累计排放的权威数据（对照年度份额口径）","done":false}
  ]'::jsonb
)
WHERE id = '00000000-0000-0000-0000-000000000200' AND is_demo = true;

-- +goose Down
UPDATE project
SET studio_state = jsonb_set(studio_state, '{resourceNeeds}', '[]'::jsonb)
WHERE id = '00000000-0000-0000-0000-000000000200' AND is_demo = true;

DELETE FROM exploration_lead WHERE id IN (
  '00000000-0000-0000-0000-000000000294',
  '00000000-0000-0000-0000-000000000295'
);
