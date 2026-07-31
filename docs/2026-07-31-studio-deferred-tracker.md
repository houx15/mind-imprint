# Studio deferred-features wave — 2026-07-31

Branch `feat/studio-deferred-2026-07-31`. Ships the 5 items deferred from the main studio batch (see `2026-07-31-studio-batch-tracker.md`).

## Decisions (locked)
- **#23 writing room** → 3 tabs (大纲/片段/正文) + a materials sidebar available in ALL three; sidebar **defaults to the LEFT** and is **draggable to reposition** (left ↔ right). Click a resource → unfold its notes → 插入到大纲/正文.
- **#8 self-note** → a dedicated `reading_note` field on the reference (migration + sqlc regen).
- **#9 sentence terminators** → CJK 。！？…  + ASCII . ! ?, with a decimal/abbreviation guard on ASCII `.`.
- **#13** → coach offers a **confirm chip** to record a dimension (克制, never auto-writes).

## Slices
- [x] **G · #9 sentence-select** — click selects the SENTENCE under the cursor (`sentences.ts` segmenter + `pointToRuneOffset` in selection.ts + Annotate wiring); #20 example-guard tightened to sentence-range overlap (whole-block escape only when a single-block source has no alternative). Tests: annotate+reading 68 green.
- [ ] **H · #8 self-annotate** — reading_note column + PATCH + ReadingRoom textarea.
- [ ] **I · #13 right-bar confirm chips** — coach detects a covered dim → confirm chip writes it on tap.
- [ ] **J · #18 cards in forming/library** — S4 card-offer pattern on forming + find_sources coaches.
- [ ] **K · #23 writing-room redesign** — 3 tabs + draggable materials sidebar.
