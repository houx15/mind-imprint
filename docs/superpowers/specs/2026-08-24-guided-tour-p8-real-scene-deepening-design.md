# Guided Tour P8 — Real-scene deepening (search / reading cards / snippet guide / writing story) · Design

**Status:** DESIGNED (2026-08-24) · builds on P7 (`docs/superpowers/specs/2026-08-23-guided-tour-p7-full-projects-walk-design.md`, PROD `d3906685`)
**Spec author:** houyx15
**Owner doc for writing-project behaviour:** `docs/2026-08-09-all-statuses.md`

## Why

A fresh walkthrough of the P7 tour found that several segments still **narrate or mock** what should be shown as a **real interaction on the read-only demo**, and that the seeded demo data is too sparse to look like a real project. Investigation of the real components (five focused code investigations, 2026-08-24) proved the real scene is far more reachable than P7 assumed:

- The demo backend does **not** flatly 403 the exploration flow. `search-guidance`, `dig` (找相似) and `review` (理一理) return canned-but-real responses; only the final **采纳/add-node** is a 403 write. So **search → real result papers → paper detail → 找相似 is already live**; P7 narrated it needlessly.
- The reading room's highlight → in-line-card mechanism exists in the `Annotate` primitive but is hard-wired to no-ops in `ReadingRoom`, so clicking a highlight does nothing — "underlined sentences, no live cards."
- The 片段 guiding box is the right component (`GuidedWritingCard`) but starved of all three content sources (AI question, English example, student text), so it collapses to "a thin line." The fix is to **seed a real generated guide card** into the demo's server-side proposal-track state (user-confirmed 2026-08-24).
- The 我的笔记 (reading room) and 阅读笔记 (writing sidebar) are empty because their data sources aren't seeded / aren't carried through the demo path.
- Two defects: the "回到探索图谱" step fires while still inside the reading room (a value-dedupe guard swallows the close); tour `title`/modal-title text renders markdown literally.

The demo stays **read-only + finished**. We make real interactions reachable, seed rich real data, add the one client-side simulation the read-only wall requires (add-node), fix the two bugs, and restructure the reading + writing tour to a coherent story line.

## Decisions (user-confirmed)

1. **Seed a real generated guide card** (not an FE mock) into the demo proposal-track so the real `GuidedWritingCard` renders a genuine filled box: question tied to "中国是否让地球变得更可持续？" + English example + the student's own text.
2. **Make the search/exploration flow a real, clickable scene** — not narrated: 让印记建议检索方向 → real seeded directions → 搜索 → 3 real result papers → open a paper → 找相似 → **采纳 (client-side simulated add)** so a new node visibly appears on the graph → **enter reading on that node**.
3. **Show the in-line card live** in the reading room by wiring the existing `Annotate` click-to-reveal (a general feature, not a demo hack); the fuller loop-driven `HangingCard` is out of scope (needs a live loop 403'd in demo).
4. **Reorder the writing story** to: mode-choice → 片段 guide → 正文 (write) → left sidebar → 还需要探索 → **trigger AI 批注 button** → AI 批注 appears → 完成.
5. **Each warren root gets a real 2nd layer.** Fill the empty 我的笔记 / 阅读笔记. Reword the 我的笔记 copy to student-relatable terms (no forward-reference to evaluation). Render markdown wherever tour text shows it.

## Feasibility ground truth (from investigation)

**Exploration writes vs canned reads on demo** (`apps/api/internal/api/`):
- Canned, reachable, real-shaped: `dig` → `cannedDig()` = 3 real papers (Chen 2019 Nature Sustainability, IEA Renewables 2023, Global Carbon Budget 2023, `demo.go:62-92`); `search-guidance` → `cannedSearchGuidance()` = **1 placeholder direction** (`demo.go:106-110`, the one genuinely-fake step); `review` → `cannedExplorationReview()` placeholder.
- Flat 403 (`loadOwnedProject`): `adopt`, `attach`, `createReference`, lead/edge create/patch/delete.
- The controls-column drill-down state machine (`ExplorationView.tsx:333`, `searchStage: idle→directions→list→detail`) is reachable **without selecting a Level-2 node** — no `selectExplorationNode` hook needed. Its stage buttons currently lack `data-tour` anchors (only `search-card-trigger` has one).

**Proposal-track guide cards** persist at `project.studio_state → proposalTrack` (Go `agent.WritingTrack`); `StepGuides map[string]string` where **each value is a double-encoded JSON string** `{"prompt","example","refHint"}`. `toStepRefs` (`api/proposal_track.go:60`) attaches them to the wire `steps[].card` **unconditionally** (independent of mode). 9 fixed parts (`understanding, question-scope, thesis, resources, challenges, method, feasibility, expected, polish`) + a `research-plan` subq-define step (no card). Student text per part = `snippet` row `section="prop:<key>"`. Zod is non-strict; `prompt`/`example` must be non-empty. Setting `mode:"guided",started:true,stepIndex:0` with `understanding` seeded avoids any live generation on GET.

**Reading-room in-line card**: `<mark>` spans (`Annotate.tsx:201-231`) call `onSelectSpan` outside select-mode and render a note panel (`Annotate.tsx:241-246`) showing `activeSpan.tag`/`activeSpan.note`. `SourceDossier.tsx:59-71` `anchorToSpan()` maps each seeded anchor to `tag=dimension`, `note=answer||question`. `ReadingRoom.tsx:787-788` hard-wires `activeSpanId={null}` + `onSelectSpan={()=>{}}` — the only blocker. Anchor content already seeded (`0086`: 3 anchors with real `dimension` + probing `question`).

**我的笔记**: `ReadingRoom.tsx:238` seeds the textarea from the `readingNote` prop = reference `reading_note`. The demo path (`StudentApp.tsx:226-229` `openDemoReadingRoom` → `pendingDemoReading`) never carries it; `WorkspaceContainer.tsx:1351-1362` passes `undefined` as the 7th `openReadingSource` arg. Textarea is `disabled={demoMode}` (displays read-only, fine).

**BUG A** (`reading-nodedone-0`): closing the immersive reader relies on `setReadingView("graph")`, but the `pendingReadingView` effect dedupes by value (`WorkspaceContainer.tsx:1246-1253`) and `"graph"` was already `lastReadingView.current` (set at `reading-warren-0`); `openDemoReadingRoom` uses a different channel (`pendingDemoReading`) that never updates it → `closeReadingSource()` never runs → reader stays mounted under the "back to map" popover.

**BUG B** (markdown): body `step.text` already renders via `<Markdown>` (`TourRunner.tsx:220`); `step.title` (`:219`), demoModal title (`:131-132`), and demo transcript bubbles (`ReadingRoom.tsx:539/559`) render plain. App renderer = `Markdown` (`@/cards/Markdown`, react-markdown+remark-gfm) + inline `CardMdInline`.

**Warren 2nd layer**: 4 roots (…0290/…0291/…0292/…0293); only …0292 has children (…0294/…0295 from `0087`). The others have none.

## Design

### Part 1 — Seeds (new migrations, demo-only, all GET-readable). Next number: 0089.

- **0089 · warren 2nd layer for every root** — add `exploration_lead` children with `parent_lead_id` under …0290, …0291, …0293 (2 each), faithful to each root's question (greening/afforestation for …0290; natural-recovery-vs-planting for …0291; renewables-vs-coal for …0293). Mint fixed ids in the …029x range. Mirror `0087`'s insert shape.
- **0090 · proposal-track guide cards + prop snippets** — `jsonb_set(studio_state,'{proposalTrack}', …)` with `mode:"guided",started:true,stepIndex:0,subQuestions:[]` and `stepGuides` for all 9 fixed keys (each a double-encoded JSON string: a Chinese guiding `prompt` tied to the research question, an **English** `example` on a *different* topic per 铁律①, optional `refHint`). Plus `snippet` rows `section="prop:<key>"` carrying the student's own text for each part (mirror `0082:236-246`). Down: `studio_state - 'proposalTrack'` + delete `prop:%` snippets.
- **0091 · reading notes + curated reference list** — set `reading_note` on reference …0260 (Chen paper) with a real student note; and `jsonb_set(studio_state,'{reference}', …)` with curated entries pointing at refs …0260–0264 so the writing-room 阅读笔记 tab (`ReferencePanel` reads `studio_state.reference`, currently `[]`) shows real notes. Optionally seed `notes[]`/`takeaway` where the panel needs them.

### Part 2 — Backend: upgrade the one fake canned response

- **`cannedSearchGuidance()`** (`apps/api/internal/api/demo.go:106-110`) → return **2-3 real China-sustainability search directions** (keyword + why), matching the bar `cannedDig` sets. e.g. 卫星植被指数 (NDVI) 与可持续性的区别；中国可再生能源装机 vs 存量煤电；净零承诺的时间表与可核查性. Keep it a pure fixture (no network). Backend test asserts the new shape.

### Part 3 — Frontend features / unlocks (read-only, isDemo-scoped or general)

- **P3a · in-line card click-to-reveal** (`ReadingRoom.tsx:787-788`) — add `const [activeSpanId,setActiveSpanId]=useState<string|null>(null)`, pass `activeSpanId={activeSpanId}` + `onSelectSpan={setActiveSpanId}`. General feature (works in real reading too; additive, outside select-mode). Clicking a highlight reveals the lens's dimension + probing question. Add a `data-tour="rr-inline-card"` on the revealed note panel for spotlighting.
- **P3b · client-side add-node simulation** (`markDemoNodeAdopted`) — mirror the proven `markDemoNodeRead` pattern: a nav hook accumulates a `demoAdoptedLeads`/`demoAdoptedRefs` set in `WorkspaceContainer`; merge synthetic `ExplorationLead` (parent = the drilled root) + `Reference` (from the chosen `cannedDig` candidate) into `view.leads`/`references` before `QuestionMindmap`/counts render. Intercept `addFromDetail`/`adopt` when `isDemo` to call the sim instead of the 403 write; no-op `onLibraryChanged`. Gate the merge on `isDemo`.
- **P3c · demo 我的笔记 carry-through** — add `readingNote` to `pendingDemoReading` (`StudentApp.tsx:226-229`; type at `WorkspaceContainer.tsx:217` + `ProjectsTab.tsx:72`) and pass it as the 7th `openReadingSource` arg (`WorkspaceContainer.tsx:1351-1362`). Value from the fixture (fixed demo note) or a reference lookup.
- **P3d · write-mode demoModal mock** — add a new `demoModal` kind that renders a static mock of the chat-action choice "印记问你：这一部分，要自己写，还是我一步步带你写？ [我自己写] [一步步带我写]" (the real chip is absent on a finished demo). Faithful to 铁律②. Route its text through markdown.
- **P3e · new `data-tour` anchors** in `ExplorationView.tsx` — directions button (`explore-directions`), the `搜索` button (`explore-search`), result list rows (`explore-result`), the PaperDetail primary/采纳 action (`explore-adopt`), and the enter-reading control (`explore-enter-reading`). Tour action steps already delegate via document-capture + `closest()`.

### Part 4 — Bug fixes

- **BUG A · force-close the reader** — root-cause fix: when the immersive reader opens (`openDemoReadingRoom`/`openReadingSource`), reset `lastReadingView.current = null` so a later `setReadingView("graph")` is not swallowed by the value-dedupe guard; `closeReadingSource()` then runs and `reading-nodedone-0` shows on the graph, not over the reader. (Alternative considered: a dedicated `closeReadingRoom()` nav hook — rejected as more surface for the same effect.) Verify `reading-library-0` still closes correctly.
- **BUG B · markdown everywhere tour text shows** — render `step.title` (`TourRunner.tsx:219`) and the demoModal `Modal` title (`:131-132`) through `CardMdInline`; render demo transcript bubbles (`ReadingRoom.tsx:539/559`) through `Markdown`. Audit all tour-authored strings (segments + demoModal mocks + transcript fixture) for `**`.

### Part 5 — Restructured tour (`tour/segments/projects.ts`)

**Reading segment — real search scene (replaces P7's narrated 找相似/采纳):**
1. exploration intro + two views (existing 0/1).
2. main Q + sub-Qs (`warren-question`) + 未归类 (`warren-unfiled`).
3. 检索卡 mental model (`search-card-trigger` → `openSearchCard` modal).
4. 还需要探索 box (`needs-resources`, seeded).
5. **drill into a chosen root** (action-click `warren-question` on …0292) → real 2nd layer.
6. **让印记建议检索方向** (action-click `explore-directions`) → real seeded directions shown.
7. **搜索 a direction** (action-click `explore-search`) → real 3-paper result list.
8. **browse + open a paper** (action-click `explore-result`) → PaperDetail; narrate 找相似.
9. **采纳** (action-click `explore-adopt`) → `markDemoNodeAdopted` → **new node visibly appears** ("采纳后，这条来源就挂上你的图谱了").
10. **enter reading on the node** (action-click `explore-enter-reading`) → "点这里进入精读" → `openDemoReadingRoom()`.
11. reading room: highlights (`rr-article`) → **click a highlight → in-line card reveals** (`rr-inline-card`, "这张透镜卡划出了这句，并向你抛出一个问题") → chat while reading (`rr-chat`) → 透镜库 (`rr-deck`, narrate) → 我的笔记 (`rr-notes`, **now filled**, reworded copy) → 完成这篇 (`rr-finish`).
12. **back to map** (BUG A fixed) → node 已读 (`warren-node-read`) → view toggle → 文献库 (`library-*`).

**Writing segment — reordered story line (item 7):**
1. 边界: AI 不代笔 (existing, center).
2. **mode choice** (P3d demoModal mock) — 自己写 / 一步步带我写.
3. 提案/正文 modes (`writing-docswitch`).
4. 大纲 · 定子问题 (`writing-outline`).
5. **片段 real guiding box** (`writing-aicard`, doc:proposal/snippets) — now a genuine filled box: question + English example + student text (item 5).
6. **正文 page** (essay/draft) — "这里你自己写正文".
7. **left sidebar** on 正文: 片段 / 阅读笔记(default) / AI批注 (`writing-refpanel`).
8. **还需要探索** box in writing (`needs-resources`).
9. **trigger AI 批注** (`writing-review-trigger`, "写完后，点这里让印记通读并批注").
10. **AI 批注 appears** (`writing-annotations`, `selectRefPanelTab("anno")`, green/blue/red).
11. 完成写作 lock (`writing-finish`).

**评估 axis dimensions** — unchanged from P7.

## 铁律 compliance

Read-only throughout: the only write-shaped gesture (采纳) is a client-side visual simulation, no backend mutation. 铁律①: the seeded guide card shows the AI *question* + an English example on a *different topic* + the student's *own* text — never AI-authored body text; the mode-choice mock shows 印记 proposing and the student confirming (铁律②). The in-line card reveal and reading notes are the student's/lens's own content. No dark patterns.

## Testing

- Backend: migrations 0089-0091 apply + down; `cannedSearchGuidance` returns the new directions; demo proposal-track reads back with 9 cards (no live generation triggered); `studio_state.reference` + `reading_note` read back. **Run FULL `internal/api` + `internal/agent`** (a seeded card_instance/proficiency-style regression is exactly the P7 failure mode — the guide-card seed touches proposal-track, re-verify no guidance/proficiency counts shift).
- Frontend: segment tests (new anchors exist, action selectors, reorder ids unique); ProposalGuideReadOnly renders filled boxes from seeded track; `activeSpanId` wiring reveals the note panel; `markDemoNodeAdopted` merges a synthetic node; markdown renders in title/modal/transcript; reader force-close on `setReadingView("graph")` after immersive open.
- **Prod-browser smoke of the whole journey** (jsdom can't hit-test): drill → directions → search → result → adopt (node appears) → enter reading → click highlight (card reveals) → notes filled → back-to-map (no reader overlay) → library; writing story in the new order with the filled 片段 box and mode-choice mock; 0 console errors (no stray 403).

## Out of scope

- Making 采纳/add-source/enter/paste actually mutate on the demo (stays 403; simulated or narrated). Per-user writable demo. Live LLM.
- The fuller loop-driven `HangingCard` guided card in the reading room (needs a demo loop-seed path; the `Annotate` click-to-reveal is the faithful minimal path this slice ships).
- Live Level-2 node-dig sidebar search (still needs a `selectExplorationNode` hook; the controls-column drill-down is the real scene we ship instead).
- Reproducing any copyrighted paper text.
