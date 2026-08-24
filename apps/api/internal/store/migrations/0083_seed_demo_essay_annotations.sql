-- +goose Up
-- Guided-tour demo project (P6 Task 1): seed real AI 批注 (margin annotations)
-- on the demo project's essay (…0200, doc_kind='essay', 0082) so the guided
-- tour can spotlight the actual "AI批注" panel instead of an empty state.
--
-- 批注 are `intervention` rows (0016), NOT a separate annotations table:
-- type='essay_annotation', card_instance_id=NULL (a whole-draft review, not a
-- card submission — see InsertReviewIntervention's comment in agentstore.go),
-- body=the note text, anchor jsonb carries the structured facets
-- ({docKind,level,nature,quote,locator}), criterion mirrors anchor.level,
-- level mirrors anchor.nature (ReplaceAnnotations' column projection). The
-- read path (GET …/proposal-annotations?doc=essay) reconstructs everything
-- from anchor, so criterion/level below only need to stay consistent with it.
--
-- Six 批注 spread across five of the essay's seven body paragraphs, quoting
-- the essay verbatim (0082's edit_buffer …02f0): one `good` on the anchor
-- evidence (论证一), two `suggest` asking for tighter citation, one `problem`
-- flagging the un-cited China-carbon-emissions counter-example, one `good` on
-- the concession/让步段, one `good` on the closing 反思. Ids continue 0082's
-- fixed …02xx block (…02f0-02f3 used; this migration takes …02f4-02f9).
-- Idempotent via ON CONFLICT (mirrors 0082).

INSERT INTO intervention (id, project_id, card_instance_id, type, anchor, criterion, body, level, created_at) VALUES
  ('00000000-0000-0000-0000-0000000002f4', '00000000-0000-0000-0000-000000000200', NULL, 'essay_annotation',
   '{"docKind":"essay","level":"sentence","nature":"good","quote":"这是一手、经同行评审、可复核的数据，构成本文最坚实的证据锚点。","locator":"第2段"}'::jsonb,
   'sentence', '很扎实：这是追回的一手、同行评审论文，作为证据锚点比自媒体转述可靠得多。', 'good',
   TIMESTAMPTZ '2026-08-21 10:05:00+00'),
  ('00000000-0000-0000-0000-0000000002f5', '00000000-0000-0000-0000-000000000200', NULL, 'essay_annotation',
   '{"docKind":"essay","level":"sentence","nature":"suggest","quote":"但同一篇论文也给了我一个至关重要的限定：中国的变绿主要来自农业集约化（约 32%）与大规模人工造林（约 42%），而非森林生态的自然恢复。","locator":"第3段"}'::jsonb,
   'sentence', '这个限定抓得准，但可以补上论文里农业集约化与人工造林占比的具体页码/图表号，方便答辩时直接定位。', 'suggest',
   TIMESTAMPTZ '2026-08-21 10:08:00+00'),
  ('00000000-0000-0000-0000-0000000002f6', '00000000-0000-0000-0000-000000000200', NULL, 'essay_annotation',
   '{"docKind":"essay","level":"sentence","nature":"suggest","quote":"国际能源署（IEA）《World Energy Investment 2023》显示，中国连续多年是全球最大的清洁能源投资国，2023 年其清洁能源投资约占全球的三成。","locator":"第4段"}'::jsonb,
   'sentence', '投资数据很有力，但目前只写了结论、没写清楚具体统计口径——补全后这条证据才经得起追问。', 'suggest',
   TIMESTAMPTZ '2026-08-21 10:11:00+00'),
  ('00000000-0000-0000-0000-0000000002f7', '00000000-0000-0000-0000-000000000200', NULL, 'essay_annotation',
   '{"docKind":"essay","level":"sentence","nature":"problem","quote":"中国自 2006 年起就是全球最大的年度二氧化碳排放国，2022 年约占全球排放的 31%。","locator":"第5段"}'::jsonb,
   'sentence', '这两个数字（起始年份、31% 占比）缺少明确的引用来源和统计口径说明，答辩时如果被追问「这个数据最初从哪来」会站不住脚，需要标注出处。', 'problem',
   TIMESTAMPTZ '2026-08-21 10:14:00+00'),
  ('00000000-0000-0000-0000-0000000002f8', '00000000-0000-0000-0000-000000000200', NULL, 'essay_annotation',
   '{"docKind":"essay","level":"sentence","nature":"good","quote":"这条让步不是对结论的削弱，而是给它装上必要的限定条件。","locator":"第5段"}'::jsonb,
   'sentence', '让步段处理得很好：没有回避对自己不利的反例，而是把它转成了限定条件而不是削弱结论。', 'good',
   TIMESTAMPTZ '2026-08-21 10:17:00+00'),
  ('00000000-0000-0000-0000-0000000002f9', '00000000-0000-0000-0000-000000000200', NULL, 'essay_annotation',
   '{"docKind":"essay","level":"sentence","nature":"good","quote":"我意识到，「不被叙事俘获」不是一种态度，而是一套可以练习的操作——溯源、交叉验证、主动证伪。","locator":"第7段"}'::jsonb,
   'sentence', '反思落在了方法而不是结论上——把「不被叙事俘获」讲成一套可执行的操作，这正是过程评估最想看到的元认知。', 'good',
   TIMESTAMPTZ '2026-08-21 10:20:00+00')
ON CONFLICT (id) DO NOTHING;

-- +goose Down
DELETE FROM intervention WHERE id IN (
  '00000000-0000-0000-0000-0000000002f4',
  '00000000-0000-0000-0000-0000000002f5',
  '00000000-0000-0000-0000-0000000002f6',
  '00000000-0000-0000-0000-0000000002f7',
  '00000000-0000-0000-0000-0000000002f8',
  '00000000-0000-0000-0000-0000000002f9'
);
