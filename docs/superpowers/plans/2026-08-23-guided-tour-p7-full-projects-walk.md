# Guided Tour P7 — Full projects-walk refinement · Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to
> implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Refine the read-only demo's projects guided tour end-to-end — restructure the reading
flow, deepen the writing walk, add plan/eval intros, seed real data (full paper, 2nd-layer node,
resource-needs), fix two bugs (未归档 location, essay `\n\n`), and add read-only teaching
variants — so every segment is faithful real-scene on the finished read-only demo.

**Architecture:** Demo stays read-only + finished; seed real GET-readable data, add `isDemo`/
read-state-scoped teaching variants + a map read-badge, add tour nav hooks + anchors, and
restructure `tour/segments/projects.ts`. Write-only controls are spotlighted-and-narrated, never
executed (backend 403s all non-GET on the demo).

**Tech Stack:** Go (`net/http`, sqlc v1.27.0, goose) + Postgres; React + Vite + TS; custom tour
engine (`apps/web/src/tour`); Playwright prod smoke.

**Spec:** `docs/superpowers/specs/2026-08-23-guided-tour-p7-full-projects-walk-design.md`

## Global Constraints

- **Demo read-only.** No tour step triggers a write; backend 403s every non-GET on `is_demo`
  (`projects.go:200-216`). New reads are GET. Write controls are shown + narrated, not clicked.
- **铁律①②:** AI never writes body text; the guiding box / review show the SCAFFOLD + review
  role only. No dark patterns.
- **Copyright:** the seeded paper is ORIGINAL prose on the topic — never a copyrighted paper
  verbatim. Keep the key facts (≈5% greening; China+India ≈⅓; 32% agriculture / 42% tree-planting;
  greening ≠ sustainable; China = top annual CO₂ emitter) so the seeded transcript + essay 批注
  stay coherent.
- **Non-demo behavior unchanged:** every `isDemo` gate must collapse to today's behavior when
  `isDemo===false`; the WarrenMap fixes (未归档 slot, read-badge) are general but must not regress
  normal graphs.
- **Anchors:** `[data-tour="<id>"]`; spotlight steps use non-`center` placement; action steps
  delegate via `closest(selector)`.
- **New nav hooks** mirror the existing one-shot pattern (`pendingWritingView`/`pendingReadingView`
  in WorkspaceContainer + StudentApp→ProjectsTab threading): ref-guarded, cleared after consume.
- Migrations: next numbers after `0084`. Run `internal/agent` tests after any card-spec touch
  (this plan touches none). Commit per task; push to main; deploy once at the end (user-run).

---

### Task 1: Migration — demo essay real newlines

**Files:** Create `apps/api/internal/store/migrations/0085_demo_essay_newlines.sql`; reference
`0082_seed_demo_project_finished.sql:251-265`.

**Interfaces:** Produces: demo essay `edit_buffer …02f0` body with real newlines.

- [ ] **Step 1:** Read the seeded essay body concatenation (`0082:251-265`). Root cause: body
  segments are plain `'…'` literals so `\n\n` is stored literally.
- [ ] **Step 2:** Migration `+goose Up`: `UPDATE edit_buffer SET content = <same essay text with
  REAL newlines> WHERE id = '…02f0'` (rebuild the content with `E'…'` escapes or literal line
  breaks; keep the exact same text, only newlines corrected). `+goose Down`: restore the literal
  form. Demo-only (`WHERE id='…02f0'`). Also fix the immutable `draft_snapshot …02f1` body the
  same way if it carries the literal `\n\n`.
- [ ] **Step 3:** Test: `getDraft`/buffer read for the demo essay returns content containing real
  `\n` (no literal backslash-n). Follow existing demo test patterns.
- [ ] **Step 4:** `CGO_ENABLED=0 go test ./internal/api/... -run 'Draft|Demo' -timeout 1800s`. PASS.
- [ ] **Step 5:** Commit.

---

### Task 2: Migration — full-length paper + inline-highlight anchors

**Files:** Create `apps/api/internal/store/migrations/0086_demo_full_paper.sql`; reference
`0082:94-100` (material `…0271`), `packages/contracts/src/studioState.ts:102-117` (`MaterialBlock`,
`Anchor`), `apps/web/src/studio/reading/ReadingRoom.tsx:742-786` + `SourceDossier.tsx` (`anchorToSpan`).

**Interfaces:** Produces: material `…0271` with a full-length `blocks` array + `anchors` that
render `<mark>` highlights in the reading room.

- [ ] **Step 1:** Discover the `Anchor` shape (fields, how `anchorToSpan` maps an anchor to a span
  in a block — by quote text? offset?) so seeded anchors actually highlight.
- [ ] **Step 2:** Author an ORIGINAL ~10-paragraph article (English or the seed's language) on the
  demo topic, preserving the key facts (see Global Constraints). Each paragraph = one `{id,text}`
  block. Migration UPDATEs material `…0271` `blocks` to this array.
- [ ] **Step 3:** Seed 2–3 `anchors` on the material pointing at real sentences in the new blocks
  (the spans a "溯源/CRAAP card lined out" — e.g. the 32%/42% mechanism sentence, the "not
  equivalent to sustainability" caution). Ensure `anchorToSpan` will match them.
- [ ] **Step 4:** Test: `getMaterialSource(demo, …0271)` returns ≥10 blocks and the seeded anchors;
  a rendering/contract check that anchors resolve to spans (unit or existing dossier test).
- [ ] **Step 5:** `CGO_ENABLED=0 go test ./internal/api/... -run 'MaterialSource|Demo' -timeout 1800s`. PASS.
- [ ] **Step 6:** Commit. (Note in report if the canned reading transcript needs a fact tweak —
  handled in Task 9.)

---

### Task 3: Migration — warren 2nd-layer node + 还需要探索 rows

**Files:** Create `apps/api/internal/store/migrations/0087_demo_warren_depth_and_needs.sql`;
reference `0082:169-189` (`exploration_lead`, `question_edge`), the `exploration_lead` schema
(`parent_lead_id`), and the `note_resource_need`/resource-need table.

- [ ] **Step 1:** Find the `exploration_lead.parent_lead_id` column + the resource-need table
  (`getResourceNeeds`/`putResourceNeeds` backing table) schema.
- [ ] **Step 2:** Migration: seed 1 `exploration_lead` with `parent_lead_id` = an existing demo
  root lead (e.g. `…0292`) + 1 sibling, so clicking that root reveals a real 2nd layer (≥2 children).
  Seed 2–3 resource-need rows for the demo so the 还需要探索 box shows populated.
- [ ] **Step 3:** Test: `GET .../exploration` returns the child lead(s) with `parent_lead_id` set;
  resource-needs GET returns the seeded rows.
- [ ] **Step 4:** `CGO_ENABLED=0 go test ./internal/api/... -run 'Explor|Resource|Demo' -timeout 1800s`. PASS.
- [ ] **Step 5:** Commit.

---

### Task 4: WarrenMap — 未归档 slot fix + map-node read badge

**Files:** Modify `apps/web/src/workspace/blocks/exploration/WarrenMap.tsx`; test under
`apps/web/test/workspace/blocks/exploration/`.

- [ ] **Step 1:** 未归档 fix: replace the hardcoded `{x:0,y:320}` (`~:384-393`) with a slot computed
  from the root nodes' bounding box (place below `max(rootY)+gap`, centered), so it never overlaps
  a root regardless of root count.
- [ ] **Step 2:** Read badge: in `WarrenNodeView`, render a small "已读 ✓" affordance on a node
  whose contained reference(s) are `reading_status==="done"` (data-driven from the node/reference
  data already available). Add `data-tour="warren-node-read"` on such a node. General feature —
  guard so normal graphs are unaffected.
- [ ] **Step 3:** Tests: unfiled slot no longer equals a root position for the demo's 4-root layout;
  a node with a done reference renders the badge + anchor, one without doesn't.
- [ ] **Step 4:** Run tests + `npm run typecheck`. PASS/clean.
- [ ] **Step 5:** Commit.

---

### Task 5: ProposalGuide read-only variant + review-trigger read-only copy

**Files:** Modify `apps/web/src/workspace/blocks/ProposalGuide.tsx` (`:433-460`),
`apps/web/src/workspace/blocks/WritingBlock.tsx` (`:497-507` review button); tests under
`apps/web/test/workspace/blocks/`.

- [ ] **Step 1:** ProposalGuide: when `locked`, render a READ-ONLY FILLED guide (the subquestion
  structure + example prompts, disabled) instead of the empty `<div>` — so `writing-aicard` frames
  a real "可填的引导框". 铁律①: scaffold only, no AI-authored body text. Use seeded/example content;
  no writes.
- [ ] **Step 2:** WritingBlock: for `isDemo`, always render a DISABLED "让印记通读并批注" button
  (mirror the 完成写作 disabled pattern at `:405-415`) with `data-tour="writing-review-trigger"`,
  read-only title. Non-demo behavior unchanged (still gated `!locked`).
- [ ] **Step 3:** Tests: locked ProposalGuide renders the filled read-only guide (not empty);
  isDemo WritingBlock renders the disabled review-trigger with anchor.
- [ ] **Step 4:** Run tests + typecheck. PASS/clean.
- [ ] **Step 5:** Commit.

---

### Task 6: ReferencePanel default → 阅读笔记 + tab-select nav hook

**Files:** Modify `apps/web/src/workspace/blocks/ReferencePanel.tsx` (`:109`, tabs `:219-225`),
`apps/web/src/workspace/WorkspaceContainer.tsx`, `apps/web/src/shell/StudentApp.tsx`,
`apps/web/src/shell/ProjectsTab.tsx`, `apps/web/src/tour/types.ts`; tests.

- [ ] **Step 1:** Change the `isDemo` default active tab from `"anno"` to `"notes"` (only when
  `hasNotes`; else fall back to `tabs[0]`). Non-demo unchanged.
- [ ] **Step 2:** Add a tour nav `selectRefPanelTab(tab: "notes"|"anno"|...)` one-shot (mirror
  `setWritingView`) threaded StudentApp→ProjectsTab→WorkspaceContainer→ReferencePanel, so a tour
  step can select the AI批注 tab explicitly. Ref-guarded + cleared.
- [ ] **Step 3:** Tests: isDemo default = notes; selectRefPanelTab("anno") switches the tab once.
- [ ] **Step 4:** Run tests + typecheck. PASS/clean.
- [ ] **Step 5:** Commit.

---

### Task 7: Nav hooks — activity-log view + search-card open (+ anchors)

**Files:** Modify `apps/web/src/workspace/blocks/PlanBlock.tsx` (view state `:625`, `ActivityLogView`
`:891`), `apps/web/src/workspace/blocks/exploration/ExplorationView.tsx` (SearchCardModal trigger
`:748`) + `SearchCardModal.tsx`, `WorkspaceContainer.tsx`, `StudentApp.tsx`, `ProjectsTab.tsx`,
`tour/types.ts`; tests.

- [ ] **Step 1:** Add tour nav `setPlanView("log"|"board"|"gantt")` one-shot → drives PlanBlock's
  local `view`; add `data-tour="manage-activity-log"` on `ActivityLogView` root (`:900`).
- [ ] **Step 2:** Add tour nav `openSearchCard()` one-shot → sets `setShowSearchCard(true)` in
  ExplorationView; add `data-tour="search-card"` on the modal + `data-tour="search-card-trigger"`
  on the 检索卡 button (`:748`).
- [ ] **Step 3:** Tests: setPlanView("log") shows ActivityLogView; openSearchCard opens the modal.
- [ ] **Step 4:** Run tests + typecheck. PASS/clean.
- [ ] **Step 5:** Commit.

---

### Task 8: Mechanical `data-tour` anchors

**Files:** Modify `apps/web/src/workspace/blocks/NeedsResourcesBox.tsx` (root),
`apps/web/src/workspace/blocks/ReadingBlock.tsx` (add-source button `:879`, `Preview` panel `:1029`,
enter-reading button `:1282`), `apps/web/src/shell/report/EvaluationReport/AxisPanel.tsx`
(`AxisDimCard` `:72`). No behavior change.

- [ ] **Step 1:** Add anchors: `needs-resources` (NeedsResourcesBox root), `library-add`,
  `library-preview`, `library-enter-reading`, and `axis-dim` on each `AxisDimCard` (or a
  per-dimension `data-tour="axis-dim-{code}"`). Confirm `writing-docswitch` already exists.
- [ ] **Step 2:** `npm run typecheck` clean.
- [ ] **Step 3:** Commit.

---

### Task 9: Tour segments — reading-flow reorder

**Files:** Modify `apps/web/src/tour/segments/projects.ts`; test `apps/web/test/tour/projects-segments.test.ts`.
Also update the canned reading transcript fixture (`apps/web/src/tour/fixtures/demoReadingTranscript.ts`)
if Task 2's fuller paper needs it.

**Interfaces:** Consumes anchors/nav from T4/T7/T8 + seeds from T2/T3 + existing rr-*/warren-*.

- [ ] **Step 1:** Add the 立题→管理 transition step (①) + the 管理 activity-log step (② via
  `setPlanView("log")` → `manage-activity-log`).
- [ ] **Step 2:** Rebuild the exploration segment (⑤⑥⑦④③): open 检索卡 (`openSearchCard` →
  `search-card`) → narrate 让印记建议检索方向 → 还需要探索 box (`needs-resources`, seeded) → select a
  node → `explore-keyword`/`explore-find` → action click 找相似 → real canned candidates
  (`explore-suggestions`, narrate 采纳) → click a root with children → real 2nd layer → 未归档 zone.
- [ ] **Step 3:** add/dive (⑧⑨): narrate add/paste (`library-add`/paste UI) + `paper-enter-reading`,
  then `openDemoReadingRoom()` into the immersive room.
- [ ] **Step 4:** 精读 (⑩⑪): real full paper (`rr-article`), a card lining out text (`rr-deck` + the
  seeded `<mark>` span), `rr-chat`, `rr-notes`, `rr-finish`.
- [ ] **Step 5:** back-to-map + node-read (⑫): after 精读, ensure immersive closes (P6 close-on-view
  fix) → graph → spotlight `warren-node-read` ("读完后这个节点标成已读").
- [ ] **Step 6:** switch-view + library (⑫⑬⑭⑮): `reading-viewtoggle` → list → `library-add`
  (familiar add, narrate) → `library-preview` metadata edit (narrate) → `library-enter-reading` (narrate).
- [ ] **Step 7:** Extend `projects-segments.test.ts`: new steps exist, spotlights non-center with
  resolvable anchors, action steps have real selectors, new nav hooks referenced.
- [ ] **Step 8:** Run tour tests + typecheck. PASS/clean.
- [ ] **Step 9:** Commit.

---

### Task 10: Tour segments — writing deepening + eval axis dimensions

**Files:** Modify `apps/web/src/tour/segments/projects.ts`; test `projects-segments.test.ts`.

**Interfaces:** Consumes T5 (guide variant + review-trigger), T6 (default notes + selectRefPanelTab),
T8 anchors (docswitch, axis-dim), seeded 批注 (P6 0083).

- [ ] **Step 1:** Rework the writing segment: 提案/正文 docswitch (⑯ `writing-docswitch`) → 大纲
  building-blocks/subquestions (⑰ point at seeded outline branches — add an outline anchor if needed
  via a tiny follow-up, else spotlight `writing-tabs`→大纲 and narrate) → sidebar default 阅读笔记
  (⑱) → 片段 real guiding box (⑲ `writing-aicard`, now filled) → how to trigger 批注 (⑤/㉑
  `writing-review-trigger`) → select 批注 tab (`selectRefPanelTab("anno")`) + real 批注
  (`writing-annotations`, relabeled ⑳) → move to 正文 (`setWritingView essay/draft`) → 正文 left
  sidebar 片段/阅读笔记/AI批注 (㉓ `writing-refpanel`) → 还需要探索 box (㉔ `needs-resources`) →
  完成写作 (existing `writing-finish`).
- [ ] **Step 2:** Keep 回顾 (定稿 `review-finalize`) as-is.
- [ ] **Step 3:** Eval axis dimensions (㉕): after the `#s5`/`#s6` whole-section steps, add
  per-dimension spotlights for 1–2 representative dims per axis using the `axis-dim` anchors + each
  dimension's `means` copy (D = 认知深度, A = 智识自主).
- [ ] **Step 4:** Extend `projects-segments.test.ts` for the writing + eval steps.
- [ ] **Step 5:** Run tour tests + typecheck. PASS/clean.
- [ ] **Step 6:** Commit.

---

### Task 11: Full verification + prod-browser smoke

**Files:** none.

- [ ] **Step 1:** Full web suite (`vitest run`) + `typecheck` green.
- [ ] **Step 2:** `CGO_ENABLED=0 go test ./internal/api/... ./internal/agent/... -timeout 1800s` green.
- [ ] **Step 3:** (After deploy) Playwright smoke the WHOLE projects journey on the demo: reading
  order correct (search → add/dive → 精读 REAL full paper + a `<mark>` highlight → back-to-map
  已读 node → switch-view → library); writing shows filled guide + review-trigger + real 批注 +
  阅读笔记 default; activity log; eval axis dimensions; essay body has real paragraphs (no literal
  `\n\n`); 0 console errors. Record in the ledger.
- [ ] **Step 4:** No commit unless smoke surfaces a fix.

## Self-review

- Spec coverage: P1a→T1, P1b→T2, P1c→T3, P2a/b→T4, P2c/d→T5, P2e/tab→T6, P2f→T7, P2g→T8,
  Part3 reading→T9, Part3 writing+eval→T10, testing→T11. ✅
- Dependency order: seeds (T1-3) + features/anchors/nav (T4-8) before segments (T9-10) before
  verify (T11). T4-8 mostly disjoint files. Batchable small anchor work is in T8.
- Read-only invariant: only GET reads + isDemo/read-state render variants; no write triggered.
