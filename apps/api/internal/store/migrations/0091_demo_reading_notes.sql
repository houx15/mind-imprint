-- +goose Up
-- Guided-tour P8 Task 3: seed reading notes so the demo's 我的笔记 (reading
-- room) and 阅读笔记 (writing-room ReferencePanel tab) both show real content.
--
-- Two separate data sources feed those two surfaces:
--   1. reference.reading_note — the student's own freeform note on a source,
--      bound to the reading room's 我的笔记 textarea (ReadingRoom.tsx). 0082
--      already seeded a short note on …0260-0264; this refreshes …0260 (the
--      reference behind the reading-room walk material …0271) with a fuller,
--      first-person note that reads like something written mid-read.
--   2. project.studio_state -> 'reference' — the CURATED pointer list the
--      writing-room panel reads (ReferencePanel.tsx:184 resolveReferences).
--      Its contract shape is ReferenceRef[] = {kind,id,label} (NOT full
--      Reference objects — packages/contracts/src/orchestrator.ts:24-29):
--      resolveReferences looks each {kind:"material", id} up in the real
--      `reference` table (fetched via GET /library) and reads its
--      readingNote/notes/takeaway there. 0082 seeded this array empty
--      ("reference":[]), so the 阅读笔记 tab renders nothing even though the
--      underlying reference rows already carry real notes. This UPDATE adds
--      one curated pointer per demo source (…0260-0264) so the tab has
--      something to resolve.
UPDATE reference SET reading_note =
  '读到这里先记一笔：论文用 NASA MODIS 2000–2017 的数据说全球绿叶面积净增约 5%，中国和印度合计贡献了净增量的三分之一以上——但机制主要是农业集约化（约 32%）和人工造林（约 42%），不是森林自然恢复。这是个关键区分：论文只证明了「变绿」，从没有说这等于「更可持续」，更没有提碳排放。我得把这条数据和 Global Carbon Project 的排放数字放在一起看，才能判断「趋势变好」是不是等于「问题已解决」。'
WHERE id = '00000000-0000-0000-0000-000000000260'
  AND project_id = '00000000-0000-0000-0000-000000000200';

UPDATE project SET studio_state = jsonb_set(studio_state, '{reference}',
  '[
    {"kind":"material","id":"00000000-0000-0000-0000-000000000260","label":"Chen et al. (2019) — 变绿数据的机制限定"},
    {"kind":"material","id":"00000000-0000-0000-0000-000000000261","label":"IEA 清洁能源投资 2023"},
    {"kind":"material","id":"00000000-0000-0000-0000-000000000262","label":"Global Carbon Budget 2023 — 排放反例"},
    {"kind":"material","id":"00000000-0000-0000-0000-000000000263","label":"《卫星图看中国变绿》— 溯源入口"},
    {"kind":"material","id":"00000000-0000-0000-0000-000000000264","label":"燃煤电厂扩张监测 — 第二反例"}
  ]'::jsonb)
WHERE id = '00000000-0000-0000-0000-000000000200' AND is_demo = true;

-- +goose Down
UPDATE reference SET reading_note = ''
WHERE id = '00000000-0000-0000-0000-000000000260'
  AND project_id = '00000000-0000-0000-0000-000000000200';

UPDATE project SET studio_state = jsonb_set(studio_state, '{reference}', '[]'::jsonb)
WHERE id = '00000000-0000-0000-0000-000000000200' AND is_demo = true;
