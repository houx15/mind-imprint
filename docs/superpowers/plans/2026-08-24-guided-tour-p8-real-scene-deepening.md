# Guided Tour P8 — Real-scene deepening · Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the P7 tour's narrated/mocked exploration + writing segments into real, clickable, richly-seeded scenes on the read-only demo, and fix two defects.

**Architecture:** Demo-only goose seed migrations (0089-0091) + one canned-fixture upgrade enrich the demo; small read-only FE unlocks (in-line card wiring, client-side add-node simulation, demo note carry-through, write-mode mock, markdown rendering, reader force-close) make real interactions reachable; the tour segments are restructured to a coherent story line. No backend writes on the demo.

**Tech Stack:** Go (`net/http`, `pgx`, goose), PostgreSQL, React + Vite + TS, Tailwind `mk-*` tokens, custom tour engine (`apps/web/src/tour/`).

**Spec:** `docs/superpowers/specs/2026-08-24-guided-tour-p8-real-scene-deepening-design.md`

## Global Constraints

- **Read-only demo:** never introduce a backend write on the demo project (`is_demo=true`). The only write-shaped gesture (采纳) is a client-side visual simulation. All new endpoints/reads must be GET or canned short-circuits.
- **铁律①:** AI never authors body text. Seeded guide cards carry an AI *question* + an English *example on a different topic* + the student's *own* text only. The mode-choice mock shows 印记 proposing / student confirming (铁律②).
- **Demo ids (pinned):** project `00000000-0000-0000-0000-000000000200`; user Phoebe …0003; material …0271 (Chen paper) + …0270; refs …0260 (Chen, `reading_status='reading'`) …0261 IEA …0262 GCP …0263 公众号 …0264 GEM coal; roots …0290/…0291/…0292/…0293; existing 2nd-layer …0294/…0295 (under …0292); resource-needs …0296/…0297/…0298; card_instance …0286; essay annotations …02f4–02f9; essay edit_buffer …02f0 / draft_snapshot …02f1.
- **Next migration number:** 0089. Mirror `0087`/`0084` jsonb_set discipline (mutate one studio_state key, leave others intact; idempotent `ON CONFLICT DO NOTHING`; provide a down migration).
- **Seeding a card_instance / proposal-track is a proficiency-regression risk (P7 lesson):** after any seed touching cards or proposal-track, run the FULL `internal/api` + `internal/agent` suites (`CGO_ENABLED=0`, api `-timeout 1800s`), not a narrow `-run` filter. Do NOT pipe `go test` through `| tail`/`| grep` (masks the non-zero exit).
- **Contracts are non-strict Zod** but mirror new fields into Go via `go run ./tools/synccards` only if a card spec changes (P8 changes no card specs). `make sqlc` only if a query changes.
- **Deploy is user/controller-run** via `./.deploy-local/deploy.sh full|web`; do not deploy inside a task.

---

### Task 1: Migration 0089 — warren 2nd layer for every root

**Files:**
- Create: `apps/api/internal/store/migrations/0089_demo_warren_full_second_layer.sql`

**Interfaces:**
- Consumes: `exploration_lead` table (columns as used by `0082:169-182` and `0087:27-34` — `id, project_id, parent_lead_id, text, status, connected_reference_id, position` — confirm exact columns by reading `0082`/`0087` before writing).
- Produces: 2 child leads under each of …0290, …0291, …0293 (…0292 already has …0294/…0295).

- [ ] **Step 1: Read the real insert shape.** Read `apps/api/internal/store/migrations/0087_*.sql` (the 2nd-layer insert) and the root inserts in `0082_seed_demo_project_finished.sql:169-188` to copy the exact column list, status enum values (`connected`/`open`), and `question_edge` usage. Note existing child ids …0294/…0295 to avoid collision.
- [ ] **Step 2: Author the migration.** `-- +goose Up`: INSERT 6 `exploration_lead` rows (fixed uuids, e.g. …02a0–02a5), 2 per root, faithful to each root's question:
  - under …0290 (卫星变绿=可持续?): a child on NDVI-vs-真实生态质量, and one on 变绿的空间分布(哪些地区).
  - under …0291 (人工造林 vs 自然恢复?): a child on 造林树种单一化的生态代价, and one on 农业复种指数的贡献占比.
  - under …0293 (可再生能源能否抵消存量煤电?): a child on 新核准煤电装机的锁定效应, and one on 电网调峰与弃风弃光率.
  Use `status='open'` for open sub-questions, `connected` + a `connected_reference_id` only if pointing at a seeded ref. `-- +goose Down`: `DELETE FROM exploration_lead WHERE id IN (…02a0..02a5)`.
- [ ] **Step 3: Apply + verify.** Run the goose migration against the dev DB (follow the repo's migrate command — check `apps/api/Makefile`/`docs`); confirm `SELECT parent_lead_id, count(*) FROM exploration_lead WHERE project_id='…0200' GROUP BY 1` shows children under …0290/…0291/…0292/…0293.
- [ ] **Step 4: Down + re-up.** Verify down removes only the new rows and re-up is clean.
- [ ] **Step 5: Commit** `git add apps/api/internal/store/migrations/0089_*.sql && git commit -m "feat(tour): seed a warren 2nd layer under every demo root (0089)"`

---

### Task 2: Migration 0090 — proposal-track guide cards + prop snippets

**Files:**
- Create: `apps/api/internal/store/migrations/0090_demo_proposal_track_guides.sql`

**Interfaces:**
- Consumes: `project.studio_state` jsonb; `snippet` table (`id, project_id, text, position, section` — confirm via `0044_snippet.sql`/`0045_snippet_section.sql`). Go read path: `agent.WritingTrack` (`apps/api/internal/agent/proposal_track.go:33-39`), keys = the 9 fixed part keys (`understanding, question-scope, thesis, resources, challenges, method, feasibility, expected, polish`).
- Produces: a populated `studio_state.proposalTrack` + `prop:<key>` snippet rows so `ProposalGuideReadOnly` renders filled boxes.

- [ ] **Step 1: Read the persistence contract.** Read `apps/api/internal/agent/proposal_track.go:33-78`, `apps/api/internal/api/proposal_track.go:60,137,167-172`, and `packages/contracts/src/proposalGuide.ts` to confirm: `stepGuides` values are **double-encoded JSON strings** `{"prompt","example","refHint"}`; `mode:"guided",started:true,stepIndex:0`; and the 9 keys/titles. Read `0082_seed_demo_project_finished.sql:16-21,236-246` for the current studio_state keys and snippet insert shape.
- [ ] **Step 2: Author guide-card content** (9 cards). For each fixed key, a Chinese `prompt` = a guiding QUESTION applied to "中国是否让地球变得更可持续？" (e.g. understanding: "这个题目里，『可持续』具体指什么？你打算用哪一两个可核查的指标来判断？"), an **English** `example` = a short paragraph on a *different* topic (never the answer, 铁律①), optional `refHint`. Keep prose real, no lorem.
- [ ] **Step 3: Author the migration.** `-- +goose Up`: `UPDATE project SET studio_state = jsonb_set(studio_state,'{proposalTrack}', '{...}'::jsonb) WHERE id='…0200' AND is_demo=true;` with `mode:"guided",started:true,stepIndex:0,subQuestions:[],stepGuides:{...}` (each value an escaped JSON string). Then INSERT `snippet` rows `section='prop:<key>'` (fixed uuids …02b0+) carrying Phoebe's own text per part, `ON CONFLICT (id) DO NOTHING`. `-- +goose Down`: `UPDATE … SET studio_state = studio_state - 'proposalTrack'` + `DELETE FROM snippet WHERE project_id='…0200' AND section LIKE 'prop:%'`.
- [ ] **Step 3.5: Verify no live generation is triggered.** Because `stepIndex:0` (`understanding`) is seeded non-empty, the gen branch (`proposal_track.go` lazy gen) must take a cache hit. Confirm by reading the gen condition; ensure no seeded step at `stepIndex` is empty.
- [ ] **Step 4: Apply + verify read-back.** Migrate; `GET`-equivalent: run the api unit that exercises `getProposalTrack` for the demo (or a quick `SELECT studio_state->'proposalTrack' FROM project WHERE id='…0200'`), confirm 9 cards decode.
- [ ] **Step 5: FULL backend suites.** `cd apps/api && CGO_ENABLED=0 go test ./internal/api/... ./internal/agent/... -timeout 1800s` — all green (proposal-track/proficiency regression guard). Do not pipe through tail/grep.
- [ ] **Step 6: Commit** `git commit -m "feat(tour): seed real proposal-track guide cards + prop snippets on demo (0090)"`

---

### Task 3: Migration 0091 — reading notes + curated reference list

**Files:**
- Create: `apps/api/internal/store/migrations/0091_demo_reading_notes.sql`

**Interfaces:**
- Consumes: `reference.reading_note` column; `project.studio_state->'reference'` (curated list read by `ReferencePanel` via `resolveReferences`). Confirm the curated-entry shape from `packages/contracts/src/reference.ts` + how `studio_state.reference` entries are shaped (read `0082` — it seeds `reference: []`).
- Produces: a real `reading_note` on …0260 + a non-empty `studio_state.reference` so both the reading-room 我的笔记 and the writing-room 阅读笔记 tab show content.

- [ ] **Step 1: Read shapes.** Read `packages/contracts/src/reference.ts:13-79` (`readingNote`, `notes[]`, `takeaway`) and how `ReferencePanel.tsx:184,242` consumes `studio_state.reference`. Determine the minimal curated-entry object shape that makes the 阅读笔记 tab render.
- [ ] **Step 2: Author the migration.** `-- +goose Up`: `UPDATE reference SET reading_note='<Phoebe's real note on the Chen greening paper>' WHERE id='…0260' AND project_id='…0200';` and `UPDATE project SET studio_state = jsonb_set(studio_state,'{reference}', '[...]'::jsonb) WHERE id='…0200' AND is_demo=true;` with curated entries pointing at …0260–0264 (carry per-source `notes`/`takeaway` where the panel needs them). `-- +goose Down`: reset `reading_note=''` on …0260 and `jsonb_set(studio_state,'{reference}','[]'::jsonb)`.
- [ ] **Step 3: Apply + verify** read-back of both fields.
- [ ] **Step 4: Commit** `git commit -m "feat(tour): seed reading notes + curated reference list on demo (0091)"`

---

### Task 4: Upgrade cannedSearchGuidance to real directions

**Files:**
- Modify: `apps/api/internal/api/demo.go:106-110` (`cannedSearchGuidance`)
- Test: the existing demo/exploration test file (find it — grep `cannedSearchGuidance`/`search-guidance` under `apps/api/internal/api/*_test.go`)

**Interfaces:**
- Consumes: the `agent.SearchGuidanceOutput` (or equivalent) shape returned by `search-guidance` — confirm the struct (`{keyword, why}[]`) by reading `search_guidance.go` + `demo.go`.
- Produces: 2-3 real China-sustainability directions instead of 1 placeholder.

- [ ] **Step 1: Read** `demo.go:106-110`, `search_guidance.go:27`, and the output struct/DTO.
- [ ] **Step 2: Write/adjust the failing test** asserting `cannedSearchGuidance()` returns ≥2 directions with non-empty `keyword`+`why` and no "演示" placeholder.
- [ ] **Step 3: Implement** — return 2-3 real directions (e.g. NDVI/植被指数 vs 真实可持续性；中国可再生能源装机 vs 存量煤电；净零承诺时间表与可核查性). Keep a pure fixture (no network).
- [ ] **Step 4: Test** the demo/exploration package green.
- [ ] **Step 5: Commit** `git commit -m "feat(tour): real demo search directions (cannedSearchGuidance)"`

---

### Task 5: Reading-room in-line card click-to-reveal

**Files:**
- Modify: `apps/web/src/studio/reading/ReadingRoom.tsx:787-788` (+ the `data-tour` on the revealed note panel; the panel lives in `apps/web/src/primitives/annotate/Annotate.tsx:241-246`)
- Test: existing ReadingRoom/Annotate test (grep)

**Interfaces:**
- Consumes: `Annotate`'s `activeSpanId` + `onSelectSpan` props; spans from `SourceDossier.tsx:59-71` (`tag=dimension`, `note=answer||question`).
- Produces: clicking a highlight sets active span → the note panel renders → anchor `rr-inline-card` for the tour.

- [ ] **Step 1: Read** `ReadingRoom.tsx` around 780-810 and `Annotate.tsx:201-246` to confirm the props and where the note panel renders.
- [ ] **Step 2: Wire state.** Add `const [activeSpanId,setActiveSpanId]=useState<string|null>(null)`; pass `activeSpanId={activeSpanId}` and `onSelectSpan={setActiveSpanId}` at `:787-788`. General feature (works outside select-mode in real reading too — verify select-mode still owns clicks when active).
- [ ] **Step 3: Anchor.** Add `data-tour="rr-inline-card"` to the active-span note panel container (`Annotate.tsx:241` region) — guard so it only tags when a span is active.
- [ ] **Step 4: Test.** Update/add a test that clicking a `<mark>` reveals the note panel (jsdom can set state; hit-test verified later in prod smoke).
- [ ] **Step 5: Commit** `git commit -m "feat(reading): reveal the lens card behind a highlight on click"`

---

### Task 6: Client-side add-node simulation (markDemoNodeAdopted)

**Files:**
- Modify: `apps/web/src/tour/types.ts` (add `markDemoNodeAdopted` to `TourNavContext`), `apps/web/src/shell/StudentApp.tsx`, `apps/web/src/workspace/ProjectsTab.tsx`, `apps/web/src/workspace/WorkspaceContainer.tsx` (accumulate set + merge), `apps/web/src/workspace/blocks/exploration/ExplorationView.tsx` (intercept `addFromDetail`/`adopt` when isDemo; merge synthetic lead/ref before render)
- Test: exploration/tour tests (grep for `markDemoNodeRead` tests as the template)

**Interfaces:**
- Consumes: the `markDemoNodeRead` pattern (`WorkspaceContainer.tsx:407`, `mergeReadByRoot`, `ExplorationView.tsx:694-697`) as the proven template; `DigCandidate` shape from `cannedDig`; `ExplorationLead`/`Reference` contracts.
- Produces: a nav hook `markDemoNodeAdopted(candidate, parentLeadId)` that makes a synthetic node appear under the drilled root, no backend write.

- [ ] **Step 1: Study the template.** Read the full `markDemoNodeRead` path (types.ts hook → StudentApp → ProjectsTab → WorkspaceContainer set + merge → ExplorationView consumption → WarrenMap). Note the `isDemo` gating added in P7 (`9c728505`).
- [ ] **Step 2: Add the hook + state.** `markDemoNodeAdopted(candidate, parentLeadId)` accumulates into a `demoAdoptedLeads` + `demoAdoptedRefs` set/map in `WorkspaceContainer` (gated `workspace?.isDemo`). Thread the setter through StudentApp→ProjectsTab like `markDemoNodeRead`.
- [ ] **Step 3: Merge before render.** In `ExplorationView` (before `QuestionMindmap`/counts), merge synthetic `ExplorationLead` (id minted, `parent_lead_id=parentLeadId=focusRoot.id`, `status='connected'`, `connected_reference_id=<synthetic ref id>`) + `Reference` (from the `DigCandidate`) into `view.leads`/`references`. Gate on `isDemo`.
- [ ] **Step 4: Intercept the write.** When `isDemo`, `addFromDetail`/`adopt` call `markDemoNodeAdopted(...)` instead of the 403 write; `onLibraryChanged` no-op.
- [ ] **Step 5: Test** that the merge produces a connected child under the root and counts update; no network call fires on demo.
- [ ] **Step 6: Commit** `git commit -m "feat(tour): simulate adopting a source into the demo graph (read-only)"`

---

### Task 7: Demo 我的笔记 carry-through

**Files:**
- Modify: `apps/web/src/shell/StudentApp.tsx:226-229` (`openDemoReadingRoom` → add `readingNote` to `pendingDemoReading`), `apps/web/src/workspace/WorkspaceContainer.tsx:217` (type) + `:1351-1362` (pass as 7th `openReadingSource` arg), `apps/web/src/workspace/ProjectsTab.tsx:72` (type)

**Interfaces:**
- Consumes: `pendingDemoReading` object; `openReadingSource(source, referenceId, …, readingNote, …)` 7th positional arg; the seeded `reference.reading_note` (Task 3) or a fixed fixture string.
- Produces: the reading-room 我的笔记 shows real content on the demo.

- [ ] **Step 1: Read** the `pendingDemoReading` type + the `openReadingSource` signature + `ReadingRoom.tsx:238` (`readingNote` prop → textarea).
- [ ] **Step 2: Carry the note.** Add `readingNote` to `pendingDemoReading` (from the fixture, matching Task 3's seeded note) and pass it as the 7th `openReadingSource` arg (currently `undefined`).
- [ ] **Step 3: Verify** the textarea seeds to the note (still `disabled={demoMode}` = read-only display).
- [ ] **Step 4: Commit** `git commit -m "feat(tour): carry the seeded reading note into the demo reading room"`

---

### Task 8: Write-mode demoModal mock

**Files:**
- Modify: `apps/web/src/tour/TourRunner.tsx` (register a new `demoModal.kind`), create a small mock component (e.g. `apps/web/src/tour/mocks/WriteModeChoiceMock.tsx` — follow the P5 question-card `demoModal` mock as the pattern; grep `demoModal` to find it)
- Test: tour renderer test

**Interfaces:**
- Consumes: the `demoModal:{kind,title}` mechanism (`tour/types.ts`, `TourRunner.tsx:131-138`).
- Produces: a static mock of "印记问你：这一部分，要自己写，还是我一步步带你写？ [我自己写] [一步步带我写]".

- [ ] **Step 1: Read** the existing P5 demoModal mock + `TourRunner.tsx:131-138` kind dispatch.
- [ ] **Step 2: Build** the mock component — a faithful static render of the chat-action choice (two buttons, non-functional, styled like `StudioTurnChips`). Route any bold text through markdown (Task 10).
- [ ] **Step 3: Register** the new kind in `TourRunner`.
- [ ] **Step 4: Test** the modal renders the mock.
- [ ] **Step 5: Commit** `git commit -m "feat(tour): write-mode choice mock (自己写 / 带我写)"`

---

### Task 9: Exploration data-tour anchors

**Files:**
- Modify: `apps/web/src/workspace/blocks/exploration/ExplorationView.tsx` (directions button …:788-810; `搜索` button in `directions` stage …:846-900; result rows in `list` stage …:902-947; PaperDetail primary/采纳 action …:949-994; enter-reading control)

**Interfaces:**
- Produces: anchors `explore-directions`, `explore-search`, `explore-result`, `explore-adopt`, `explore-enter-reading` on real controls the tour action-clicks.

- [ ] **Step 1: Read** `ExplorationView.tsx` stages 788-994 to place anchors on the exact clickable elements (tour action steps delegate via document-capture + `closest()`, so anchoring the button/li is enough).
- [ ] **Step 2: Add** the five `data-tour` anchors. Where a stage renders a list, anchor the first actionable row deterministically (or a stable representative).
- [ ] **Step 3: Test** anchors present (segment/render test).
- [ ] **Step 4: Commit** `git commit -m "feat(tour): anchor the exploration search controls"`

---

### Task 10: Markdown rendering in tour title / modal / transcript

**Files:**
- Modify: `apps/web/src/tour/TourRunner.tsx:219` (`step.title`) + `:131-132` (demoModal `Modal` title); `apps/web/src/studio/reading/ReadingRoom.tsx:539,559` (transcript bubbles)

**Interfaces:**
- Consumes: `CardMdInline` (inline) + `Markdown` (block) from `@/cards/Markdown`.
- Produces: `**bold**` etc. render everywhere tour text shows.

- [ ] **Step 1: Read** `apps/web/src/cards/Markdown.tsx` for `Markdown`/`CardMdInline` APIs.
- [ ] **Step 2: Wrap** `step.title` and the demoModal `Modal` title through `CardMdInline`; transcript `m.body` (`:539/559`) through `Markdown`.
- [ ] **Step 3: Audit** all P8 tour-authored strings + demoModal mocks + `demoReadingTranscript.ts` for `**`/markdown and confirm they render.
- [ ] **Step 4: Test** a title with `**bold**` renders `<strong>`.
- [ ] **Step 5: Commit** `git commit -m "fix(tour): render markdown in step titles, modal titles, and transcript"`

---

### Task 11: BUG A — force-close the reader on view change

**Files:**
- Modify: `apps/web/src/workspace/WorkspaceContainer.tsx` (reset `lastReadingView.current` when the immersive reader opens — in the `openDemoReadingRoom`/`pendingDemoReading` handler …:1347-1365, and/or `openReadingSource` :441-region)
- Test: WorkspaceContainer/tour test

**Interfaces:**
- Consumes: the `pendingReadingView` value-dedupe guard (`:1246-1253`), `lastReadingView` ref, `closeReadingSource` (`:441`).
- Produces: after the immersive reader opens, a subsequent `setReadingView("graph")` is not swallowed → `closeReadingSource()` runs → `reading-nodedone-0` shows on the graph.

- [ ] **Step 1: Reproduce/read** the guard at `:1246-1253` and confirm `lastReadingView.current` is `"graph"` when `reading-nodedone-0` fires.
- [ ] **Step 2: Fix root cause.** When the immersive reader opens (`pendingDemoReading` handler / `openReadingSource`), set `lastReadingView.current = null` so the next `setReadingView(...)` always passes the guard.
- [ ] **Step 3: Verify** `reading-library-0` (`setReadingView("list")`) still closes correctly, and normal (non-tour) reading-room close is unaffected.
- [ ] **Step 4: Test** the guard-reset behavior.
- [ ] **Step 5: Commit** `git commit -m "fix(tour): close the reading room when returning to the map (view-dedupe guard)"`

---

### Task 12: Restructure the reading tour segment

**Files:**
- Modify: `apps/web/src/tour/segments/projects.ts` (reading segment: `reading-warren` / `reading-room` / `reading-nodedone` / `reading-library`)
- Test: segment test (unique ids, anchors exist, action selectors, non-center spotlights)

**Interfaces:**
- Consumes: anchors from Tasks 5/9 (`rr-inline-card`, `explore-*`), the hook from Task 6 (`markDemoNodeAdopted`), Task 7's filled note, Task 11's close fix. Existing anchors `warren-question/unfiled/node-read`, `search-card(-trigger)`, `needs-resources`, `rr-article/chat/deck/notes/finish`, `reading-viewtoggle`, `library-*`.
- Produces: the real search scene + reading-room card/notes steps + reworded copy.

- [ ] **Step 1: Read** the current reading segment in `projects.ts` (P7 order) and the step schema in `tour/types.ts`.
- [ ] **Step 2: Rewrite** the reading segment to the spec's order (Part 5): intro/views → main Q + unfiled → 检索卡 mental model → 还需要探索 → **drill into …0292** (action `warren-question`) → **建议检索方向** (action `explore-directions`) → **搜索** (action `explore-search`) → **open a paper** (action `explore-result`) → narrate 找相似 → **采纳** (action `explore-adopt`, onEnter/afteraction fires `markDemoNodeAdopted` with a chosen `cannedDig` candidate under …0292) → "new node appeared" → **enter reading** (action `explore-enter-reading` → `openDemoReadingRoom()`) → reading room: `rr-article` → **click highlight → `rr-inline-card`** → `rr-chat` → `rr-deck` (narrate) → **`rr-notes` (filled, reworded copy)** → `rr-finish` → **back to map** (Task 11) → `warren-node-read` → view toggle → `library-*`.
- [ ] **Step 3: Reword** the 我的笔记 copy — student-relatable, no forward-reference to evaluation. e.g. "「我的笔记」是你自己的地盘——边读边写下想法、疑问、想引用的句子。印记不替你写，也不动它。"
- [ ] **Step 4: Wire the adopt→node.** Ensure the 采纳 action, via `markDemoNodeAdopted`, targets the drilled root (…0292) and a `cannedDig` candidate (GCP/IEA — carbon/energy fit). Confirm the enter-reading step reaches the reading room.
- [ ] **Step 5: Test** the segment (ids unique, every anchor referenced exists in code, action selectors valid, no `placement:"center"` on action steps that need a spotlight).
- [ ] **Step 6: Commit** `git commit -m "feat(tour): real search→adopt→read scene in the reading segment"`

---

### Task 13: Reorder the writing tour segment

**Files:**
- Modify: `apps/web/src/tour/segments/projects.ts` (writing segment)
- Test: segment test

**Interfaces:**
- Consumes: Task 8's write-mode demoModal; the seeded filled 片段 box (Tasks 2); anchors `writing-docswitch/outline/aicard/refpanel/annotations/review-trigger/finish`, `needs-resources`; nav hooks `setWritingView`, `selectRefPanelTab`.
- Produces: the reordered writing story (spec Part 5).

- [ ] **Step 1: Read** the current writing segment (P7 order: 0 边界→1 docswitch→2 outline→3 refpanel→4 aicard→5 annotations→6 trigger→7 refpanel→8 needs→9 finish).
- [ ] **Step 2: Rewrite** to: 边界 (center) → **mode-choice mock** (Task 8) → 提案/正文 (`writing-docswitch`) → 大纲 (`writing-outline`) → **片段 filled box** (`writing-aicard`, `setWritingView({doc:"proposal",tab:"snippets"})`) → **正文 page** (`setWritingView({doc:"essay",tab:"draft"})`, "这里你自己写正文") → **left sidebar** (`writing-refpanel`, default 阅读笔记) → **还需要探索** (`needs-resources`) → **trigger 批注** (`writing-review-trigger`) → **AI 批注 appears** (`writing-annotations`, `selectRefPanelTab("anno")`) → 完成 (`writing-finish`).
- [ ] **Step 3: Verify copy** — 片段 step describes the now-real box (question + example + your own text); trigger step says "写完后，点这里让印记通读并批注"; 批注 step shows the green/blue/red result.
- [ ] **Step 4: Test** the segment (unique ids, anchors exist, `selectRefPanelTab("anno")` fired on a tab where it isn't clobbered by the snippets effect — do the 批注 spotlight after the doc/tab is on draft).
- [ ] **Step 5: Commit** `git commit -m "feat(tour): reorder writing story (mode→片段→正文→sidebar→needs→trigger→批注)"`

---

## Self-Review

- **Spec coverage:** item ① → T1; item ② search → T4/T6/T9/T12, in-line card → T5/T12, notes → T3/T7/T12(copy); item ③ → T10; item ④ → T11; item ⑤ → T2/T13; item ⑥ → T8/T13; item ⑦ → T13. All covered.
- **Type consistency:** `markDemoNodeAdopted(candidate, parentLeadId)` threaded StudentApp→ProjectsTab→WorkspaceContainer→ExplorationView, mirroring `markDemoNodeRead`. `pendingDemoReading` gains `readingNote` in all three type sites (T7). Anchors defined in T5/T9 are the exact strings referenced in T12/T13.
- **Ordering:** T1-T4 (seeds/backend) and T5-T11 (FE features/bugs) are independent and can run in any order; T12 depends on T5/T6/T7/T9/T11; T13 depends on T8/T2. Run in listed order.
- **Risk flags:** T2 (proposal-track seed) carries the P7 proficiency-regression risk → FULL api+agent suites (Step 5). T6 is the largest FE change (4 files threaded) → standard model, careful review. T5 changes a general (non-demo) component → verify select-mode unaffected.
