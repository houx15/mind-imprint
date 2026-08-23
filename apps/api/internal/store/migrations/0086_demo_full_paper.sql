-- +goose Up
-- Guided-tour P7 Task 2: replace the 3-sentence stub on the demo project's
-- primary-source material (…0271, Chen et al. 2019 Nature Sustainability —
-- seeded by 0082:94-100) with a full-length, ~10-paragraph ORIGINAL article on
-- the same demo topic, and seed real inline-highlight anchors so the reading
-- room renders <mark> spans the guided tour (Task 9) can point at.
--
-- Facts kept stable (must stay coherent with the essay/reflection/evaluation
-- report already seeded by 0082-0085, none of which quote this material's
-- exact prose — only its title/DOI/derived facts): ~5% global greening
-- 2000-2017; China+India ~1/3 of the net increase despite ~9% of land; ~32%
-- agriculture / ~42% tree-planting inside China; "greening is NOT equivalent
-- to overall sustainability"; China = world's top ANNUAL CO2 emitter.
--
-- COPYRIGHT: this prose is original — it paraphrases the real paper's public
-- findings, it does not reproduce its text.
--
-- Anchor shape (packages/contracts/src/anchor.ts): {id, material_id, block_id,
-- start, end, quote, dimension, author, question, answer} — start/end are
-- RUNE indices into the named block's text (Go utf8.RuneCountInString). The
-- web reading room's anchorToSpan (apps/web/src/studio/material/
-- SourceDossier.tsx) turns an anchor into a <mark> span keyed on block_id +
-- the start/end RANGE — the `quote` field is descriptive only, never matched
-- against. MaterialSource.anchors is NOT a material column: it is derived
-- (studio/projection.go's projectMaterials) from every non-skipped
-- card_instances.anchors entry whose own embedded material_id targets this
-- material — so seeding anchors means seeding a card_instances row, not a
-- material column. This mirrors the CRAAP card_id already referenced by
-- 0082's card_completed event for this project.
--
-- Idempotent via ON CONFLICT (mirrors 0082/0083). Demo-only: every statement
-- is scoped to material …0271 (project …0200).

UPDATE material SET blocks = '[
  {"id":"b1","text":"Over the past two decades, satellites orbiting far above the Earth have quietly recorded a change that surprised many climate scientists: large stretches of the planet’s surface have grown measurably greener."},
  {"id":"b2","text":"Between 2000 and 2017, instruments aboard NASA’s MODIS satellites tracked the density of leaf cover across every continent, and the resulting data showed that global green leaf area increased by about 5 percent over that period — a shift large enough to be visible from space."},
  {"id":"b3","text":"What stood out most was where the greening was concentrated. China and India together account for more than one-third of the net increase in global green leaf area, despite together covering only about 9 percent of the planet’s vegetated land."},
  {"id":"b4","text":"Inside China specifically, researchers trace the added greenery to deliberate human choices rather than untouched wilderness bouncing back: about 32 percent of the greening is linked to increasingly intensive agriculture, and about 42 percent to large-scale, government-backed tree-planting programs."},
  {"id":"b5","text":"That distinction matters. A hillside reforested by a national planting campaign and a hillside where a farm now yields two crops a year instead of one both register as ‘greener’ to a satellite sensor, even though neither one is a forest quietly recovering on its own."},
  {"id":"b6","text":"The scientists behind the analysis were careful about how far their own data could be pushed: greening is not equivalent to overall environmental sustainability, and the paper stops well short of claiming that more leaf area adds up to a healthier planet."},
  {"id":"b7","text":"Leaf area is one narrow signal. It says nothing directly about biodiversity, water tables, soil quality, or the survival of the species that used to live on the land before it was replanted or replowed — all of which can decline even while the greenness reading climbs."},
  {"id":"b8","text":"Meanwhile, a separate and less flattering statistic has held steady for close to two decades: China is the world’s top annual emitter of carbon dioxide, a position it has occupied since the mid-2000s, and its yearly emissions still make up close to a third of the global total."},
  {"id":"b9","text":"The two facts are not actually in tension, even though they can feel that way. A country can plant trees and intensify farmland at record scale while its power plants keep burning record amounts of coal — the greening reflects deliberate land-use policy, not a broader energy transition that has already finished."},
  {"id":"b10","text":"Put together, the honest reading of the data is more careful than either headline alone suggests: the greening is real, measurable, and substantially driven by policy rather than accident — but it sits alongside, not in place of, the emissions record, and neither number should be allowed to erase the other."}
]'::jsonb
WHERE id = '00000000-0000-0000-0000-000000000271';

-- ── A completed CRAAP session on …0271, carrying 3 real inline anchors ───────
-- (status 'completed', not 'skipped' — projectMaterials excludes skipped-card
-- anchors) so getMaterialSource(demo, …0271) returns non-empty anchors that
-- resolve to real <mark> spans: the China+India one-third finding (b3), the
-- 32%/42% mechanism sentence (b4), and the "not equivalent to sustainability"
-- caution (b6). Each span covers its whole (single-sentence-dense) paragraph.
INSERT INTO card_instances (id, project_id, card_id, status, anchors, completed_at)
VALUES (
  '00000000-0000-0000-0000-000000000286', '00000000-0000-0000-0000-000000000200', 'craap', 'completed',
  '[
    {"id":"anchor-271-1","material_id":"00000000-0000-0000-0000-000000000271","block_id":"b3","start":0,"end":243,
     "quote":"China and India together account for more than one-third of the net increase in global green leaf area, despite together covering only about 9 percent of the planet’s vegetated land.",
     "dimension":"关键数据","author":"ai",
     "question":"仅占全球陆地约 9% 的两个国家贡献了三分之一以上的净增量——这个比例说明了什么？","answer":""},
    {"id":"anchor-271-2","material_id":"00000000-0000-0000-0000-000000000271","block_id":"b4","start":0,"end":300,
     "quote":"about 32 percent of the greening is linked to increasingly intensive agriculture, and about 42 percent to large-scale, government-backed tree-planting programs.",
     "dimension":"机制","author":"ai",
     "question":"变绿的主因是农业集约化与人工造林——这和「森林自然恢复」是一回事吗？","answer":""},
    {"id":"anchor-271-3","material_id":"00000000-0000-0000-0000-000000000271","block_id":"b6","start":0,"end":255,
     "quote":"greening is not equivalent to overall environmental sustainability, and the paper stops well short of claiming that more leaf area adds up to a healthier planet.",
     "dimension":"限定条件","author":"ai",
     "question":"论文自己划的边界是什么？「变绿」等于「更可持续」吗？","answer":""}
  ]'::jsonb,
  now() - interval '4 days'
)
ON CONFLICT (id) DO UPDATE SET anchors = EXCLUDED.anchors, status = EXCLUDED.status;

-- +goose Down
DELETE FROM card_instances WHERE id = '00000000-0000-0000-0000-000000000286';

UPDATE material SET blocks = '[
  {"id":"b1","text":"Satellite data from NASA MODIS (2000–2017) show the Earth is greening: global green leaf area increased by 5% over the period."},
  {"id":"b2","text":"China and India together account for more than one-third of the net increase in global green leaf area, despite together covering only about 9% of the global land area."},
  {"id":"b3","text":"In China the greening is driven mainly by intensive agriculture (about 32%) and ambitious tree-planting programmes (about 42%); the paper does not claim this greening is equivalent to overall environmental sustainability."}
]'::jsonb
WHERE id = '00000000-0000-0000-0000-000000000271';
