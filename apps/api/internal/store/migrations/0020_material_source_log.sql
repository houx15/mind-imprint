-- +goose Up
-- Slice 6b. Three things the live 素材 layer needs:
--
-- 1. material.task_id NOT NULL was Slice-3 debt: a legacy FK to the task
--    surface that 5d deleted. A project-scoped ingestion handler has no task
--    to point at, and every project created after project-creation lands will
--    have none either. Drop the constraint; keep the column (old rows still
--    reference their task).
ALTER TABLE material ALTER COLUMN task_id DROP NOT NULL;

-- 2. source_log_entry gains its material link. The table has never been
--    written to (no queries existed until this slice), so there is no data to
--    backfill.
ALTER TABLE source_log_entry
  ADD COLUMN material_id uuid REFERENCES material(id) ON DELETE CASCADE;
CREATE INDEX source_log_entry_material_idx ON source_log_entry (material_id);

-- 3. The demo project's two materials have carried blocks='[]' since 0018 —
--    the real article text lived only in a frontend fixture, which means the
--    CRAAP anchor generator has been running against empty text. Move the
--    content server-side. Real sources (AGENTS.md: no lorem ipsum): a
--    self-media blog riffing on Chen et al. 2019, Nature Sustainability
--    (DOI 10.1038/s41893-019-0220-7), and the finding itself.
UPDATE material SET
  source_url = 'https://mp.weixin.qq.com/s/demo-china-greening',
  blocks = '[
    {"id":"b1","text":"过去二十年里发生了一件几乎没人注意到的事：根据 NASA 卫星数据，地球比 2000 年整整绿了一圈，而这背后最大的推手，是中国。"},
    {"id":"b2","text":"变化大到能从太空里看见。2000 到 2017 年间，NASA 的 MODIS 卫星记录到全球绿叶面积增加了 5%，相当于新增了一整片亚马逊雨林那么大的绿色；仅占全球陆地面积 9% 的中国和印度，就贡献了这其中三分之一以上的增量。"},
    {"id":"b3","text":"很难不把这读成一个信号：那个曾经和雾霾、燃煤电厂划等号的国家，如今悄悄成了地球变绿背后最大的力量——中国的环保政策，正在起效。"}
  ]'::jsonb
WHERE id = '00000000-0000-0000-0000-000000000110';

UPDATE material SET
  title = 'Chen et al. (2019), Nature Sustainability',
  source_url = 'https://doi.org/10.1038/s41893-019-0220-7',
  blocks = '[
    {"id":"b1","text":"基于 NASA MODIS 卫星 2000–2017 年数据：全球绿叶面积净增 5%，中国、印度合计贡献全球净增量的三分之一以上；增量主要来自农业集约化耕作与大规模植树工程，而非森林自然恢复。"}
  ]'::jsonb
WHERE id = '00000000-0000-0000-0000-000000000111';

INSERT INTO source_log_entry (id, project_id, material_id, url, title, takeaway, tier, time_spent_s) VALUES
  ('00000000-0000-0000-0000-000000000170', '00000000-0000-0000-0000-000000000101', '00000000-0000-0000-0000-000000000110',
   'https://mp.weixin.qq.com/s/demo-china-greening', '《卫星图看中国变绿》',
   '把 NASA 的卫星图转述成「中国让地球更可持续」，结论被放大了，需要横向核实。', '二手 · 需追源', 240),
  ('00000000-0000-0000-0000-000000000171', '00000000-0000-0000-0000-000000000101', '00000000-0000-0000-0000-000000000111',
   'https://doi.org/10.1038/s41893-019-0220-7', 'Chen et al. (2019), Nature Sustainability',
   '卫星确认地球在变绿、中国贡献最大，但机制是农业集约化与人工造林——论文本身没说这等于「更可持续」。', '一手论文', 610)
ON CONFLICT (id) DO NOTHING;

-- +goose Down
DELETE FROM source_log_entry WHERE id IN (
  '00000000-0000-0000-0000-000000000170', '00000000-0000-0000-0000-000000000171');
DROP INDEX IF EXISTS source_log_entry_material_idx;
ALTER TABLE source_log_entry DROP COLUMN material_id;
UPDATE material SET blocks = '[]'::jsonb, source_url = NULL
  WHERE id IN ('00000000-0000-0000-0000-000000000110', '00000000-0000-0000-0000-000000000111');
ALTER TABLE material ALTER COLUMN task_id SET NOT NULL;
