# Guided Tour — P5 (Real-Scene Fidelity + Chrome Polish) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Act on the user's post-ship feedback: (1) make the tour's "you try it" (`action`) steps' highlighted element truly clickable; (2) redesign the 印记 bubble + welcome modal to use correct design tokens, reuse `@/ui` `Button`/`Modal`, size to content (not fixed), fix cramped buttons, and add a gently bouncing 印记; (3) replace centered hand-waving with **real-scene** or **real-UI-in-a-modal** for 提问卡, the reading room + 文献库 table, and in-course mechanics; (4) highlight **all nine** evaluation-report sections accurately; (5) enrich intro copy for course categories, tool cards, and projects.

**Architecture:** Reuse the P1–P4 tour engine (`apps/web/src/tour/`). Frontend-only — the demo project `…0200` already seeds rich reading/writing/plan data, so **no migration/seed change**. Add two nav hooks to `TourNavContext` (`setReadingView`, `openDemoReadingRoom`) threaded through `WorkspaceContainer` → `ReadingBlock` with the same ref-guarded `pending*` pattern P3 used for `pendingRoom`. Add a `demoModal` field to `TourStep` so a step can pair its 印记 line with a static real-UI mock rendered in a `@/ui` `Modal`. Rewrite the reading/eval/intro segments accordingly.

**Spec:** `docs/superpowers/specs/2026-08-22-new-user-guided-tour-design.md` (this slice refines §5 courses, §6 projects, §7/§8 engine + finale). This plan's copy and per-section mapping supersede the earlier "centered explanation is acceptable" deferrals recorded in the P3 ledger.

## Global Constraints

- Reuse the P1 engine; **no new npm dependency**; all tour config stays frontend-local in `apps/web/src/tour/`.
- **Design tokens only.** Use the size/color tokens the design system defines — `text-mk-h3`/`text-mk-body`/`text-mk-small`, `text-mk-ink`/`text-mk-muted`/`text-mk-accent-700`, `bg-mk-surface`/`bg-mk-paper`, `rounded-mk-md`/`rounded-mk-lg`, `shadow-mk-lg`, `ring-mk-border`. **Never** raw `text-white` for body text (a filled accent button's label may keep `text-white` only because that is what `@/ui` `Button variant="primary"` itself uses — prefer the `Button` component over re-implementing it). Reuse `@/ui` `Button` and `Modal` wherever a button or modal is rendered.
- **mk-\* alpha trap:** mk tokens are bare CSS vars → Tailwind alpha syntax (`bg-mk-x/NN`) emits NO css. For any translucent fill use inline `rgba(...)`/`color-mix(...)` or a real color, never `bg-mk-*/NN`.
- **No live LLM in the tour.** Every AI-flavored surface is a frozen mock or the world-readable finished demo. Never trigger a real coach/LLM call.
- **Read-only demo.** The backend 403s all writes to `…0200`; tour steps must only read/navigate, never POST.
- Frontend tests: `cd apps/web && npx vitest run <path>`; full `npm run test`; `npm run typecheck`. Run tests in the FOREGROUND — do not background and yield.
- Git: stage specific files (never `git add -A`); untracked `docs/*.md` are not ours; commit per task with trailer `Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>`; the controller pushes.
- Boundary hook blocks `/dev/null` redirects (use `2>&1` / a file under the repo) and `cd` in compound commands.

---

## Task 1: Engine — action-step click-through + bubble redesign (tokens, sizing, bounce)

**Files:**
- Modify: `apps/web/src/tour/TourRunner.tsx`
- Modify: `apps/web/src/index.css` (add a `@keyframes mk-pebble-bounce` + `.mk-pebble-bounce` utility next to the existing `mk-pebble-*` keyframes)
- Test: `apps/web/test/tour/TourRunner.test.tsx` (extend)

**Interfaces:**
- Consumes: `TourStep` (existing), `useTour()`, `@/ui` `Button`, `Pebble`.
- Produces: same `TourRunner` export; behavior changes only.

**What's wrong today (root causes):**
1. The portal root `<div className="fixed inset-0 z-[100]">` captures ALL clicks, so on an `action` step the user cannot click the highlighted element to advance — the `actionEvent` listener never fires. (The dim is a box-shadow spread, which does not itself capture events; the ROOT div does.)
2. The bubble hand-rolls buttons with `text-mk-small`, several buttons crammed into one flex row that wraps on small anchors, and `text-white` on a control that is not the accent button.

**Design decisions (rulings):**
- **Click-through:** set the portal root to `pointer-events-none`. Give the popover `pointer-events-auto`. For the spotlight, render an explicit **backdrop that blocks clicks except over the anchor on `action` steps**:
  - Keep the box-shadow spotlight `<div>` as the visual (rounded dim). Set its `pointerEvents` to `"none"` on `action` steps (so the click passes through to the real element) and `"auto"` on non-action anchored steps (so it swallows anchor clicks — highlight-but-inert).
  - Add a separate full-screen **click-blocker** `<div>` (`pointer-events-auto`, transparent) BEHIND the spotlight for **non-action** steps only (`centered` OR `next`-anchored), so the dimmed area still blocks stray clicks (preserves today's modal feel). On `action` steps omit this blocker so the rest of the screen is also reachable during "you try it".
  - Net: `action` → anchor (and page) clickable, listener advances; `next`/`center` → fully blocked as before.
- **Bubble sizing:** `min-w-[280px] max-w-[400px] w-max` so it hugs content but never gets too narrow/wide; keep `overflow-y-auto` + `maxHeight: calc(100vh - 16px)` for long text. Do NOT hard-fix height.
- **Text sizes:** title `text-mk-body font-semibold text-mk-ink` (already), body `text-mk-body text-mk-ink leading-relaxed` (bump from small→body — this is the "font size is wrong" fix), meta/skip controls `text-mk-small text-mk-muted`.
- **Buttons:** use `@/ui` `Button`. Layout controls in a footer that is `flex flex-wrap items-center justify-between gap-x-3 gap-y-2` so buttons never overlap/cram. Left: `结束` as `<Button variant="link" size="sm">`. Right group (`flex items-center gap-2`): `跳过本节` (`variant="link" size="sm"`), conditional `上一步` (`variant="secondary" size="sm"`), and either the `试试看` hint pill OR `下一步` (`variant="primary" size="sm"`).
- **试试看 on action steps:** since the real element is now clickable, keep a non-button hint. Render it as a pill using tokens: `rounded-mk-full bg-mk-accent-50 px-3 py-1 text-mk-small font-semibold text-mk-accent-700` reading `点亮处可点 →`. (Communicates "click the highlight to continue".)
- **Bouncing 印记:** wrap the header `<Pebble>` in `<span className="mk-pebble-bounce inline-flex">`. Add to `index.css`:
  ```css
  @keyframes mk-pebble-bounce { 0%,100%{transform:translateY(0)} 50%{transform:translateY(-4px)} }
  .mk-pebble-bounce { animation: mk-pebble-bounce 1.6s var(--mk-ease, ease-in-out) infinite; }
  @media (prefers-reduced-motion: reduce) { .mk-pebble-bounce { animation: none; } }
  ```

- [ ] **Step 1: Add the bounce keyframes to `index.css`** next to the other `mk-pebble-*` keyframes (search `mk-pebble-blink`).

- [ ] **Step 2: Failing test** — extend `TourRunner.test.tsx`: render a segment with one `action` step (anchor present, `actionEvent`) and assert the portal root has `pointer-events: none` (or that the spotlight/root does not block — assert `style.pointerEvents === "none"` on the outermost portal div). Add a second assertion: a `next` step renders the blocker div (query for the non-action full-screen blocker). Use the observables the existing tests use.

- [ ] **Step 3: Run to verify it fails.** `cd apps/web && npx vitest run test/tour/TourRunner.test.tsx`

- [ ] **Step 4: Implement** the pointer-events restructure + bubble redesign + bouncing Pebble + `@/ui` `Button` footer per the rulings above. Keep the existing `clampToViewport` / `useLayoutEffect` measure logic and the `resolveAnchor` spotlight effect intact; only change the DOM/pointer-events/classes and the button rendering.

- [ ] **Step 5: Run + full suite + typecheck.** `cd apps/web && npx vitest run test/tour/TourRunner.test.tsx && npm run test && npm run typecheck` — all green.

- [ ] **Step 6: Commit.**
```bash
git add apps/web/src/tour/TourRunner.tsx apps/web/src/index.css apps/web/test/tour/TourRunner.test.tsx
git commit -m "feat(tour): clickable action-step spotlight + token-correct sized bubble + bouncing 印记" -m "Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 2: Engine — `demoModal` step capability + `QuestionCardMock`

Let a step pair its 印记 line with a static real-UI mock in a `@/ui` `Modal`, and build the first mock (提问卡).

**Files:**
- Modify: `apps/web/src/tour/types.ts` (add `demoModal?` to `TourStep`)
- Create: `apps/web/src/tour/mocks/QuestionCardMock.tsx` (static, non-interactive 提问卡 replica)
- Create: `apps/web/src/tour/mocks/index.tsx` (registry: `demoMockFor(kind): ReactNode`)
- Modify: `apps/web/src/tour/TourRunner.tsx` (when `step.demoModal` set, render `<Modal open>` with the mock; the 印记 bubble sits below/over it as the explanation)
- Test: `apps/web/test/tour/mocks.test.tsx` (new) + extend `TourRunner.test.tsx`

**Interfaces:**
- Produces: `TourStep.demoModal?: { kind: DemoMockKind; title?: string }`; `type DemoMockKind = "question-card"` (extensible); `demoMockFor(kind: DemoMockKind): ReactNode`.

**Design decisions (rulings):**
- A `demoModal` step is centered (ignores `anchor` for spotlight): the mock IS the focus. Render the `@/ui` `Modal` (`open`, `onClose={t.stop}`, `title={step.demoModal.title ?? null}`) containing `demoMockFor(kind)`, and keep the 印记 bubble visible for the explanation + 下一步/结束 controls. Put the bubble BELOW the modal content (inside the modal, as a footer region) so there is one focused surface, not two overlapping popovers. If the existing `Modal` can't host the bubble cleanly, render the mock inside the modal body and the 印记 explanation as the modal's footer using the same header/controls markup as the normal bubble. Reuse `@/ui` `Button` for controls.
- `QuestionCardMock` = a **static, non-interactive** replica of `apps/web/src/studio/QuestionCardModal.tsx`: header (accent `提问卡` pill + title "从大题目，问出一个值得研究的问题"), the fixed 4-step methodology strip (1 拆解 "圈出题目里的关键词" · 2 追问 "每个词到底指什么？" · 3 连接 "连到你的经历 / 已知材料" · 4 连不上就去探索 "到阅读室或搜索"), 2–3 sample chat bubbles (student = `self-end`, accent fill; ai = `self-start`, `bg-mk-paper`) using the Phoebe/China-sustainability content, and a disabled input row (placeholder "用你自己的话说……" + a disabled 发送 button). No handlers, no state, no LLM. Use design tokens; no raw hex except where mirroring is unavoidable (prefer tokens).
- Sample dialogue (use verbatim):
  - ai: "「中国是否让地球更可持续」这个题目挺大的。我们先拆一下——你觉得这里面哪个词最需要先说清楚？"
  - student: "可能是「可持续」吧，它可以指很多方面。"
  - ai: "很好。那在你最关心的那个方面——比如能源、碳排放、还是生物多样性——你更想聚焦哪一个？先选一个，我们把题目缩小。"

- [ ] **Step 1: Add `demoModal` to `TourStep`** in `types.ts` with the `DemoMockKind` type + a doc comment.

- [ ] **Step 2: Build `QuestionCardMock.tsx`** (static markup per the ruling) and `mocks/index.tsx` with `demoMockFor`.

- [ ] **Step 3: Failing test** `mocks.test.tsx`: render `demoMockFor("question-card")` and assert it shows the 提问卡 pill, the 4 methodology steps, and at least 2 chat bubbles; assert the input/发送 is disabled (non-interactive).

- [ ] **Step 4: Wire `TourRunner`** to render the modal path when `step.demoModal` is set; extend `TourRunner.test.tsx` to assert that a step with `demoModal:{kind:"question-card"}` renders the mock and still shows 印记's text + a 下一步 control.

- [ ] **Step 5: Run + typecheck.** `cd apps/web && npx vitest run test/tour/mocks.test.tsx test/tour/TourRunner.test.tsx && npm run typecheck`

- [ ] **Step 6: Commit.**
```bash
git add apps/web/src/tour/types.ts apps/web/src/tour/mocks apps/web/src/tour/TourRunner.tsx apps/web/test/tour/mocks.test.tsx apps/web/test/tour/TourRunner.test.tsx
git commit -m "feat(tour): step can pair its line with a real-UI mock modal + 提问卡 mock" -m "Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 3: Welcome modal redesign (tokens + `@/ui` + bouncing 印记)

**Files:**
- Modify: `apps/web/src/tour/WelcomeModal.tsx`
- Test: extend/adjust `apps/web/test/tour/WelcomeModal.test.tsx` if present (else add a minimal render test)

**Design decisions (rulings):**
- Keep `@/ui` `Modal`. Replace the two hand-rolled `<button>`s with `@/ui` `Button`: 课程 = `variant="primary" size="md"` full-width; 项目 = `variant="secondary" size="md"` full-width; 稍后再说 = `variant="link" size="sm"`.
- Wrap the `<Pebble size={48}>` in `<span className="mk-pebble-bounce inline-flex">`.
- Text: greeting `text-mk-h3`, body `text-mk-body text-mk-muted leading-relaxed`, footer hint `text-mk-small text-mk-muted`. Slightly larger body per feedback.
- Keep the copy (greeting, welcome line, 课程/项目/稍后, footer re-trigger hint) — only the presentation changes.

- [ ] **Step 1:** Rewrite `WelcomeModal.tsx` per the rulings (reuse `Button`, bounce wrapper, token sizes).
- [ ] **Step 2:** Update/add the render test to assert the two `Button`s + 稍后 control call `onPick("courses")`/`onPick("projects")`/`onDismiss`.
- [ ] **Step 3: Run + typecheck.** `cd apps/web && npx vitest run test/tour/WelcomeModal.test.tsx && npm run typecheck`
- [ ] **Step 4: Commit.**
```bash
git add apps/web/src/tour/WelcomeModal.tsx apps/web/test/tour/WelcomeModal.test.tsx
git commit -m "feat(tour): welcome modal uses @/ui Button + token sizes + bouncing 印记" -m "Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 4: Nav hooks — `setReadingView` + `openDemoReadingRoom` (thread to ReadingBlock)

Enable the tour to drive the reading room into a specific inner view (list/graph) and into the 精读 immersive view, deterministically (auto-navigate, not relying on user clicks).

**Files:**
- Modify: `apps/web/src/tour/types.ts` (extend `TourNavContext`)
- Modify: `apps/web/src/shell/StudentApp.tsx` (assemble the new hooks)
- Modify: `apps/web/src/workspace/WorkspaceContainer.tsx` (hold `pendingReadingView`; pass to `ReadingBlock`; expose a setter to the tour path used for `setStudioRoom`)
- Modify: `apps/web/src/workspace/blocks/ReadingBlock.tsx` (accept `forceView?: "list" | "graph"` prop; apply via ref-guarded effect to `viewModeMemo` + `setViewModeRaw`, mirroring `pendingRoom`)
- Modify: `apps/web/src/workspace/blocks/ReadingBlock.tsx` — add `data-tour="reading-view-list"` and `data-tour="reading-view-graph"` to the two `ViewModeToggle` buttons (for optional action-step use / assertions)
- Test: `apps/web/test/workspace/ReadingBlock.forceView.test.tsx` (new) + extend the StudentApp tour nav test

**Interfaces:**
- Produces on `TourNavContext`:
  - `setReadingView: (view: "list" | "graph") => void` — switches the open project's reading room inner view.
  - `openDemoReadingRoom: () => void` — opens a seeded demo material into the 精读 immersive reading room (best-effort; see ruling).
- `ReadingBlock` gains `forceView?: "list" | "graph"` (applied once per distinct value via a ref guard so it never fights the user's manual toggling — same pattern as `pendingRoom` in `WorkspaceContainer`).

**Design decisions (rulings):**
- Mirror the existing `pendingRoom` mechanism exactly (see `WorkspaceContainer.tsx:102,125,1077-1079,1346-1357`). `setReadingView` sets `pendingReadingView` state in `WorkspaceContainer`; it is passed as `forceView` to `ReadingBlock`; `ReadingBlock` applies it in a `useEffect` guarded by a `useRef` that records the last-applied value, calling `viewModeMemo.set(projectId, v)` + `setViewModeRaw(v)`. This overrides the mount-time "always graph" default only when the tour asks.
- `openDemoReadingRoom`: open the demo's first material (`…0260`, linked to a reference) into the 精读 room via the existing `setReadingSource` path. **Ruling:** implement the hook to open the immersive reading room for a seeded material id; if wiring the immersive open cleanly from outside proves fragile within this task, land `setReadingView` (the higher-value, explicitly-requested one) and stub `openDemoReadingRoom` to `setReadingView("list")` as a safe fallback, leaving a `// TODO(P5-T5)` note — Task 5 will decide whether the 精读 segment goes real-scene or modal-mock based on what this hook can deliver. Record which path you shipped in the report.
- **No backend change** — the demo already seeds 5 references + 2 materials + warren leads.

- [ ] **Step 1: Failing test** `ReadingBlock.forceView.test.tsx`: mount `ReadingBlock` with `forceView="list"` for a project whose data has ≥1 reference (use the existing reading test fixtures/mocks) and assert it renders the `[data-tour="library-table"]` (list view), not the graph. Then rerender with `forceView` unchanged and assert it does NOT re-force after a manual toggle to graph (ref guard).

- [ ] **Step 2: Run to verify it fails.** `cd apps/web && npx vitest run test/workspace/ReadingBlock.forceView.test.tsx`

- [ ] **Step 3: Implement** `forceView` in `ReadingBlock` (+ the two toggle `data-tour` anchors), `pendingReadingView` in `WorkspaceContainer`, and the `TourNavContext` additions assembled in `StudentApp` (+ `openDemoReadingRoom` per the ruling). Extend the StudentApp tour-nav test to prove `setReadingView`/`openDemoReadingRoom` reach the container.

- [ ] **Step 4: Run + full suite + typecheck.** `cd apps/web && npx vitest run test/workspace/ReadingBlock.forceView.test.tsx && npm run test && npm run typecheck`

- [ ] **Step 5: Commit.**
```bash
git add apps/web/src/tour/types.ts apps/web/src/shell/StudentApp.tsx apps/web/src/workspace/WorkspaceContainer.tsx apps/web/src/workspace/blocks/ReadingBlock.tsx apps/web/test/workspace/ReadingBlock.forceView.test.tsx apps/web/test/shell/StudentApp.tour.test.tsx
git commit -m "feat(tour): setReadingView + openDemoReadingRoom nav hooks (forceView threaded to ReadingBlock)" -m "Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 5: Reading segments → real-scene

Rewrite the reading segments in `projects.ts` so warren map + 文献库 table are spotlighted on the REAL demo elements, and 提问卡-in-reading + 精读 use real-scene or the `demoModal` mock instead of centered text.

**Files:**
- Modify: `apps/web/src/tour/segments/projects.ts` (`reading-warren`, `reading-room`, `reading-library` segments)
- Test: extend `apps/web/test/tour/segments.test.ts` (or the projects-segment test) to assert the reading steps now carry real anchors / demoModal as specified

**Design decisions (rulings) — the new reading flow:**
1. **`reading-warren`** (enter reading, graph view):
   - step 0 `onEnter`: `nav.setStudioRoom("reading"); nav.setReadingView("graph")` — centered intro line.
   - step 1 anchor `[data-tour="reading-viewtoggle"]` — "阅读房间有两种视图：**图书馆**（管理读过的资料）和**探索**（围绕问题找线索）。现在看到的是探索视图。"
   - step 2 anchor `[data-tour="warren-question"]` — main-question copy (as today).
   - step 3 anchor `[data-tour="warren-unfiled"]` — 未归档 copy (as today).
   - step 4 = the AI-keyword point → **`demoModal:{kind:"question-card"}`? No** — keyword is a different surface. Ruling: keep step 4 as a centered line anchored to `[data-tour="explore-keyword"]` IF that element renders in the demo graph without a selected node; the implementer must verify at build time. If it does not render, make step 4 centered (no anchor) with the same copy. Record which was used.
2. **`reading-library`** (switch to list, real table):
   - step 0 `onEnter`: `nav.setStudioRoom("reading"); nav.setReadingView("list")` — centered: "切到**图书馆**视图，看看已经读过、收进来的资料。"
   - step 1 anchor `[data-tour="library-table"]` (now real — demo has 5 references) — "每精读完一篇资料，它就会带着你的笔记和评估收进这张表。点开任意一行，能回到当时的笔记和标注——写作要引用时，来这里翻。"
3. **`reading-room`** (精读):
   - Ruling depends on Task 4's `openDemoReadingRoom` outcome. **If** it reliably opens the immersive 精读 view: step 0 `onEnter: nav.openDemoReadingRoom()`, then spotlight real elements (正文区, 划句即问, 工具卡/笔记 controls, 完成精读) using data-tour anchors — add any missing anchors in the immersive reading component in THIS task. **Else** (fallback): keep the 精读 explanation as ONE centered step plus a `demoModal` illustrating 划句即问 is out of scope — instead keep the current 4 centered steps but tighten them to 2, clearly framed as "精读一篇资料时（示例项目里这篇已经读完，收进了图书馆）". Record which path shipped.
- Every reading step must still work if an anchor is briefly absent (the engine falls back to centered via `resolveAnchor` null-on-timeout) — but the goal is that with the demo data + `setReadingView`, the intended anchors resolve immediately.

- [ ] **Step 1:** Rewrite the three reading segments per the ruling. At build time, run the app path mentally / via the segment test to confirm the anchors used exist for the demo. Where an anchor is conditional and unverifiable in a unit test, prefer the real anchor and note it for the whole-branch browser smoke.
- [ ] **Step 2:** Update the segments test to assert: `reading-warren` step 1 anchors `reading-viewtoggle`, step 2 `warren-question`, step 3 `warren-unfiled`; `reading-library` step 1 anchors `library-table` and its step 0 onEnter calls `setReadingView("list")` (assert via a mock nav ctx capturing calls).
- [ ] **Step 3: Run + typecheck.** `cd apps/web && npx vitest run test/tour && npm run typecheck`
- [ ] **Step 4: Commit.**
```bash
git add apps/web/src/tour/segments/projects.ts apps/web/test/tour
git commit -m "feat(tour): reading room + 文献库 shown in real-scene (warren/library anchors, real demo data)" -m "Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 6: Eval report all-9-sections + richer intro copy + 提问卡 in 立题

Two content changes bundled (same file family, both copy/step edits, one reviewer surface): the evaluation-report segment gets one accurate step per `#s1..#s9`; the 立题 提问卡 step switches to the `demoModal`; and the intro/category/tool-card copy is enriched.

**Files:**
- Modify: `apps/web/src/tour/segments/projects.ts` (`evaluation-report` segment; `forming` 提问卡 step; `projects-intro` copy)
- Modify: `apps/web/src/tour/segments/courses.ts` (`courses-categories`, `courses-tujian` copy)
- Test: extend the segments test to assert the eval segment has 9 anchored steps `#s1..#s9` and the 立题 step carries `demoModal`.

**Design decisions (rulings):**
- **Evaluation-report segment** — replace the 3-step (s1/s5/s9) version with 10 steps: an intro (center, as today) + one step per section. Anchors `#s1..#s9`, `placement:"right"` for narrow sections and `"top"` for wide ones (implementer picks per section width; default `"right"`). Copy per section (verbatim):
  - `#s1` 基本信息："报告开头是**项目基本信息**——课题、类型、起止时间、里程碑，还有几个关键计数：和 AI 聊了多少轮、读了几篇资料、写了多少字。"
  - `#s2` 综述："**综述**用一段话讲清这个项目整体是怎么推进的——资料、写作、和 AI 协作各自的样子，最后给出改进建议和推荐课程。"
  - `#s3` 过程时间线："**过程时间线**把你做过的每一步按时间排开——什么时候聊、什么时候读、什么时候写，每一步用了几轮 AI。过程被看见，而不只是结果。"
  - `#s4` 材料清单："**材料清单**列出你用过的每一份资料：最终怎么判定、能支撑什么、不能支撑什么、用在了哪里。"
  - `#s5` 认知深度 D："**认知深度 D**——你的思考钻得有多深。六个维度各有一个评级、对应的行为证据，和下一步建议。"
  - `#s6` 智识自主 A："**智识自主 A**——你在多大程度上是自己在推进思考，而不是被 AI 牵着走。同样六个维度、带证据。"
  - `#s7` 提问透镜："**提问透镜**回看你问 AI 的那些问题——你怎么问，往往最能反映你怎么想。"
  - `#s8` 工具卡与子代理："**工具卡与子代理**记录你这一路召唤过哪些思维工具（CRAAP、溯源、让步段……）以及它们帮你做了什么。"
  - `#s9` 风险提示："末尾是**风险提示**——这次项目里的薄弱环节或需要警惕的地方，供你下次做得更好。这份报告就是「你的思维印记」：记录你怎么想，不只是你写了什么。"
- **立题 提问卡 step** (`forming-2`): set `demoModal:{kind:"question-card", title:"提问卡 · 帮你把题目想清楚"}`, keep the 铁律-safe copy ("…让印记用一串问题帮你把题目想清楚——而不是直接帮你定题，定题这件事得是你自己的。"). Remove the `placement:"center"` text-only framing (the mock is now the focus).
- **`projects-intro`** — enrich to 2 steps (keep) but make them more concrete: mention论文/报告/申请文书 examples (keep) AND add that the过程 becomes评估 (keep). Add one concrete sentence to step 0 about the五个房间 existing so the学生 has a map. Keep 铁律 boundary line.
- **`courses-categories`** — name what the categories are for: "课程按主题分了几类——有的练**溯源与信息甄别**，有的练**论证与思辨结构**，有的练**研究方法**。用这些标签快速筛到你当下最想练的一类。" (Do not invent exact category names beyond what the catalog shows; keep to themes.)
- **`courses-tujian`** step 0 — say what a card IS: "这是你的**思维工具卡图鉴**。每张卡是一种**可复用的思考方法**——比如 CRAAP 用来给资料做溯源体检、让步段用来处理反例。练过一次，卡就会被点亮、攒起星星。"

- [ ] **Step 1:** Rewrite the `evaluation-report` segment (intro + 9 section steps), the `forming-2` 提问卡 step (demoModal), `projects-intro`, `courses-categories`, `courses-tujian-0` per the rulings.
- [ ] **Step 2:** Extend the segments test: eval segment has steps anchoring `#s1..#s9` (all nine present, in order); `forming-2` has `demoModal.kind === "question-card"`.
- [ ] **Step 3: Run + full suite + typecheck.** `cd apps/web && npx vitest run test/tour && npm run test && npm run typecheck`
- [ ] **Step 4: Commit.**
```bash
git add apps/web/src/tour/segments/projects.ts apps/web/src/tour/segments/courses.ts apps/web/test/tour
git commit -m "feat(tour): eval report all 9 sections + 提问卡 modal in 立题 + richer category/card/project copy" -m "Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 7: In-course mechanics on real elements

The courses tour currently has two centered `courses-player` steps. Enter the real course the user clicked and spotlight the REAL player controls; use the `demoModal` for the LLM-driven 提问 part.

**Files:**
- Modify: `apps/web/src/tour/segments/courses.ts` (`courses-enter` + `courses-player`)
- Possibly modify the course player component to add `data-tour` anchors if the existing hooks (`.course-nav__next`, `data-testid="course-region"`, `data-testid="ask-bubble"`) are insufficient — grep first; prefer existing hooks.
- Test: extend the courses-segments test.

**Design decisions (rulings):**
- `courses-enter-0` stays an `action` step (now clickable via Task 1): user clicks a real course card → the course opens. Its `actionEvent` selector targets a course card inside `[data-tour="courses-grid"]` (verify the selector resolves to a clickable card, e.g. `[data-tour="courses-grid"] a, [data-tour="courses-grid"] button`).
- `courses-player` (inside the real course):
  - step 0 anchor the real slide/next control (`.course-nav__next` if present; else `[data-testid="course-region"]`) — "进入一门课后，内容一屏一屏推进：印记先讲解，再请你回答小问题。想清楚了，点这里往下走——**不用赶**。"
  - step 1 = the in-course 提问: if a real ask box exists (`[data-testid="ask-bubble"]`), anchor it — "遇到疑问，随时用这个提问框问我。" Else use `demoModal:{kind:"question-card"}` framed as在课程里提问. Ruling: prefer the real ask-box anchor; fall back to demoModal. Record which shipped.
- Keep it to ≤2 steps so the tour doesn't overstay inside a live course. Do not complete or write to the course (no live LLM).

- [ ] **Step 1:** grep the course player for existing anchors/testids; wire `courses-enter`/`courses-player` per the ruling (add `data-tour` anchors only if no suitable hook exists).
- [ ] **Step 2:** Extend the courses-segments test to assert `courses-player` step 0 has a real anchor and step 1 has either a real anchor or `demoModal`.
- [ ] **Step 3: Run + full suite + typecheck.** `cd apps/web && npx vitest run test/tour && npm run test && npm run typecheck`
- [ ] **Step 4: Commit.**
```bash
git add apps/web/src/tour/segments/courses.ts apps/web/test/tour
git commit -m "feat(tour): in-course mechanics shown on real player controls" -m "Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 8: Backend — surface the demo project + its report in every user's lists (`isDemo` flag)

Today `listProjects` (`apps/api/internal/api/projects.go:114`, `ListProjectsByUser`) and `listEvaluationReports` (`apps/api/internal/api/evaluation_report.go:94`, `ListEvaluationReports`) return only the caller's OWN rows — so the world-readable demo project `…0200` (owner Phoebe `…0003`) is invisible to every other user. The user wants the demo (and its evaluation report) shown in EVERY user's list, pinned last, clearly marked. This task adds the backend support; Task 9 does the UI.

**Files:**
- Modify: `apps/api/internal/store/queries/*.sql` (add `ListDemoProjects` + `ListDemoEvaluationReports`; find the file holding `ListProjectsByUser` / `ListEvaluationReports`)
- Run: `make sqlc` (regenerates `apps/api/internal/store/sqlc/*` — CGO_ENABLED=0, sqlc pinned v1.27.0)
- Modify: `apps/api/internal/api/projects.go` (`projectListItem` gains `IsDemo bool json:"isDemo"`; `listProjects` appends demo rows the user doesn't own, and marks an owned demo row)
- Modify: `apps/api/internal/api/evaluation_report.go` (`entry` gains `IsDemo bool json:"isDemo"`; `listEvaluationReports` appends the demo report)
- Test: `apps/api/internal/api/projects_test.go` + an evaluation-report list test (extend existing) — assert a non-owner's list includes the demo marked `isDemo:true`, and the owner's list has exactly one demo entry (deduped) marked `isDemo:true`.

**Interfaces:**
- Produces: `GET /projects` items carry `isDemo`; `GET /evaluation-reports` (or whatever the route is — confirm in `api.go`) entries carry `isDemo`. The demo appears exactly once per list for every authenticated user.

**Design decisions (rulings):**
- New sqlc queries select `is_demo = true` rows (there is one demo today, but query generically). `ListDemoProjects` returns the same columns `ListProjectsByUser` does (id, title, qualification, status, cover, created_at, last_active_at). `ListDemoEvaluationReports` returns project_id, title, qualification, created_at for `evaluation_report` rows whose project `is_demo`.
- Merge logic (both endpoints): build a set of the user's own project ids. For each demo row: if the user already owns it (owner viewing), set `IsDemo=true` on the existing item (do NOT duplicate); else append a fresh item with `IsDemo=true`. Demo items go at the END of the slice (append after own rows; Task 9 also sorts client-side as belt-and-suspenders).
- The count chips (aiCalls/activityLog) for an appended demo the user doesn't own can be 0 (best-effort, same as today's error path) — do not add per-demo count queries.
- No migration/seed change — `is_demo` already exists (0081) and `…0200` is flagged (0082).

- [ ] **Step 1: Failing test.** Extend `projects_test.go`: seed a second user (non-owner) and assert their `GET /projects` includes the demo project with `isDemo:true` as the LAST entry, and their other projects have `isDemo:false`. (Use existing test seed helpers / the demo seeded by migrations in the test DB.)
- [ ] **Step 2: Run to verify it fails.** `cd apps/api && CGO_ENABLED=0 go test ./internal/api/ -run TestListProjects -timeout 1800s` (adjust -run to the actual test name).
- [ ] **Step 3: Add the two sqlc queries + `make sqlc`; implement the merge in both handlers + add `IsDemo` to both DTOs.**
- [ ] **Step 4: Run the api suite.** `cd apps/api && CGO_ENABLED=0 go test ./internal/api/ -timeout 1800s` — green. (Run in FOREGROUND; testcontainers boots a throwaway Postgres.)
- [ ] **Step 5: Commit.**
```bash
git add apps/api/internal/store/queries apps/api/internal/store/sqlc apps/api/internal/api/projects.go apps/api/internal/api/evaluation_report.go apps/api/internal/api/projects_test.go
git commit -m "feat(demo): surface the read-only demo project + its report in every user's lists (isDemo)" -m "Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 9: Frontend — demo tag/title, sort-to-bottom, and guide-or-leave guard modal

**Files:**
- Modify: `packages/contracts/src/workspace.ts` (or wherever the project-list item + report-list entry schemas live — grep) to add `isDemo: z.boolean().optional().default(false)` to BOTH list-item schemas; and `apps/web/src/api/projects.ts` + `apps/web/src/api/evaluationReport.ts` if they carry their own parse shape.
- Modify: `apps/web/src/workspace/WorkspaceContainer.tsx` (the project directory list: render a 「示例」 tag chip + title marker on `isDemo` cards; sort `isDemo` items to the bottom; intercept a manual click on an `isDemo` card → open the guard modal instead of the studio)
- Create: `apps/web/src/workspace/DemoProjectGuardModal.tsx` (reuse `@/ui` `Modal` + `Button`)
- Modify: `apps/web/src/shell/report/ReportsView.tsx` (render the 示例 tag on the demo report entry + sort it to the bottom)
- Modify: `apps/web/src/shell/StudentApp.tsx` + `apps/web/src/shell/ProjectsTab.tsx` (thread an `onRequestDemoTour` callback so the guard modal's "带我逛" plays `journeyStarting("projects")`)
- Test: extend the relevant component tests (WorkspaceContainer directory / ReportsView) + a DemoProjectGuardModal render test.

**Design decisions (rulings):**
- **Tag + title:** on an `isDemo` list card/row, render a small pill using tokens (`rounded-mk-full bg-mk-accent-50 px-2 py-0.5 text-mk-small font-semibold text-mk-accent-700`) reading `示例`, placed before/adjacent to the title, AND prefix nothing destructive to the stored title — show the marker as a chip next to the real title. (Do not mutate the title string sent to the server.)
- **Sort to bottom:** stable-sort the list so `isDemo` items come last, preserving existing order otherwise.
- **Guard modal (project only):** clicking an `isDemo` project card does NOT open the studio. It opens `DemoProjectGuardModal`:
  - title `示例项目`, body: "这是一个只读的**示例项目**，用来给你演示「项目」是怎么用的——你不能在里面编辑。要我带你逛一遍吗？"
  - primary `@/ui` `Button` "好，带我逛一遍" → calls `onRequestDemoTour()` (StudentApp plays `journeyStarting("projects")`) and closes the modal.
  - secondary `Button variant="secondary"` "不用了" → closes the modal, stays on the project list (does NOT enter the studio). This is the whole point: prevent free-roam edits of the read-only demo.
- **Tour-driven open bypasses the guard:** the tour opens the demo via `initialProjectId={DEMO_PROJECT_ID}` (StudentApp `openDemoProject` → `pendingProjectId`), which is the initial-open path in WorkspaceContainer, NOT a manual card click. Ensure the guard only fires on the manual click handler, so the tour still drives straight into the studio. Verify this separation explicitly.
- **Report entry:** clicking the demo report just opens the read-only report (reports have no edit-bug risk) — no guard modal there; only the tag + sort-to-bottom.

- [ ] **Step 1:** Add `isDemo` to the two list-item contracts (+ any frontend parse). Typecheck to find all consumers.
- [ ] **Step 2:** Build `DemoProjectGuardModal.tsx` + a render test (asserts the two buttons call `onGuide`/`onClose`).
- [ ] **Step 3:** In `WorkspaceContainer`, sort `isDemo` last, render the 示例 chip, and intercept the manual open of an `isDemo` card → guard modal; thread `onRequestDemoTour`. In `ReportsView`, tag + sort the demo report last.
- [ ] **Step 4:** Thread `onRequestDemoTour` from `StudentApp` (plays `journeyStarting("projects")`) through `ProjectsTab` to `WorkspaceContainer`.
- [ ] **Step 5: Tests + full suite + typecheck.** `cd apps/web && npx vitest run <touched tests> && npm run test && npm run typecheck`
- [ ] **Step 6: Commit.**
```bash
git add packages/contracts/src apps/web/src/api apps/web/src/workspace/WorkspaceContainer.tsx apps/web/src/workspace/DemoProjectGuardModal.tsx apps/web/src/shell/report/ReportsView.tsx apps/web/src/shell/StudentApp.tsx apps/web/src/shell/ProjectsTab.tsx apps/web/test
git commit -m "feat(demo): 示例 tag + sort-to-bottom + guide-or-leave guard modal for the demo project" -m "Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Self-Review (author's pass)

- **Feedback coverage:** action-step clickable (T1) ✓; bubble not-fixed + tokens + reuse `@/ui` + no cramped buttons + bouncing 印记 (T1, T3) ✓; 提问卡 real-UI modal (T2, T6) ✓; reading + 文献库 real-scene (T4, T5) ✓; in-course mechanics real (T7) ✓; eval all 9 sections (T6) ✓; richer copy for categories/cards/projects (T6) ✓; modal-with-real-UI capability (T2) ✓.
- **No backend/seed change** — demo `…0200` already carries the reading/writing/plan data these steps spotlight. If any reading anchor turns out unpopulated at browser-smoke time, that's a demo-content gap to fix in a follow-up, not a blocker for the engine/copy work.
- **Type consistency:** `TourNavContext` gains `setReadingView`/`openDemoReadingRoom`; `TourStep` gains `demoModal?`; `ReadingBlock` gains `forceView?`. Names used identically across types.ts, StudentApp, WorkspaceContainer, ReadingBlock, and the segments.
- **铁律 safety:** every AI surface is a frozen mock or the finished world-readable demo; 提问卡 copy keeps "AI 不替你定题"; writing room keeps "正文永远你自己写". No live LLM, no writes to the demo.
- **Deferred/ruled fallbacks:** `openDemoReadingRoom` and the 精读 / in-course 提问 steps carry explicit real-scene-else-mock rulings so a hard-to-drive immersive surface never blocks the slice; the shipped path is recorded in each task's report and reconciled at the whole-branch browser smoke.
