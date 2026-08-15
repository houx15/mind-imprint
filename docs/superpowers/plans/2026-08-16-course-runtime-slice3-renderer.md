# Course Runtime — Slice 3: Renderer Shell + Static Blocks (`course-renderer`)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create `packages/course-renderer` — the React rendering layer: a block-renderer registry, the four-preset LayoutRenderer, Text/Images renderers, NarrationPlayer, FocusManager, the SlicePlayer that wires the runtime to the DOM, and the CoursePlayer lifecycle (Opening → Slices → Closing) with **fallback-only** Opening/Closing. End result: a static golden course plays end-to-end, driven by the workflow.

**Architecture:** `course-renderer` is a host-agnostic React library. It never resolves URLs, persists sessions, or generates scenes itself — those come through the `CourseRuntimeAdapters` (Slice 2). `SlicePlayer` is the hub: it seeds block state with `initSliceState` (Slice 2), starts a `WorkflowRuntime`, and runs the loop — apply an effect list to block state / narration / focus, let block renderers emit runtime Events through the `RuntimeEventBus`, feed each Event to the runtime, apply the resulting effects. `CoursePlayer` owns the course-level lifecycle and the ordered linear Part/Slice walk. Block renderers are pure and selected by `type` from the registry; adding a block type later = registering a renderer, never touching the registry or layout.

**Tech Stack:** React 18, TypeScript 5.4, `@mind-imprint/course-contract` + `@mind-imprint/course-runtime` (`workspace:*`), Vitest + jsdom + `@testing-library/react` + `@vitejs/plugin-react`. Tailwind class names may be used but the package ships no Tailwind build — it emits class strings the host app styles (matches how `apps/web` owns Tailwind). Where visual correctness matters in a package test, assert structure/roles/classes, not pixels.

**Authoritative spec:** design doc §17 (Renderer Architecture) — especially §17.3–§17.8, §17.13, §17.14; §10 (Layout), §11 (Narration), §6.1/§6.2 (Opening/Closing), §9.1/§9.2 (Text/Images).

## Global Constraints

- Package `@mind-imprint/course-renderer`, `"type": "module"`, source-only. `react`/`react-dom` are **peerDependencies** (`^18.3.0`); dev-install them for tests.
- Vitest uses `environment: "jsdom"`, `globals: true`, and the react plugin — mirror `apps/web/vitest.config.ts` (read it for the exact shape). Add `test/setup.ts` importing `@testing-library/jest-dom`.
- No direct `Date.now()`/`Math.random()` in render logic — ids/timestamps come from the injected bus/adapters (Slice 2). Component-local state (e.g. gallery active item) is fine.
- Reuse Slice 2 primitives: `WorkflowRuntime`, `RuntimeEventBus`, `initSliceState`, `applyEffect`, `applyEvent`, `CourseRuntimeAdapters`, `InMemorySessionAdapter`. Reuse Slice 1 types. Do not redeclare.
- Static-block scope only. FillBlank/SingleChoice → Slice 4; Video/PDF → Slice 5; InteractiveHtml → Slice 6. Register a `NotImplementedRenderer` placeholder for the not-yet-built types so the registry is total (and a test asserts the placeholder renders for, e.g., `singleChoice`).
- No `/dev/null`/`2>/dev/null` redirects (boundary hook). Never `git add -A` — stage explicit paths. pnpm workspace: `pnpm install` after adding the package; test via `pnpm --filter @mind-imprint/course-renderer test`.

---

### Task 1: Scaffold the package (React + jsdom vitest)

**Files:** Create `packages/course-renderer/{package.json,tsconfig.json,vitest.config.ts}`, `src/index.ts`, `test/setup.ts`, `test/smoke.test.tsx`.

- [ ] **Step 1: Smoke test** — render `<div>` via RTL, assert it mounts; and assert `COURSE_RENDERER_VERSION === "0.0.0"`.
- [ ] **Step 2: package.json** — `dependencies`: `@mind-imprint/course-contract`, `@mind-imprint/course-runtime` (`workspace:*`); `peerDependencies`: `react`, `react-dom` `^18.3.0`; `devDependencies`: `react`, `react-dom`, `@types/react`, `@types/react-dom`, `@testing-library/react`, `@testing-library/jest-dom`, `@testing-library/user-event`, `@vitejs/plugin-react`, `jsdom`, `vitest`, `typescript` (copy versions from `apps/web/package.json`).
- [ ] **Step 3: tsconfig.json** — extends base; add `"types": ["vitest/globals", "@testing-library/jest-dom"]`.
- [ ] **Step 4: vitest.config.ts** — react plugin, `environment: "jsdom"`, `globals: true`, `setupFiles: ["./test/setup.ts"]`, `include: ["test/**/*.test.ts?(x)", "src/**/*.test.ts?(x)"]`. `test/setup.ts` → `import "@testing-library/jest-dom";`.
- [ ] **Step 5: Run** `pnpm install`, then `pnpm --filter @mind-imprint/course-renderer test` + `typecheck` → PASS.
- [ ] **Step 6: Commit** — `feat(course-renderer): scaffold react package`.

---

### Task 2: BlockRendererProps + registry

**Files:** Create `src/blocks/types.ts`, `src/blocks/registry.ts`, `src/blocks/NotImplementedRenderer.tsx`. Test `test/registry.test.tsx`.

**Spec:** §17.6.

**Interfaces:**
- Produces: `BlockRendererProps<TBlock>` = `{ block: TBlock; assetResolver: AssetResolver; state: BlockSessionState; visible: boolean; enabled: boolean; focusedItemId?: string; emit: (sourceId: string, type: WorkflowEventType, payload?: unknown) => void }` (the `emit` here is the slice-scoped emitter from the bus; the block passes its own id as sourceId). `BlockRenderer<T> = React.ComponentType<BlockRendererProps<T>>`. `blockRenderers: Record<BlockType, BlockRenderer<any>>` and `getBlockRenderer(type): BlockRenderer`. Unknown type → throw at lookup (fail before playback, §17.6).

- [ ] **Step 1: Failing test** — `getBlockRenderer("text")` returns a component; `getBlockRenderer("singleChoice")` returns the `NotImplementedRenderer` (registered placeholder); an unregistered string throws.
- [ ] **Step 2: Run → FAIL.**
- [ ] **Step 3: Implement** the props type, a registry seeded with `text`/`images` (Task 3–4 fill them; start with placeholders) and `NotImplementedRenderer` for `pdf`/`video`/`interactiveHtml`/`fillBlank`/`singleChoice`. `NotImplementedRenderer` renders a labelled `data-block-type` box.
- [ ] **Step 4: Run → PASS.** — [ ] **Step 5: Export + commit** — `feat(course-renderer): block registry + props`.

---

### Task 3: TextRenderer

**Files:** Create `src/blocks/TextRenderer.tsx`. Test `test/text.test.tsx`. Register `text` in the registry.

**Spec:** §9.1, §17.7. Restricted Markdown, sanitized, **no raw HTML/scripts/iframes**.

- [ ] **Step 1: Failing test** — renders `content` markdown (`**bold**` → `<strong>`); a raw `<script>` / `<iframe>` in content is NOT rendered as an element (escaped/stripped). Hidden block (`visible=false`) still occupies its slot but is `aria-hidden`/`hidden`.
- [ ] **Step 2: Run → FAIL.**
- [ ] **Step 3: Implement** with `react-markdown` + `remark-gfm` (already in the monorepo; add as dep) and **no `rehype-raw`** (so raw HTML is inert by default). Apply platform typography classes. Visibility handled by the wrapper (Task 6 SlicePlayer sets `visible`), but TextRenderer respects `visible` by rendering `hidden`/`aria-hidden` when false rather than unmounting (§10: hidden blocks keep their slot).
- [ ] **Step 4: Run → PASS.** — [ ] **Step 5: Commit** — `feat(course-renderer): text block renderer`.

---

### Task 4: ImagesRenderer

**Files:** Create `src/blocks/ImagesRenderer.tsx`. Test `test/images.test.tsx`. Register `images`.

**Spec:** §9.2, §17.8. `single`/`side-by-side`/`gallery`; alt + caption; item focus; `image.selected` event on gallery change.

- [ ] **Step 1: Failing test** — `single` renders one `<img>` with resolved `src` (assetResolver stub returns `/resolved/<path>`) + alt; `side-by-side` renders two; `gallery` renders navigation and firing "next" emits `image.selected` with the new item id via `emit`; `focusedItemId` marks the matching item (a `data-focused` attr/class).
- [ ] **Step 2: Run → FAIL.**
- [ ] **Step 3: Implement.** Resolve `item.source` through `assetResolver`. Gallery keeps local active-item state; on change call `emit(block.id, "image.selected", { itemId })`. No completion event (§9.2).
- [ ] **Step 4: Run → PASS.** — [ ] **Step 5: Commit** — `feat(course-renderer): images block renderer`.

---

### Task 5: LayoutRenderer

**Files:** Create `src/layout/LayoutRenderer.tsx`. Test `test/layout.test.tsx`.

**Spec:** §10, §17.5. Map the four presets to stable desktop CSS; reserve hidden-block positions (no reflow on reveal); canonical slot names.

**Interfaces:**
- Produces: `LayoutRenderer({ layout, renderSlot }: { layout: LayoutDefinition; renderSlot: (slotId: string, blockIds: BlockId[]) => ReactNode })`. It owns only the grid/flex frame; SlicePlayer supplies `renderSlot` which mounts the block renderers for that slot.

- [ ] **Step 1: Failing test** — `full` → one region with `data-slot="main"`; `split-horizontal` ratio `2:1` → two regions `left`/`right` with a grid-template reflecting 2:1 (assert the inline `gridTemplateColumns` or a class encoding it); `split-vertical` → rows; `grid` with 3 cells → `cell-1..3`. Every slot from the layout appears exactly once.
- [ ] **Step 2: Run → FAIL.**
- [ ] **Step 3: Implement** with CSS grid: split-* → `grid-template-columns/rows` from the ratio (`2:1` → `2fr 1fr`); grid → `repeat` cells (2 cols for 2, etc.). Each slot is a positioned region; hidden blocks inside keep their box (they render but `hidden`), so revealing doesn't reflow siblings.
- [ ] **Step 4: Run → PASS.** — [ ] **Step 5: Commit** — `feat(course-renderer): layout renderer (4 presets)`.

---

### Task 6: NarrationPlayer + injectable audio

**Files:** Create `src/narration/NarrationPlayer.tsx`, `src/narration/audioEngine.ts`. Test `test/narration.test.tsx`.

**Spec:** §11, §17.13. Prepared audio + transcript; play/pause/stop/replay; emits `narration.ended`; only one narration track at a time.

**Interfaces:**
- Produces: `AudioEngine` interface `{ play(url): void; pause(): void; stop(): void; onEnded(cb: () => void): () => void }`, a default `HtmlAudioEngine` (wraps one `HTMLAudioElement`, single-track), and a `NarrationController` the SlicePlayer drives via effects. `NarrationPlayer` renders the accessible transcript + controls for the active narration and calls `emit(narrationId, "narration.ended")` when the engine reports ended. The AudioEngine is injected through context (default `HtmlAudioEngine`) so tests supply a `FakeAudioEngine` that lets them fire `ended` synchronously.

- [ ] **Step 1: Failing test** — with a `FakeAudioEngine`, `play` a narration, fire `ended`, assert an `narration.ended` event was emitted with that narration id. Starting a second narration stops the first (single-track): assert the engine's `stop` was called before the second `play`. Transcript text is rendered.
- [ ] **Step 2: Run → FAIL.**
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run → PASS.** — [ ] **Step 5: Commit** — `feat(course-renderer): narration player with injectable audio`.

---

### Task 7: FocusManager

**Files:** Create `src/focus/FocusManager.tsx` (or a `useFocus` hook + a `FocusTarget` wrapper). Test `test/focus.test.tsx`.

**Spec:** §17 focus actions (`focus`/`clearFocus`, TargetRef with optional itemId).

**Interfaces:**
- Produces: a focus context holding the current `TargetRef | null`; block/item wrappers read it and set `data-focused="true"` + a visible focus ring class when they match. SlicePlayer sets focus from `focus`/`clearFocus` effects.

- [ ] **Step 1: Failing test** — set focus to `{ blockId: "a" }` → the block-a wrapper is `data-focused`; `{ blockId: "img", itemId: "x" }` focuses only item x; `clearFocus` removes it.
- [ ] **Step 2–4:** Run→FAIL, implement, Run→PASS. — [ ] **Step 5: Commit** — `feat(course-renderer): focus manager`.

---

### Task 8: SlicePlayer (the wiring hub)

**Files:** Create `src/slice/SlicePlayer.tsx`. Test `test/slicePlayer.test.tsx`, plus `test/support/staticCourse.ts` (a small static course: 2–3 slices, text + images, full + split layouts, workflows that advance on `narration.ended`/`student.continue` and end with `completeSlice`+`navigate`).

**Spec:** §17.4, §17.14, §12 (effect application + event routing).

**Interfaces:**
- Produces: `SlicePlayer({ slice, adapters, bus, onSliceComplete, onNavigateNext, restoreStepId? })`.
  Behavior:
  1. `initSliceState(slice)` (Slice 2) → block state; override from any restored `SliceSessionState`.
  2. `bus.setActiveSlice(slice.id)`; get the scoped emitter.
  3. `new WorkflowRuntime(slice.workflow, { restoreStepId })`; `applyEffects(runtime.start())`.
  4. Subscribe to the bus; on each event: `applyEffects(runtime.send(event))` and fold `applyEvent` into block state; persist via `adapters.sessionAdapter.saveSliceState`.
  5. `applyEffects(effects)`: for each, mutate block state (`show/hide/enable/disable/resetBlock` via `applyEffect`), drive `NarrationController` (`playNarration`/`pause`/`stop`), `FocusManager` (`focus`/`clearFocus`), block media (`playBlock`/`pauseBlock`/`resetBlock` → a per-block imperative handle registered by media renderers, Slice 5; no-op for static), timers (`startTimer` → schedule a `timer.elapsed` event through the bus; use an injectable timer so tests are deterministic), `completeSlice` → mark state completed + call `onSliceComplete`, `navigate` → `onNavigateNext`.
  6. Render `LayoutRenderer` with a `renderSlot` that mounts each block via `getBlockRenderer`, passing `visible`/`enabled`/`focusedItemId`/`state`/`assetResolver`/`emit`.
- Timer injection: accept an optional `scheduler: { setTimeout; clearTimeout }` (default the globals) so tests fire timers synchronously.

- [ ] **Step 1: Failing test** — mount `SlicePlayer` with the static support slice whose workflow: step `intro` plays narration → on `narration.ended` shows a hidden text block and enables a `student.continue` control → the test fires the continue event via the bus → workflow reaches `completeSlice`+`navigate` → assert `onSliceComplete` and `onNavigateNext` fired, and the revealed block is now visible. Use `FakeAudioEngine` to fire `narration.ended`.
- [ ] **Step 2: Run → FAIL.**
- [ ] **Step 3: Implement.** Keep effect application in one pure-ish `applyEffects` routine; block visual state in a `useReducer` over `Record<BlockId, BlockSessionState>`.
- [ ] **Step 4: Run → PASS.** — [ ] **Step 5: Commit** — `feat(course-renderer): slice player wiring runtime to dom`.

---

### Task 9: Opening/Closing scenes (fallback-only) + CoursePlayer lifecycle

**Files:** Create `src/scenes/OpeningScene.tsx`, `src/scenes/ClosingScene.tsx`, `src/course/CoursePlayer.tsx`. Test `test/coursePlayer.test.tsx`.

**Spec:** §17.3 (lifecycle), §6.1/§6.2 (Opening/Closing), §13 (navigation).

**Interfaces:**
- Produces: `CoursePlayer({ document, adapters, sessionId?, studentId })`:
  1. `validateCourseDefinition(document)` (Slice 1) — on failure render a diagnostic error surface, do not play.
  2. create/restore `CourseSession` via `adapters.sessionAdapter`.
  3. Opening: call `adapters.openingGenerator.generate(...)`; in THIS slice supply a generator that returns the static `opening.fallback` (`fallbackUsed:true`). Render `OpeningScene` (greeting text + the fixed start action labelled `一起开始吧`); on start → status `in-progress`.
  4. Walk parts/slices in linear array order; render one `SlicePlayer` at a time; `onNavigateNext` advances the index; a required incomplete slice blocks manual forward nav (§13) — but auto-advance after `completeSlice`.
  5. After the final slice: Closing via `adapters.closingGenerator.generate(...)` (fallback here) → `ClosingScene` (summary + takeaways + transfer) → status `completed`.
- `OpeningScene`/`ClosingScene` are presentational; they take a `RuntimeSceneResult` + course facts.

- [ ] **Step 1: Failing test** — mount `CoursePlayer` with the static support course + `InMemorySessionAdapter` + fallback scene generators. Assert: Opening renders with `一起开始吧`; clicking it starts slice 1; driving each slice to completion (fire the bus events) walks to the last slice; Closing renders the prepared summary + takeaways; the session in the adapter ends `status:"completed"`. Also: a structurally-invalid document renders the error surface and never mounts a SlicePlayer.
- [ ] **Step 2: Run → FAIL.**
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run → PASS.** Then run the FULL package suite + typecheck.
- [ ] **Step 5: Export the public surface** (`CoursePlayer`, `SlicePlayer`, `LayoutRenderer`, registry, `BlockRendererProps`, scene components) from `src/index.ts`; commit — `feat(course-renderer): opening/closing scenes + course player lifecycle`.

---

## Self-Review Notes

- **End-to-end proof:** Task 9's test is the slice's acceptance gate — a static course actually plays Opening → Slices (workflow-driven reveal/branch) → Closing, with session state persisted to the in-memory adapter. This is the "renders a golden course end-to-end" deliverable.
- **Deferred (registered as NotImplemented):** `fillBlank`/`singleChoice` (Slice 4), `video`/`pdf` (Slice 5), `interactiveHtml` (Slice 6). A test asserts the placeholder renders so the registry stays total.
- **Real Opening/Closing generation** (LLM/backend) is Slice 7; this slice wires the generator adapter seam and proves the fallback path.
- **Host owns Tailwind:** the package emits class strings; `apps/web` (Slice 8) provides the actual CSS. Package tests assert structure/roles/attributes, not computed pixels.
