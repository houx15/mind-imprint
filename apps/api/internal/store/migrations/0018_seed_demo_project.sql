-- +goose Up
-- Demo Task+Project for the Studio read path (Slice 5b). Fixed UUIDs, idempotent
-- via ON CONFLICT DO NOTHING. Phoebe = 00000000-0000-0000-0000-000000000003.
-- The task exists only to satisfy the legacy NOT NULL task_id FK (dropped in 5c).
--
-- Authored so studio.Project(sk, ..., studio.Load(...)) reconstructs the 5a
-- STUDIO_FIXTURE shape: S0-S3 done, S4 (build_argument) current with a bare
-- claim + an orphan evidence node, S5-S6 locked, coach anchor on the
-- 治理决心 claim, and a 4-row onboarding rubric.

-- Owned by the seeded admin (00000000-0000-0000-0000-000000000005), NOT Phoebe:
-- this row is only a legacy NOT NULL task_id FK anchor for the demo project's
-- card_instances/material rows (dropped in 5c). It must not belong to Phoebe,
-- or her legacy /api/v1/tasks list (TestTasksCRUD) would see it and fail the
-- "initial list is empty" assertion — the Studio projection below never joins
-- to task.user_id, so the anchor's owner is otherwise irrelevant.
INSERT INTO tasks (id, user_id, title, status) VALUES
  ('00000000-0000-0000-0000-000000000100', '00000000-0000-0000-0000-000000000005', '思维印记 · 0457 演示', 'active')
ON CONFLICT (id) DO NOTHING;

INSERT INTO project (id, user_id, qualification, title, status) VALUES
  ('00000000-0000-0000-0000-000000000101', '00000000-0000-0000-0000-000000000003', '0457 个人报告',
   'To what extent is China making the world more environmentally sustainable?', 'active')
ON CONFLICT (id) DO NOTHING;

-- decode_task nodes → S0 onboarding.
INSERT INTO graph_node (id, project_id, type, author, body) VALUES
  ('00000000-0000-0000-0000-000000000140', '00000000-0000-0000-0000-000000000101', 'rubric_translation', 'ai',
   '{"restate_prompt":"这次要写的是一篇个人报告：「中国在多大程度上让世界变得更具环境可持续性？」（0457 全球社会中的环境系统与社会）。用你自己的话说说，这道题到底在问什么，你打算怎么回答，以及评分标准里你觉得最容易被忽略的是哪一条。","rows":[{"official":"Analysis of different perspectives","plain":"能从不同视角分析，不只罗列观点","weak":true},{"official":"Use & evaluation of evidence / sources","plain":"用可信来源，并说清它可不可信","weak":true},{"official":"Personal response & reflection","plain":"给出自己的判断，并回看研究过程","weak":true},{"official":"Communication & organisation","plain":"结构清楚、表达清晰","weak":false}]}'::jsonb),
  ('00000000-0000-0000-0000-000000000141', '00000000-0000-0000-0000-000000000101', 'milestone_plan', 'student',
   '{"steps":["立题","找素材","评估来源","搭论证","成稿","反思归档"]}'::jsonb)
ON CONFLICT (id) DO NOTHING;

-- build_argument (S4) graph: a done core claim, a bare "治理决心" claim, and an
-- orphan evidence node. With no graph_edge rows connecting them, all four
-- machine gate items are unmet, so ReconcileGates reports build_argument as
-- empty (not partial) — GateDTO carries no status field, so this is invisible
-- on the wire either way.
INSERT INTO graph_node (id, project_id, type, author, body) VALUES
  ('00000000-0000-0000-0000-000000000142', '00000000-0000-0000-0000-000000000101', 'claim', 'student',
   '{"text":"中国的环保治理呈现真实且持续增强的决心，但存量排放问题尚未解决——「趋势变好」不等于「问题已解决」。"}'::jsonb),
  ('00000000-0000-0000-0000-000000000143', '00000000-0000-0000-0000-000000000101', 'claim', 'student',
   '{"text":"中国有治理决心（裸主张，尚无证据）。"}'::jsonb),
  ('00000000-0000-0000-0000-000000000144', '00000000-0000-0000-0000-000000000101', 'evidence', 'student',
   '{"text":"可再生能源投资全球第一。"}'::jsonb)
ON CONFLICT (id) DO NOTHING;

-- gate_state nodes: S0-S3 solid → done; none for S4 (build_argument stays current).
INSERT INTO graph_node (id, project_id, type, author, body) VALUES
  ('00000000-0000-0000-0000-000000000150', '00000000-0000-0000-0000-000000000101', 'gate_state', 'ai', '{"contract":"decode_task","confirmed_solid":true,"items":{}}'::jsonb),
  ('00000000-0000-0000-0000-000000000151', '00000000-0000-0000-0000-000000000101', 'gate_state', 'ai', '{"contract":"frame_question","confirmed_solid":true,"items":{}}'::jsonb),
  ('00000000-0000-0000-0000-000000000152', '00000000-0000-0000-0000-000000000101', 'gate_state', 'ai', '{"contract":"evaluate_perspectives","confirmed_solid":true,"items":{}}'::jsonb),
  ('00000000-0000-0000-0000-000000000153', '00000000-0000-0000-0000-000000000101', 'gate_state', 'ai', '{"contract":"evaluate_sources","confirmed_solid":true,"items":{}}'::jsonb)
ON CONFLICT (id) DO NOTHING;

-- plan: build_argument is the head.
INSERT INTO graph_node (id, project_id, type, author, body) VALUES
  ('00000000-0000-0000-0000-000000000160', '00000000-0000-0000-0000-000000000101', 'plan', 'ai',
   '{"route":["build_argument","draft_polish","reflect_archive"],"reason":"intake"}'::jsonb)
ON CONFLICT (id) DO NOTHING;

-- materials (Slice-6 readiness; not projected in 5b). task_id satisfies the FK.
INSERT INTO material (id, task_id, project_id, kind, source, title, blocks, scratch) VALUES
  ('00000000-0000-0000-0000-000000000110', '00000000-0000-0000-0000-000000000100', '00000000-0000-0000-0000-000000000101', 'article', 'fetched', '《卫星图看中国变绿》', '[]', ''),
  ('00000000-0000-0000-0000-000000000111', '00000000-0000-0000-0000-000000000100', '00000000-0000-0000-0000-000000000101', 'article', 'fetched', 'Nature Sustainability: global greening', '[]', '')
ON CONFLICT (id) DO NOTHING;

-- interventions: the 孤儿证据 flag + the D5 nudge (anchor label read by the coach).
INSERT INTO intervention (id, project_id, type, anchor, criterion, body, level, output_check_verdict) VALUES
  ('00000000-0000-0000-0000-000000000120', '00000000-0000-0000-0000-000000000101', 'flag',
   '{"label":"孤儿证据"}'::jsonb, 'D5', '图上有一处「孤儿证据」：你收集了「可再生投资全球第一」，却没连到任何主张。它到底在替你证明什么？', 'I2', 'pass'),
  ('00000000-0000-0000-0000-000000000121', '00000000-0000-0000-0000-000000000101', 'diagnostic',
   '{"label":"论证图 · 治理决心主张"}'::jsonb, 'D5', '那就把它连到「治理决心」那条主张下——不过那条现在是「裸主张」，还没有证据。这两处正好互相补上。补完，门禁第①条就过了。', 'I2', 'pass')
ON CONFLICT (id) DO NOTHING;

-- card_instances: steelman (提示后, linked by intervention 0121) + concession (自发).
INSERT INTO card_instances (id, task_id, project_id, card_id, status) VALUES
  ('00000000-0000-0000-0000-000000000130', '00000000-0000-0000-0000-000000000100', '00000000-0000-0000-0000-000000000101', 'steelman', 'completed'),
  ('00000000-0000-0000-0000-000000000131', '00000000-0000-0000-0000-000000000100', '00000000-0000-0000-0000-000000000101', 'concession', 'completed')
ON CONFLICT (id) DO NOTHING;

-- link steelman to intervention 0121 → spont = 提示后.
UPDATE intervention SET card_instance_id = '00000000-0000-0000-0000-000000000130'
  WHERE id = '00000000-0000-0000-0000-000000000121';

-- +goose Down
DELETE FROM intervention   WHERE project_id = '00000000-0000-0000-0000-000000000101';
DELETE FROM card_instances WHERE project_id = '00000000-0000-0000-0000-000000000101';
DELETE FROM material       WHERE project_id = '00000000-0000-0000-0000-000000000101';
DELETE FROM graph_node     WHERE project_id = '00000000-0000-0000-0000-000000000101';
DELETE FROM project        WHERE id = '00000000-0000-0000-0000-000000000101';
DELETE FROM tasks          WHERE id = '00000000-0000-0000-0000-000000000100';
