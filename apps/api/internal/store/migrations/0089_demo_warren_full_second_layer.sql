-- +goose Up
-- Guided-tour P8 Task 1: 0087_demo_warren_depth_and_needs.sql gave the demo
-- project (…0200) a 2nd warren layer under exactly ONE of its four root leads
-- (…0292 → …0294/…0295). The other three roots — …0290 (卫星变绿=可持续?),
-- …0291 (人工造林 vs 自然恢复?), …0293 (可再生能源能否抵消存量煤电?) — are
-- still leaves. The real-scene tour walk needs every root to reveal a real
-- 2nd layer when clicked, not just the one Task 3 happened to pick. This
-- seeds two children under each of the three remaining roots, following the
-- exact shape 0087 established: `origin='guide'` + `connected_reference_id`
-- pointing at an already-seeded reference for the child that a seeded source
-- answers, `origin='manual'` + no reference for the child that stays a real
-- open question. Content stays on the China-sustainability topic and ties
-- directly to the reference/root it hangs off (0082's reference rows
-- …0260–…0264): no lorem (AGENTS.md quality bar). …0292's own children
-- (…0294/…0295) are untouched. Idempotent via ON CONFLICT (mirrors 0087).

INSERT INTO exploration_lead (id, project_id, text, status, origin, source_reference_id, connected_reference_id, position, parent_lead_id) VALUES
  -- ── children of …0290 「卫星显示的「变绿」是否等于环境「更可持续」？」 ──────
  ('00000000-0000-0000-0000-0000000002a0', '00000000-0000-0000-0000-000000000200',
   'MODIS 卫星测的是植被指数（NDVI）增长，这是否等同于生态系统质量真正提升——生物多样性、原始林覆盖有没有一起变好？', 'connected', 'guide',
   NULL, '00000000-0000-0000-0000-000000000260', 0, '00000000-0000-0000-0000-000000000290'),
  ('00000000-0000-0000-0000-0000000002a1', '00000000-0000-0000-0000-000000000200',
   '全球净增的绿叶面积里，中国具体是哪些地区贡献最大——是西北的人工绿化带，还是东部的农业密集区？', 'open', 'manual',
   NULL, NULL, 1, '00000000-0000-0000-0000-000000000290'),

  -- ── children of …0291 「变绿主要来自人工造林与农业，还是森林自然恢复？」 ────
  ('00000000-0000-0000-0000-0000000002a2', '00000000-0000-0000-0000-000000000200',
   '人工造林大量使用单一树种（如杨树、桉树），这种「绿化」是否带来生物多样性下降、地下水消耗过快等生态代价？', 'open', 'manual',
   NULL, NULL, 0, '00000000-0000-0000-0000-000000000291'),
  ('00000000-0000-0000-0000-0000000002a3', '00000000-0000-0000-0000-000000000200',
   'Chen et al. (2019) 里「农业集约化」贡献约 32% 的绿叶面积净增，具体是来自复种指数提高（一年多熟），还是耕地扩张？', 'connected', 'guide',
   NULL, '00000000-0000-0000-0000-000000000260', 1, '00000000-0000-0000-0000-000000000291'),

  -- ── children of …0293 「可再生能源投资全球第一，能否抵消存量煤电的影响？」 ──
  ('00000000-0000-0000-0000-0000000002a4', '00000000-0000-0000-0000-000000000200',
   '2022 年新核准的燃煤电厂大多要运行三四十年，这种「锁定效应」是否意味着存量煤电短期内根本退不出？', 'connected', 'guide',
   NULL, '00000000-0000-0000-0000-000000000264', 0, '00000000-0000-0000-0000-000000000293'),
  ('00000000-0000-0000-0000-0000000002a5', '00000000-0000-0000-0000-000000000200',
   '可再生能源装机增长很快，但电网调峰能力跟不上，弃风弃光率有没有跟着上升？「投资全球第一」是否不等于「用得上」？', 'open', 'manual',
   NULL, NULL, 1, '00000000-0000-0000-0000-000000000293')
ON CONFLICT (id) DO NOTHING;

-- +goose Down
DELETE FROM exploration_lead WHERE id IN (
  '00000000-0000-0000-0000-0000000002a0',
  '00000000-0000-0000-0000-0000000002a1',
  '00000000-0000-0000-0000-0000000002a2',
  '00000000-0000-0000-0000-0000000002a3',
  '00000000-0000-0000-0000-0000000002a4',
  '00000000-0000-0000-0000-0000000002a5'
);
