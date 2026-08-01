# Studio batch-3 — 2026-08-01

Branch `feat/studio-batch3-2026-08-01` off main `bbcc646`. Nine studio-feedback items, **all frontend** — reuse existing endpoints (putReadingBrief, coach, finishProject, getOutline/getSnippets, exports), **no contracts/Go/migration changes**.

## Decisions (locked, via AskUserQuestion + direct)
- **#6 体检视角** → inline descriptions (each lens shows a plain-language one-liner in the dropdown).
- **#5 finish flow** → two guarded moments (写完了 confirm → 回顾; 完成回顾 confirm → archive). Archived project = draft read-only.
- **#9 outline/snippets/draft flow** → "make the call, optimize for how it feels" → **Option 2 (middle)**: switchable materials box (材料/大纲/片段) + 把大纲导入正文 + click-to-insert; draft stays free-form, snippets stay flat, no schema change, no section editor (honors 铁律 ②).

## Items
- [x] **#1 back button** — PlanBlock: `hasBoard` flag → FormingPhase gets `onBackToBoard` (only when a board exists) → "回到计划板" returns to WorkingPhase.
- [x] **#2 reading stage tag** — ReadingBlock: inline `PhaseTagPicker` (<select>) per row; `setPhaseTag` optimistic + persists via `putReadingBrief` (full-replace, carries existing reason/focus). Threaded RefTable→Row.
- [x] **#3 writing card modal** — WritingBlock CoachRail: opened StudioCardSheet moved from inline-in-rail to a centered `fixed inset-0` modal (backdrop/onSkip closes).
- [x] **#4 探索图谱 default** — ReadingBlock: defaults to graph when the project has sources, else list; **per-project memo** (module-level Map) so a manual toggle survives room re-entry (review M1).
- [x] **#5 guarded finish** — WritingBlock: "写完了" (renamed from 写完了·去完成) → confirm modal → 回顾; archived (status evaluating/done) → draft readOnly + 体检/upload hidden + coach rail sealed (review L2). ReviewBlock: "完成回顾" confirm modal before finish().
- [x] **#6 体检视角 descriptions** — WritingBlock: `VOICE_META`; options show `label · desc`; aria-label preserved.
- [x] **#7 问印记 on selection** — WritingBlock DraftPane: removed the top "就这一段问印记" button; floating "问印记" chip on mouse-selection (coords from onMouseUp rel. to paneRef), flips below near the top (review L3). Calls existing `onFocusPart`. (`paragraphAtCaret` kept — reused by #9's caret logic; keyboard-only selection is a known Low, mouse-driven is the intended #7 design.)
- [x] **#8 English exports** — export/index.ts: all labels/section titles/filenames → English; relevance cell uses ASCII quotes/"; " (review cosmetic).
- [x] **#9 outline/snippets/draft flow** — MaterialsSidebar: 材料/大纲/片段 switcher; context-aware place() (draft-at-caret on 正文 vs new snippet); 把大纲导入正文 (headings). WritingBlock: DraftPane registers `insertAtCaret` (live text/caret via refs, no-op when locked) via `draftInsertRef`.

## Commits
- `86d2c7f` — items #1–#8.
- `633297e` — batch-3 whole-branch review fixes (M1 view-default sticky, L2 sealed coach rail, L3 chip flip, cosmetic ASCII bib).
- `4384e1a` — #9 sidebar connect + draft caret insert.
- `<pending>` — #9 review fixes (M1 archived browse-only, L2 depth clamp, L3 never-focused → append at end) + this tracker.

## Reviews
- Batch-3 whole-branch review (86d2c7f): no Critical/High; M1 + L2/L3 + cosmetic folded into `633297e`. L1 (keyboard selection / dead paragraphAtCaret) intentionally left.
- #9 focused review (4384e1a): no Critical/High. Fixed — **M1** archived project: sidebar was flashing false "已插入" on a locked draft → now browse-only (place/import hidden, 只读浏览 note; also stops post-archive snippet adds from the sidebar); **L2** outlineToHeadings `#` count + margin clamped ≥ 0; **L3** never-focused textarea inserted at position 0 → now appends at end until focused. L4 (outline-refetch race) / L5 (render-phase ref assign) accepted as-is.

## Tests
web 636 green · typecheck clean. New/updated: ReadingBlock (#2/#4), WritingBlock (#3/#5/#7/#9), MaterialsSidebar (#9 + M1 locked).
