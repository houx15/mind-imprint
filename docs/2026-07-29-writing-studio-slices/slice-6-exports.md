# Slice 6 · Real export formats — SHARED SPEC

The unifying spine: each room quietly grows the deliverable the school requires. Slices 2–5 shipped structured `.md`/`.csv` placeholders; this slice makes them the REAL formats, aligned to the actual student deliverables in `docs/reference/real student essay writing/` (verified columns below). Frontend-only (`apps/web/`). Client-side generation; **dynamically import** the libs so they're lazy chunks, not main-bundle weight.

Add deps to `apps/web`: `xlsx` (SheetJS, spreadsheets) and `docx` (Word). Use `const XLSX = await import("xlsx")` / `const docx = await import("docx")` inside the export fns.

New module `apps/web/src/workspace/export/` with pure fns + a tiny `saveBlob(name, blob)` helper (createObjectURL + click + revoke).

## 1. Annotated Bibliography → `.xlsx`  (from ReadingBlock, single + batch)
Aligned to `资源评估表.xlsx` (verified header row):
`Resource | Classification | Authors | Author Credentials | Journal name / Website | Relevance of resource (cited parts) | Evaluation of this resource | Should I use this resource?`
- Top rows: `Research Title: <project.title>`, `How many articles/papers: <n>`, blank, then the header row, then one row per reference.
- Cell mapping per `Reference`: title→Resource; classification→Classification; author→Authors; credentials→Author Credentials; url→Journal/Website; **Relevance** = the reference's `notes` joined as `「quote」→ finding；…` (the cited parts from the Reading Room); evaluation→Evaluation; `decision` → Should I use (`use`→YES, `maybe`→MAYBE, `drop`→NO, null→"待定"). Skip `pending` refs.
- Filename `注释书目_AnnotatedBibliography.xlsx`.

## 2. Timescale / Gantt → `.xlsx`  (from PlanBlock working-phase export)
Aligned to `timescale.xlsx` (verified: WBS NUMBER | TASK TITLE | STAGE I … STAGE II day columns | SELF-COMMENT):
- Title rows: `TIMESCALE`, `Project: <title>`, `Name / Date`.
- Header: `WBS NUMBER | TASK TITLE | 类型 | 状态` then day columns `1..TIMELINE_DAYS` grouped visually by stage (put a STAGE label row before each stage group).
- One row per plan item (grouped by stage, ordered by start): WBS number = `stageIndex.itemIndex`; task title; tag label (读/写/省); status (待办/进行中/完成); and mark the day cells `start+1 .. start+days` with an "X"/"█" (or the tag). Keep it a legible WBS+gantt grid.
- Filename `项目计划_Timescale.xlsx`.

## 3. Proposal → `.docx`  (from PlanBlock ProposalWriter export)
Aligned to `P+A.docx` §1–§4 (the proposal half):
- Heading: project title + qualification.
- Four sections: `§1 题目、目标与职责` → proposal.objective; `§2 选题理由` → reason; `§3 活动与时间安排` → activities; `§4 资源` → resources. Each a bold heading + the student's text as paragraphs.
- Filename `开题报告_Proposal.docx`.

## 4. Activity Log → `.docx`  (from PlanBlock log-view export)
Aligned to `record form.docx` (production log):
- Heading: `活动日志 / Production Log — <title>`.
- A table: Date | 记录 | 来源(自动/我记的), one row per `LogEntry`, chronological.
- Filename `活动日志_ProductionLog.docx`.

## Wiring
Replace the existing `.md`/`.csv` Blob downloads in `PlanBlock.tsx` (plan export, log export, proposal-writer export) and `ReadingBlock.tsx` (annotated-bib single + batch export) with calls to the new export fns. Keep the buttons/labels as they are. All export fns take already-loaded data (references/plan items/log entries/proposal + project title/qual) — no new network calls; the blocks already hold this data.

## Acceptance
- `pnpm --filter web build` (the dynamic-import chunks build) + `pnpm --filter web test` green. Add `apps/web/test/workspace/export.test.ts` asserting each fn returns a non-empty Blob of the right mime/extension for representative input (you can assert on the produced Blob size/type; deep-parsing xlsx/docx in the test is optional).
- Manual: exporting each produces a file that opens in Excel/Word with the right columns/sections.
- No main-bundle regression: verify `xlsx`/`docx` land in separate chunks (dynamic import), not the entry chunk.
