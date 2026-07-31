# Studio deferred-features wave — 2026-07-31

Branch `feat/studio-deferred-2026-07-31`. Ships the 5 items deferred from the main studio batch (see `2026-07-31-studio-batch-tracker.md`).

## Decisions (locked)
- **#23 writing room** → 3 tabs (大纲/片段/正文) + a materials sidebar available in ALL three; sidebar **defaults to the LEFT** and is **draggable to reposition** (left ↔ right). Click a resource → unfold its notes → 插入到大纲/正文.
- **#8 self-note** → a dedicated `reading_note` field on the reference (migration + sqlc regen).
- **#9 sentence terminators** → CJK 。！？…  + ASCII . ! ?, with a decimal/abbreviation guard on ASCII `.`.
- **#13** → coach offers a **confirm chip** to record a dimension (克制, never auto-writes).

## Slices
- [x] **G · #9 sentence-select** — click selects the SENTENCE under the cursor (`sentences.ts` segmenter + `pointToRuneOffset` in selection.ts + Annotate wiring); #20 example-guard tightened to sentence-range overlap (whole-block escape only when a single-block source has no alternative). Tests: annotate+reading 68 green.
- [x] **H · #8 self-annotate** — dedicated `reading_note` column (migration 0043 + sqlc regen, pinned to repo's v1.27.0 so the diff is minimal); patchable via `PATCH /references/{rid}` (`readingNote`); a collapsible "我的笔记" box in the ReadingRoom seeded from the reference + saved on blur. Tests: reference CRUD round-trip + contracts 306 + web reading 34 green.
- [x] **I · #13 right-bar confirm chips** — on 立题, one cheap classifier (`ProposeFormingDim`, gated to uncovered dims + rune floor + shared classify cap, metered) detects when the student articulated a still-empty kick-off dim → the coach turn returns a `dimSuggestion` → PlanBlock shows a 克制 confirm chip ("记进「目标」？" + a faithful one-line summary of HER words) that writes the dim + persists on tap (打开由学生确认; append-not-clobber). AI never auto-writes the proposal. Tests: 5 agent unit tests (returns/none/rejects-covered/no-spend guards) + coach/projection + web blocks green.
- [ ] **J · #18 cards in forming/library** — S4 card-offer pattern on forming + find_sources coaches.
- [x] **K · #23 writing-room redesign** — K1: 3 tabs (大纲/片段/正文); 片段 is a net-new snippet board (table 0044 + whole-set-replace API, lifted into `useSnippets`). K2: a floating, DRAGGABLE materials sidebar (`MaterialsSidebar`) — defaults left, drag the header to move, collapses to a 材料 tab; lists references (getLibrary, zero new backend) and unfolds each source's 我的笔记 / 归纳 / reading-card notes; 收进片段 appends any fragment to the snippet board from any tab. Tests: snippet round-trip + client + MaterialsSidebar (list/unfold/insert/collapse) + WritingBlock; web workspace 78 green.
