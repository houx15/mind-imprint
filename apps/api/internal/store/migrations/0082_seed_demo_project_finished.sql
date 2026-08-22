-- +goose Up
-- Guided-tour demo project (P2 Task 3): a NEW, dedicated, shared read-only demo
-- writing-project the tour walks through. This migration seeds rooms 1-3
-- (立题 / 管理 / 阅读); Task 4 adds 写作/回顾 and flips status to 'finished',
-- Task 5 adds the evaluation report.
--
-- Owner = Phoebe (00000000-0000-0000-0000-000000000003). is_demo=true makes it
-- world-readable + write-blocked (0081). All demo rows use fixed UUIDs in the
-- …02xx block. Do NOT reuse …0101 — that is the shared write fixture (0018).
-- Materials are PROJECT-scoped (project_id set, task_id NULL — material_scope_ck
-- allows project_id alone since 0023). Content on the China-sustainability
-- research question; substantive, no lorem (AGENTS.md quality bar).
-- Idempotent via ON CONFLICT (mirrors 0018/0020).

-- ── Core: the demo project ──────────────────────────────────────────────────
INSERT INTO project (id, user_id, qualification, title, status, studio_state, is_demo) VALUES
  ('00000000-0000-0000-0000-000000000200', '00000000-0000-0000-0000-000000000003', '0457 个人报告 · IB DP 环境系统与社会',
   'To what extent is China making the world more environmentally sustainable?', 'active',
   '{"stage":"topic_discussion","openTool":"chat","widthTier":"chat","reference":[],"updatedAtTurn":0,"started":true}'::jsonb,
   true)
ON CONFLICT (id) DO NOTHING;

-- ── 立题: the four-dimension proposal ────────────────────────────────────────
INSERT INTO project_proposal (project_id, objective, reason, activities, resources) VALUES
  ('00000000-0000-0000-0000-000000000200',
   '我要回答的问题是：中国在多大程度上让世界变得更具环境可持续性？我不满足于「碳排放全球第一」这种一刀切结论，而是想把「趋势在变好」和「问题已解决」这两件事分开——用可信的一手数据，判断中国的环境行动到底是真实的结构性转变，还是被叙事放大的表面功夫，最后给出一个带限定条件的、能被证据支撑的判断。',
   '我选这个题，是因为我先读到一篇自媒体文章，说「NASA 卫星显示中国让地球变绿了」，那种「一个曾经等于雾霾的国家如今拯救地球」的反转叙事很打动我，但也让我警惕：结论是不是被放大了？我想亲手追到卫星数据的原始论文，看看它到底说了什么、没说什么，用这道题训练自己「不被叙事俘获」的判断力。',
   '第一步用 CRAAP / 横向溯源把那篇自媒体文章追回到 Chen et al. (2019, Nature Sustainability) 原论文与 NASA MODIS 数据；第二步分别就「可再生能源投资」「植树造林与农业集约化」「存量碳排放与煤电」三条线各找一手来源交叉验证；第三步用论证图把「主张—证据—推理」连起来，主动去撞反例（中国是全球最大碳排放国）；第四步用让步段处理反例，写出带限定条件的结论；最后成稿、导出、复盘。',
   '一手论文 Chen et al. (2019) Nature Sustainability（NASA MODIS 2000–2017 绿叶面积数据）；IEA《World Energy Investment》与 BloombergNEF 的中国可再生能源投资数据；Global Carbon Project / Our World in Data 的中国碳排放与煤电时间序列；学校图书馆的 IB DP ESS 指南与评分表 D/E/F/H；思维印记的 CRAAP、让步段、反例检验工具卡。')
ON CONFLICT (project_id) DO NOTHING;

-- ── 管理: plan board (read/write/review across todo/doing/done) ───────────────
INSERT INTO plan_item (id, project_id, title, tag, col, stage, start_day, days, position) VALUES
  ('00000000-0000-0000-0000-000000000210', '00000000-0000-0000-0000-000000000200',
   '追源：把自媒体文章追回 NASA / Nature 原始数据', 'read', 'done', '找素材', 0, 2, 0),
  ('00000000-0000-0000-0000-000000000211', '00000000-0000-0000-0000-000000000200',
   '交叉验证三条线：新能源投资 / 造林 / 存量排放', 'read', 'done', '评估来源', 2, 3, 1),
  ('00000000-0000-0000-0000-000000000212', '00000000-0000-0000-0000-000000000200',
   '搭论证图：主张—证据—推理，主动撞反例', 'write', 'doing', '搭论证', 5, 3, 2),
  ('00000000-0000-0000-0000-000000000213', '00000000-0000-0000-0000-000000000200',
   '成稿并复盘：写让步段、导出、过程评估', 'review', 'todo', '成稿反思', 8, 2, 3)
ON CONFLICT (id) DO NOTHING;

-- ── 管理: activity log (mix of auto / me) ────────────────────────────────────
INSERT INTO activity_log_entry (id, project_id, entry_date, text, source) VALUES
  ('00000000-0000-0000-0000-000000000220', '00000000-0000-0000-0000-000000000200', current_date - 6,
   '完成立题：确定研究问题与四维框架，AI 生成了分步计划。', 'auto'),
  ('00000000-0000-0000-0000-000000000221', '00000000-0000-0000-0000-000000000200', current_date - 5,
   '用 CRAAP 把《卫星图看中国变绿》追回 Chen et al. (2019) 原论文，发现自媒体把结论放大了。', 'me'),
  ('00000000-0000-0000-0000-000000000222', '00000000-0000-0000-0000-000000000200', current_date - 3,
   '收藏并读完 4 篇来源，其中 2 篇为一手数据；开始把证据连进论证图。', 'me')
ON CONFLICT (id) DO NOTHING;

-- ── 管理/过程: events feeding the report timeline ────────────────────────────
--   framework_finished = 立题 done marker. surface='studio', payload '{}'.
INSERT INTO event (id, project_id, user_id, surface, type, payload, created_at) VALUES
  ('00000000-0000-0000-0000-000000000230', '00000000-0000-0000-0000-000000000200', '00000000-0000-0000-0000-000000000003',
   'studio', 'milestone:framework_finished', '{}'::jsonb, now() - interval '6 days'),
  ('00000000-0000-0000-0000-000000000231', '00000000-0000-0000-0000-000000000200', '00000000-0000-0000-0000-000000000003',
   'studio', 'source_added', '{"title":"《卫星图看中国变绿》","tier":"二手 · 需追源"}'::jsonb, now() - interval '5 days'),
  ('00000000-0000-0000-0000-000000000232', '00000000-0000-0000-0000-000000000200', '00000000-0000-0000-0000-000000000003',
   'studio', 'source_added', '{"title":"Chen et al. (2019), Nature Sustainability","tier":"一手论文"}'::jsonb, now() - interval '5 days'),
  ('00000000-0000-0000-0000-000000000233', '00000000-0000-0000-0000-000000000200', '00000000-0000-0000-0000-000000000003',
   'studio', 'reading_focus', '{"question":"卫星显示的变绿是否等于更可持续？"}'::jsonb, now() - interval '4 days'),
  ('00000000-0000-0000-0000-000000000234', '00000000-0000-0000-0000-000000000200', '00000000-0000-0000-0000-000000000003',
   'studio', 'card_completed', '{"card_id":"craap"}'::jsonb, now() - interval '4 days')
ON CONFLICT (id) DO NOTHING;

-- ── 论证图: claim + evidence graph nodes ─────────────────────────────────────
INSERT INTO graph_node (id, project_id, type, author, body) VALUES
  ('00000000-0000-0000-0000-000000000240', '00000000-0000-0000-0000-000000000200', 'claim', 'student',
   '{"text":"中国的环境治理呈现真实且持续增强的决心，全球绿叶面积净增中中国贡献最大——但存量碳排放仍居世界第一，因此「趋势变好」不等于「问题已解决」。"}'::jsonb),
  ('00000000-0000-0000-0000-000000000241', '00000000-0000-0000-0000-000000000200', 'evidence', 'student',
   '{"text":"Chen et al. (2019, Nature Sustainability) 基于 NASA MODIS 2000–2017 数据：全球绿叶面积净增 5%，中国、印度合计贡献全球净增量三分之一以上。"}'::jsonb),
  ('00000000-0000-0000-0000-000000000242', '00000000-0000-0000-0000-000000000200', 'evidence', 'ai',
   '{"text":"Global Carbon Project：中国 2006 年起为全球最大年度 CO₂ 排放国，2022 年约占全球排放的 31%——这是需要用让步段处理的反例。"}'::jsonb)
ON CONFLICT (id) DO NOTHING;

-- ── 阅读: collections ────────────────────────────────────────────────────────
INSERT INTO collection (id, project_id, name, position) VALUES
  ('00000000-0000-0000-0000-000000000250', '00000000-0000-0000-0000-000000000200', '一手数据来源', 0),
  ('00000000-0000-0000-0000-000000000251', '00000000-0000-0000-0000-000000000200', '反例与背景', 1)
ON CONFLICT (id) DO NOTHING;

-- ── 阅读: materials (project-scoped, non-empty blocks — real article text) ────
INSERT INTO material (id, project_id, kind, source, title, source_url, blocks, scratch) VALUES
  ('00000000-0000-0000-0000-000000000270', '00000000-0000-0000-0000-000000000200', 'article', 'pasted',
   '《卫星图看中国变绿》（自媒体转述）', 'https://mp.weixin.qq.com/s/demo-china-greening-tour',
   '[
     {"id":"b1","text":"过去二十年里发生了一件几乎没人注意到的事：根据 NASA 卫星数据，地球比 2000 年整整绿了一圈，而这背后最大的推手，是中国。"},
     {"id":"b2","text":"变化大到能从太空里看见。2000 到 2017 年间，NASA 的 MODIS 卫星记录到全球绿叶面积增加了 5%，相当于新增了一整片亚马逊雨林那么大的绿色；仅占全球陆地面积 9% 的中国和印度，就贡献了这其中三分之一以上的增量。"},
     {"id":"b3","text":"很难不把这读成一个信号：那个曾经和雾霾、燃煤电厂划等号的国家，如今悄悄成了地球变绿背后最大的力量——中国的环保政策，正在起效。"}
   ]'::jsonb, '自媒体把「变绿」直接等同于「更可持续」，结论被放大了，需要追回原始论文横向核实。'),
  ('00000000-0000-0000-0000-000000000271', '00000000-0000-0000-0000-000000000200', 'article', 'fetched',
   'Chen et al. (2019), Nature Sustainability — China and India lead in greening', 'https://doi.org/10.1038/s41893-019-0220-7',
   '[
     {"id":"b1","text":"Satellite data from NASA MODIS (2000–2017) show the Earth is greening: global green leaf area increased by 5% over the period."},
     {"id":"b2","text":"China and India together account for more than one-third of the net increase in global green leaf area, despite together covering only about 9% of the global land area."},
     {"id":"b3","text":"In China the greening is driven mainly by intensive agriculture (about 32%) and ambitious tree-planting programmes (about 42%); the paper does not claim this greening is equivalent to overall environmental sustainability."}
   ]'::jsonb, '一手论文本身没有说「变绿=更可持续」，机制是农业集约化与人工造林。')
ON CONFLICT (id) DO NOTHING;

-- ── 阅读: references (≥4, real-looking, use / done, notes on 2+) ──────────────
INSERT INTO reference (id, project_id, title, author, credentials, year, url, journal, abstract,
                       collection_id, credibility, evaluation, decision, reading_status, reading_note, takeaway, material_id, position) VALUES
  ('00000000-0000-0000-0000-000000000260', '00000000-0000-0000-0000-000000000200',
   'China and India lead in greening of the world through land-use management',
   'Chen, C., Park, T., Wang, X., 等', 'Boston University / NASA Ames，Nature 系刊同行评审', '2019',
   'https://doi.org/10.1038/s41893-019-0220-7', 'Nature Sustainability',
   'NASA MODIS 2000–2017 卫星数据：全球绿叶面积净增 5%，中国、印度贡献全球净增量三分之一以上，机制为农业集约化与植树造林。',
   '00000000-0000-0000-0000-000000000250', 'strong',
   '一手、同行评审、数据可复核，是全篇论证的锚点来源。', 'use', 'done',
   '关键：论文只说「变绿」，没说「更可持续」。变绿的主因是农业集约化(≈32%)与人工造林(≈42%)，不是森林自然恢复——这正好是我的核心限定条件。',
   '["greening≠sustainability","中国+印度贡献>1/3","造林与农业集约化为主因"]'::jsonb,
   '00000000-0000-0000-0000-000000000271', 0),
  ('00000000-0000-0000-0000-000000000261', '00000000-0000-0000-0000-000000000200',
   'World Energy Investment 2023 — China clean energy spending',
   'International Energy Agency (IEA)', '国际能源署，政府间权威机构', '2023',
   'https://www.iea.org/reports/world-energy-investment-2023', 'IEA Flagship Report',
   '中国连续多年为全球最大清洁能源投资国，2023 年清洁能源投资约占全球三成。',
   '00000000-0000-0000-0000-000000000250', 'strong',
   '权威机构、方法透明，可支撑「可再生投资全球第一」这条证据。', 'use', 'done',
   '给「治理决心」主张提供第二条独立证据线（不只靠卫星数据）。但投资额大不等于总量已达标，仍要配存量排放看。',
   '["中国清洁能源投资全球第一","约占全球投资三成"]'::jsonb,
   NULL, 1),
  ('00000000-0000-0000-0000-000000000262', '00000000-0000-0000-0000-000000000200',
   'Global Carbon Budget 2023 — national CO₂ emissions',
   'Friedlingstein, P., 等 / Global Carbon Project', 'Global Carbon Project 国际研究联盟', '2023',
   'https://globalcarbonproject.org/carbonbudget/', 'Earth System Science Data',
   '中国自 2006 年起为全球最大年度 CO₂ 排放国，2022 年约占全球排放 31%。',
   '00000000-0000-0000-0000-000000000251', 'strong',
   '一手核算数据，是必须正面处理的反例来源。', 'use', 'done',
   '这是我的反例：要写让步段承认「中国是最大排放国」，再论证「趋势与结构性投入」如何限定这个反例，而不是回避它。',
   '["中国2006起最大排放国","2022约占全球31%","反例必须让步处理"]'::jsonb,
   NULL, 2),
  ('00000000-0000-0000-0000-000000000263', '00000000-0000-0000-0000-000000000200',
   '《卫星图看中国变绿》',
   '某微信公众号', '自媒体，无署名专业资质', '2021',
   'https://mp.weixin.qq.com/s/demo-china-greening-tour', '微信公众号',
   '把 NASA 卫星图转述为「中国让地球更可持续」，结论被放大，作为二手入口而非证据。',
   '00000000-0000-0000-0000-000000000251', 'weak',
   '入口线索但不可作证据：把「变绿」偷换成「更可持续」，需横向溯源。', 'use', 'done',
   '保留它是为了展示溯源过程本身——从这篇追到 Chen et al. 原论文，正是我的方法论证据。',
   '["二手转述","结论被放大","溯源入口"]'::jsonb,
   '00000000-0000-0000-0000-000000000270', 3),
  ('00000000-0000-0000-0000-000000000264', '00000000-0000-0000-0000-000000000200',
   'China''s coal power expansion in 2022',
   'Global Energy Monitor & CREA', '独立能源监测机构', '2023',
   'https://globalenergymonitor.org/report/china-coal-2022/', 'Global Energy Monitor Report',
   '2022 年中国新核准大量燃煤电厂，与其可再生扩张并行——制约「已解决」结论。',
   '00000000-0000-0000-0000-000000000251', 'mixed',
   'NGO 报告，立场需留意，但煤电核准数据可交叉核实。', 'use', 'done',
   '第二条反例线：可再生与煤电同时扩张，说明是「结构转型进行中」而非「问题已解决」，进一步限定我的结论。',
   '["煤电与新能源并行扩张","转型进行中≠已完成"]'::jsonb,
   NULL, 4)
ON CONFLICT (id) DO NOTHING;

-- ── 阅读: source log (material-linked reading sessions) ───────────────────────
INSERT INTO source_log_entry (id, project_id, material_id, url, title, takeaway, tier, time_spent_s) VALUES
  ('00000000-0000-0000-0000-000000000280', '00000000-0000-0000-0000-000000000200', '00000000-0000-0000-0000-000000000270',
   'https://mp.weixin.qq.com/s/demo-china-greening-tour', '《卫星图看中国变绿》',
   '把 NASA 的卫星图转述成「中国让地球更可持续」，结论被放大了，需要横向核实。', '二手 · 需追源', 240),
  ('00000000-0000-0000-0000-000000000281', '00000000-0000-0000-0000-000000000200', '00000000-0000-0000-0000-000000000271',
   'https://doi.org/10.1038/s41893-019-0220-7', 'Chen et al. (2019), Nature Sustainability',
   '卫星确认地球在变绿、中国贡献最大，但机制是农业集约化与人工造林——论文本身没说这等于「更可持续」。', '一手论文', 610)
ON CONFLICT (id) DO NOTHING;

-- ── 阅读: exploration leads (sub-questions) + a question edge ─────────────────
INSERT INTO exploration_lead (id, project_id, text, status, origin, source_reference_id, connected_reference_id, position) VALUES
  ('00000000-0000-0000-0000-000000000290', '00000000-0000-0000-0000-000000000200',
   '卫星显示的「变绿」是否等于环境「更可持续」？', 'connected', 'takeaway',
   '00000000-0000-0000-0000-000000000263', '00000000-0000-0000-0000-000000000260', 0),
  ('00000000-0000-0000-0000-000000000291', '00000000-0000-0000-0000-000000000200',
   '变绿主要来自人工造林与农业，还是森林自然恢复？', 'connected', 'takeaway',
   '00000000-0000-0000-0000-000000000260', '00000000-0000-0000-0000-000000000260', 1),
  ('00000000-0000-0000-0000-000000000292', '00000000-0000-0000-0000-000000000200',
   '既然是最大碳排放国，为什么还能说治理有决心？', 'open', 'manual',
   NULL, NULL, 2),
  ('00000000-0000-0000-0000-000000000293', '00000000-0000-0000-0000-000000000200',
   '可再生能源投资全球第一，能否抵消存量煤电的影响？', 'connected', 'guide',
   NULL, '00000000-0000-0000-0000-000000000264', 3)
ON CONFLICT (id) DO NOTHING;

INSERT INTO question_edge (id, project_id, from_lead_id, to_lead_id, label, status) VALUES
  ('00000000-0000-0000-0000-0000000002a0', '00000000-0000-0000-0000-000000000200',
   '00000000-0000-0000-0000-000000000290', '00000000-0000-0000-0000-000000000291', '子问题', 'confirmed'),
  ('00000000-0000-0000-0000-0000000002a1', '00000000-0000-0000-0000-000000000200',
   '00000000-0000-0000-0000-000000000292', '00000000-0000-0000-0000-000000000293', '反驳/张力', 'confirmed')
ON CONFLICT (id) DO NOTHING;

-- ── 阅读→写作: citations (source → essay section) ─────────────────────────────
INSERT INTO citation (id, project_id, reference_id, section) VALUES
  ('00000000-0000-0000-0000-0000000002b0', '00000000-0000-0000-0000-000000000200',
   '00000000-0000-0000-0000-000000000260', 'claim:00000000-0000-0000-0000-000000000240'),
  ('00000000-0000-0000-0000-0000000002b1', '00000000-0000-0000-0000-000000000200',
   '00000000-0000-0000-0000-000000000262', 'subq:concession')
ON CONFLICT (id) DO NOTHING;

-- ── studio thread + a seeded studio-surface message ──────────────────────────
INSERT INTO chat_thread (id, user_id, title, seeded_project_id) VALUES
  ('00000000-0000-0000-0000-0000000002c0', '00000000-0000-0000-0000-000000000003',
   '中国是否让地球更可持续', '00000000-0000-0000-0000-000000000200')
ON CONFLICT (id) DO NOTHING;

INSERT INTO chat_message (id, thread_id, role, content, surface) VALUES
  ('00000000-0000-0000-0000-0000000002c1', '00000000-0000-0000-0000-0000000002c0', 'user',
   '我看到一篇文章说 NASA 卫星显示中国让地球变绿了，这能说明中国让世界更可持续吗？', 'studio'),
  ('00000000-0000-0000-0000-0000000002c2', '00000000-0000-0000-0000-0000000002c0', 'assistant',
   '这是个值得追一追的说法。在下结论前，先问一个问题：那篇文章的「变绿」数据，最初是从哪来的？我们能不能一起追回它的原始来源，看看原文到底说了什么？', 'studio')
ON CONFLICT (id) DO NOTHING;

-- ════════════════════════════════════════════════════════════════════════════
-- Task 4: 写作 (writing) + 回顾 (review) rooms, then flip the project to FINISHED.
-- All new rows use fixed UUIDs in the …02d0–…02f2 range; …0235 is the finished
-- event. Content stays on the China-sustainability question; full-length essay,
-- real reflection answers — no lorem (AGENTS.md quality bar).
-- ════════════════════════════════════════════════════════════════════════════

-- ── 写作: outline (depth/position tree — thesis → three evidence lines → 让步 → 结论) ─
INSERT INTO outline_node (id, project_id, text, depth, position) VALUES
  ('00000000-0000-0000-0000-0000000002d0', '00000000-0000-0000-0000-000000000200',
   '引论：从「NASA 卫星显示中国让地球变绿」的说法出发，区分「趋势变好」与「问题已解决」', 0, 0),
  ('00000000-0000-0000-0000-0000000002d1', '00000000-0000-0000-0000-000000000200',
   '论证一：卫星数据证明中国是全球变绿的最大贡献者（Chen et al. 2019）', 0, 1),
  ('00000000-0000-0000-0000-0000000002d2', '00000000-0000-0000-0000-000000000200',
   '机制：变绿主因是农业集约化(≈32%)与人工造林(≈42%)，而非森林自然恢复', 1, 2),
  ('00000000-0000-0000-0000-0000000002d3', '00000000-0000-0000-0000-000000000200',
   '论证二：清洁能源投资连续多年全球第一（IEA 2023）', 0, 3),
  ('00000000-0000-0000-0000-0000000002d4', '00000000-0000-0000-0000-000000000200',
   '让步段：中国仍是全球最大碳排放国，且煤电与新能源并行扩张（Global Carbon Project / GEM）', 0, 4),
  ('00000000-0000-0000-0000-0000000002d5', '00000000-0000-0000-0000-000000000200',
   '结论：带限定条件的判断——决心真实、趋势向好，但「更可持续」尚是进行时而非完成时', 0, 5)
ON CONFLICT (id) DO NOTHING;

-- ── 写作: snippets (collected fragments; two carry a section anchor) ──────────
INSERT INTO snippet (id, project_id, text, section, position) VALUES
  ('00000000-0000-0000-0000-0000000002e0', '00000000-0000-0000-0000-000000000200',
   '「全球绿叶面积净增 5%，中国、印度合计贡献超三分之一」——Chen et al. (2019), Nature Sustainability。这是全篇的锚点数据。',
   '论证一', 0),
  ('00000000-0000-0000-0000-0000000002e1', '00000000-0000-0000-0000-000000000200',
   '关键限定：论文本身没有说「变绿=更可持续」，变绿主因是农业集约化与人工造林——不能把机制偷换成结论。',
   '论证一', 1),
  ('00000000-0000-0000-0000-0000000002e2', '00000000-0000-0000-0000-000000000200',
   '反例备忘：中国自 2006 年起为全球最大年度 CO₂ 排放国，2022 年约占全球 31%；煤电与新能源同时扩张——必须用让步段正面处理，不能回避。',
   NULL, 2)
ON CONFLICT (id) DO NOTHING;

-- ── 写作: the essay (正文, doc_kind='essay') — full-length prose incl. ## 反思 ──
--   The report's D6 (metacognition) detection scans the essay for a 反思/Reflection
--   heading, so it is REQUIRED here. The draft snapshot mirrors the same content.
INSERT INTO edit_buffer (id, project_id, doc_kind, content) VALUES
  ('00000000-0000-0000-0000-0000000002f0', '00000000-0000-0000-0000-000000000200', 'essay',
E'# 中国在多大程度上让世界变得更具环境可持续性？\n\n' ||
'一篇在社交媒体上广泛流传的文章宣称：「NASA 卫星显示，中国让地球变绿了。」这句话很有冲击力——一个曾经与雾霾、燃煤电厂划等号的国家，如今似乎成了地球生态的拯救者。但正是这种过于圆满的反转叙事让我警惕。本文的判断是：中国的环境行动确实呈现出真实且持续增强的决心，其治理趋势正在向好；然而「趋势变好」并不等于「问题已解决」。把这两件事分开，是回答这道题的关键。\n\n' ||
'## 论证一：中国是全球变绿的最大贡献者\n\n' ||
'我首先把那篇自媒体文章横向溯源，追回到它真正的一手来源——Chen 等人 2019 年发表于《Nature Sustainability》的论文。该研究基于 NASA MODIS 卫星 2000 至 2017 年的数据，发现全球绿叶面积净增约 5%，而仅占全球陆地面积约 9% 的中国与印度，合计贡献了全球净增量的三分之一以上。这是一手、经同行评审、可复核的数据，构成本文最坚实的证据锚点。\n\n' ||
'但同一篇论文也给了我一个至关重要的限定：中国的变绿主要来自农业集约化（约 32%）与大规模人工造林（约 42%），而非森林生态的自然恢复。更重要的是，论文作者从未声称「变绿」等同于「环境更可持续」。那篇自媒体文章正是在这里把机制偷换成了结论。溯源这一步，让我把「地球变绿」这条证据的边界看清楚了。\n\n' ||
'## 论证二：清洁能源投资的规模\n\n' ||
'为了不让结论只依赖单一证据线，我又找了第二条独立的证据。国际能源署（IEA）《World Energy Investment 2023》显示，中国连续多年是全球最大的清洁能源投资国，2023 年其清洁能源投资约占全球的三成。这为「治理决心真实」提供了独立于卫星数据的支撑：一个国家愿意把如此规模的资本投入可再生能源，很难说这只是叙事包装。\n\n' ||
'## 反思与让步：最大的碳排放国\n\n' ||
'然而，如果我就此收尾，就会犯下我最初警惕的那个错误——被有利的叙事俘获。我主动去撞反例：根据 Global Carbon Project 的核算，中国自 2006 年起就是全球最大的年度二氧化碳排放国，2022 年约占全球排放的 31%。与此同时，Global Energy Monitor 的监测显示，2022 年中国在扩张可再生能源的同时，也新核准了大量燃煤电厂。投资额巨大不等于存量问题已经解决；新能源与煤电并行扩张，恰恰说明这是一场「正在进行的结构转型」，而不是一个「已经完成的胜利」。这条让步不是对结论的削弱，而是给它装上必要的限定条件。\n\n' ||
'## 结论\n\n' ||
'综合三条证据线，我给出一个带限定条件的判断：中国在「让世界更可持续」这件事上，展现了真实、可测量且持续增强的努力——它是全球变绿的最大贡献者，也是最大的清洁能源投资者。但它同时仍是全球最大的碳排放国，其转型仍在进行中。因此更准确的说法是：中国正在使世界变得更可持续，但这是一个进行时，而非完成时。「变绿」是真的，「已经可持续」还不是。\n\n' ||
'## 反思\n\n' ||
'这道题最大的收获不在结论，而在方法。如果我停在那篇自媒体文章，我会得到一个漂亮却经不起追问的答案。真正让判断站得住的，是三个动作：把「变绿」这个说法追回它的一手论文、给主张配上第二条独立证据、以及主动去撞那个对我不利的反例（最大碳排放国）并用让步段处理它。我意识到，「不被叙事俘获」不是一种态度，而是一套可以练习的操作——溯源、交叉验证、主动证伪。下一次遇到同样「太过圆满」的说法时，我会更快地问出那句：这个数据最初是从哪来的？')
ON CONFLICT (project_id, doc_kind) DO NOTHING;

-- ── 写作: an immutable draft snapshot of the essay (seq 1) ────────────────────
INSERT INTO draft_snapshot (id, project_id, doc_kind, seq, content, span_index) VALUES
  ('00000000-0000-0000-0000-0000000002f1', '00000000-0000-0000-0000-000000000200', 'essay', 1,
   (SELECT content FROM edit_buffer
      WHERE project_id = '00000000-0000-0000-0000-000000000200' AND doc_kind = 'essay'),
   '[]'::jsonb)
ON CONFLICT (project_id, doc_kind, seq) DO NOTHING;

-- ── 写作: the 完成写作 milestone for the essay (REQUIRED for a finished project) ─
INSERT INTO writing_finish (id, project_id, doc_kind, finished_at) VALUES
  ('00000000-0000-0000-0000-0000000002f2', '00000000-0000-0000-0000-000000000200', 'essay',
   now() - interval '1 day')
ON CONFLICT (project_id, doc_kind) DO NOTHING;

-- ── 回顾: the five reflection answers (substantive, done=true) ────────────────
INSERT INTO project_reflection (project_id, answers, done) VALUES
  ('00000000-0000-0000-0000-000000000200',
   '[
     "最初我几乎相信了「中国让地球变绿」这个说法，因为它太符合一个动人的反转故事。转折点是我用 CRAAP 把那篇自媒体文章追回 Chen et al. (2019) 的原论文，发现论文根本没说「变绿=更可持续」——那一刻我意识到自己差点被叙事俘获。",
     "我最满意的一步是主动去撞反例：明知中国是全球最大碳排放国，我没有回避，而是把它写进让步段，用它给结论装上限定条件。这让我的判断从「站队」变成了「有边界的论证」。",
     "最难的是区分「趋势变好」和「问题已解决」这两件事。数据既支持前者（变绿、投资第一）又提醒后者尚未成立（排放第一、煤电扩张），我花了很久才想清楚该用「进行时而非完成时」来同时容纳这两组事实。",
     "如果重来，我会更早地为核心主张找第二条独立证据线。一开始我过度依赖卫星数据这一条，直到搭论证图时才补上 IEA 的投资数据——单一证据让我的论证一度很脆弱。",
     "带得走的能力是「溯源—交叉验证—主动证伪」这套动作。以后再遇到「太过圆满」的说法，我会先问它的一手来源在哪，而不是先问它是否符合我的直觉。这比这道题的结论本身更重要。"
   ]'::jsonb, true)
ON CONFLICT (project_id) DO NOTHING;

-- NOTE: the old project_mirror_prose table was RETIRED in migration 0065 (the
-- evaluation report, seeded in Task 5, replaced the mirror-prose narrative). The
-- 回顾 room's persisted state is now project_reflection (above) + the report.

-- ── Flip to FINISHED + the process-tree finished event ───────────────────────
UPDATE project SET status = 'finished' WHERE id = '00000000-0000-0000-0000-000000000200';

INSERT INTO event (id, project_id, user_id, surface, type, payload, created_at) VALUES
  ('00000000-0000-0000-0000-000000000235', '00000000-0000-0000-0000-000000000200', '00000000-0000-0000-0000-000000000003',
   'studio', 'project_finished', '{}'::jsonb, now() - interval '1 day')
ON CONFLICT (id) DO NOTHING;

-- +goose Down
-- Task 4 rows first (FK-safe; the finished event goes with the …0230-block delete below).
UPDATE project SET status = 'active' WHERE id = '00000000-0000-0000-0000-000000000200';
DELETE FROM project_reflection   WHERE project_id = '00000000-0000-0000-0000-000000000200';
DELETE FROM writing_finish       WHERE project_id = '00000000-0000-0000-0000-000000000200';
DELETE FROM draft_snapshot       WHERE project_id = '00000000-0000-0000-0000-000000000200';
DELETE FROM edit_buffer          WHERE project_id = '00000000-0000-0000-0000-000000000200';
DELETE FROM snippet              WHERE project_id = '00000000-0000-0000-0000-000000000200';
DELETE FROM outline_node         WHERE project_id = '00000000-0000-0000-0000-000000000200';
DELETE FROM chat_message      WHERE thread_id = '00000000-0000-0000-0000-0000000002c0';
DELETE FROM chat_thread       WHERE id = '00000000-0000-0000-0000-0000000002c0';
DELETE FROM citation          WHERE project_id = '00000000-0000-0000-0000-000000000200';
DELETE FROM question_edge     WHERE project_id = '00000000-0000-0000-0000-000000000200';
DELETE FROM exploration_lead  WHERE project_id = '00000000-0000-0000-0000-000000000200';
DELETE FROM source_log_entry  WHERE project_id = '00000000-0000-0000-0000-000000000200';
DELETE FROM reference         WHERE project_id = '00000000-0000-0000-0000-000000000200';
DELETE FROM material          WHERE project_id = '00000000-0000-0000-0000-000000000200';
DELETE FROM collection        WHERE project_id = '00000000-0000-0000-0000-000000000200';
DELETE FROM graph_node        WHERE project_id = '00000000-0000-0000-0000-000000000200';
DELETE FROM event             WHERE project_id = '00000000-0000-0000-0000-000000000200';
DELETE FROM activity_log_entry WHERE project_id = '00000000-0000-0000-0000-000000000200';
DELETE FROM plan_item         WHERE project_id = '00000000-0000-0000-0000-000000000200';
DELETE FROM project_proposal  WHERE project_id = '00000000-0000-0000-0000-000000000200';
DELETE FROM project           WHERE id = '00000000-0000-0000-0000-000000000200';
