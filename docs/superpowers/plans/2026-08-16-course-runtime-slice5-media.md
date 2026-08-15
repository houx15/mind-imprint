# Course Runtime — Slice 5: Media Blocks (Video + VideoInteraction + PDF)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the media block renderers to `packages/course-renderer`: `VideoRenderer` + `VideoInteractionController` (declarative cue timeline) and `PdfRenderer`, plus the VideoInteraction document schema in `packages/course-contract`. Wire `playBlock`/`pauseBlock`/`resetBlock` effects to the video via the SlicePlayer's per-block imperative handle.

**Architecture:** `VideoRenderer` owns an accessible media player and emits typed media Events; the Slice Workflow treats the video as ONE component and normally waits on its final `block.completed` (§14). `VideoInteractionController` is a declarative media timeline, NOT a second workflow engine: it watches video time, pauses at cues, renders the cue activity (reusing Slice 4's assessment widgets), evaluates it, resumes, and gates `block.completed` under `video-ended-and-interactions-completed`. `PdfRenderer` is an isolated viewer (page nav / zoom / download) that emits behavior Events but never claims learning completion (§9.3). Media playback is behind an injectable engine so jsdom tests stay deterministic (jsdom does not implement `HTMLMediaElement` playback).

**Tech Stack:** React 18, contracts + runtime packages, Vitest + jsdom + RTL. PDF rendering library choice is an implementation decision (§17.9) — this slice abstracts it behind a `PdfEngine` seam and ships a lightweight default; do NOT block on a heavy PDF lib for tests.

**Authoritative spec:** §9.3 (PDF), §9.4 (Video), §14 (VideoInteraction), §17.9 (PdfRenderer), §17.10 (Video + controller).

## Global Constraints

- Consume Slice 3 `BlockRendererProps`; register `video` and `pdf`, replacing their `NotImplementedRenderer` entries.
- **Injectable media engine** (`VideoEngine`) and **PDF engine** (`PdfEngine`) via context, defaulting to real DOM implementations, overridable with fakes in tests — no direct reliance on jsdom media playback. All emitted events flow through `emit`.
- Reuse Slice 4 assessment widgets inside video cues (single-choice / fill-blank activities, §14). Do not fork a second assessment implementation.
- `playBlock`/`pauseBlock`/`resetBlock` effects reach the video through a SlicePlayer-owned imperative registry: a media renderer registers a handle (`{ play; pause; reset }`) on mount keyed by block id; SlicePlayer calls it when applying those effects (Slice 3 left these as no-ops). This task wires the registry.
- No `Date.now()` in logic; cue timing is driven by the engine's reported currentTime. No `/dev/null` redirects; never `git add -A`.

---

### Task 1: VideoInteraction document schema (in `course-contract`)

**Files:** Create `packages/course-contract/src/videoInteraction.ts`. Test `packages/course-contract/test/videoInteraction.test.ts`. Export from course-contract `src/index.ts`.

**Spec:** §14. `VideoInteractionDocument` (schemaVersion `"1.1"`), cues with strictly-increasing `atSeconds`, activity = singleChoice | fillBlank reusing the block sub-schemas exported in Slice 1 (`ChoiceOption`, `SingleChoiceAssessment`/`FillBlankAssessment`, `*CompletionRule`).

**Interfaces:**
- Produces: `VideoInteractionDocument`, `VideoInteractionCue`, `VideoInteractionActivity` schemas + types, and `validateVideoInteraction(doc, video): ValidationIssue[]` (referential: cue ids unique; `atSeconds` strictly increasing and within `durationSeconds`; `blockId`/`source` match the owning VideoBlock).

- [ ] **Step 1: Failing test** — parse the §14 example; reject non-increasing cue times; reject a cue time beyond duration; `validateVideoInteraction` flags a source/blockId mismatch vs the owning block.
- [ ] **Step 2: Run → FAIL.** — [ ] **Step 3: Implement** (`.strict()`; reuse exported assessment sub-schemas). — [ ] **Step 4: Run → PASS.** — [ ] **Step 5: Commit** — `feat(course-contract): video interaction document schema`.

---

### Task 2: SlicePlayer media handle registry (in `course-renderer`)

**Files:** Modify `src/slice/SlicePlayer.tsx`. Create `src/media/mediaRegistry.ts`. Test `test/media/mediaRegistry.test.tsx`.

**Interfaces:**
- Produces: a `MediaHandleRegistry` context — `register(blockId, handle: { play(): void; pause(): void; reset(): void }): () => void` and internal `get(blockId)`. SlicePlayer's `applyEffects` calls `get(targetId)?.play()/pause()/reset()` for `playBlock`/`pauseBlock`/`resetBlock`.

- [ ] **Step 1: Failing test** — a fake media block registers a handle; firing a `playBlock` effect through SlicePlayer calls the handle's `play`. (Drive via a minimal slice whose workflow issues `playBlock`.)
- [ ] **Step 2: Run → FAIL.** — [ ] **Step 3: Implement** the registry + wire the three effects. — [ ] **Step 4: Run → PASS.** — [ ] **Step 5: Commit** — `feat(course-renderer): media handle registry for play/pause/reset effects`.

---

### Task 3: VideoRenderer

**Files:** Create `src/blocks/media/VideoRenderer.tsx`, `src/media/videoEngine.ts`. Test `test/media/video.test.tsx`. Register `video`.

**Spec:** §9.4, §17.10.

Behavior:
- Renders a `<video>` (poster, captions `<track>`), resolved via `assetResolver`. Registers a media handle (`play`/`pause`/`reset`).
- Emits `video.started` / `video.paused` / `video.ended` from engine events. On `video-ended` (rule `video-ended`) or on ended+all-required-cues-complete (rule `video-ended-and-interactions-completed`) emits `block.completed`.
- `VideoEngine` seam: `{ play; pause; reset; currentTime(): number; onTimeUpdate(cb); onEnded(cb); seek(t) }`. Default wraps the `<video>` element; `FakeVideoEngine` lets tests drive time/ended.

- [ ] **Step 1: Failing tests** — with `FakeVideoEngine`: `play()` emits `video.started`; firing `ended` with completion `video-ended` emits `block.completed`; poster + caption track present with resolved src.
- [ ] **Step 2: Run → FAIL.** — [ ] **Step 3: Implement.** — [ ] **Step 4: Run → PASS.** — [ ] **Step 5: Commit** — `feat(course-renderer): video renderer`.

---

### Task 4: VideoInteractionController (cue timeline)

**Files:** Create `src/blocks/media/VideoInteractionController.tsx`. Test `test/media/videoInteraction.test.tsx`. `VideoRenderer` mounts it when the block has an `interaction`.

**Spec:** §14, §17.10.

Behavior:
- Loads + validates the referenced interaction document (via `assetResolver` → the host provides the JSON; accept it as a prop/loader seam so tests inject it directly). Validates with `validateVideoInteraction`.
- Watches `engine.currentTime()`; at each cue's `atSeconds`, if `pauseVideo` pause the engine and render the cue activity (reuse Slice 4 `SingleChoiceRenderer`/`FillBlankRenderer` in an "inline cue" mode). Emits `video.interaction.shown`.
- On cue activity completion, emits `video.interaction.completed` and resumes the video (if paused). Required cues must complete before the video's `block.completed` under `video-ended-and-interactions-completed`.
- Cue fire is idempotent (each cue shows once); time monotonic assumption but tolerate seeks (a cue already completed does not re-fire).

- [ ] **Step 1: Failing tests** — with `FakeVideoEngine` + an injected interaction doc (one required cue at 42s): advance time to 42 → engine paused, `video.interaction.shown` emitted, cue activity rendered. Complete the activity → `video.interaction.completed`, engine resumed. Then fire `ended` → `block.completed` (required cue satisfied). Variant: fire `ended` with the required cue NOT completed → NO `block.completed`.
- [ ] **Step 2: Run → FAIL.** — [ ] **Step 3: Implement.** — [ ] **Step 4: Run → PASS.** — [ ] **Step 5: Commit** — `feat(course-renderer): video interaction cue timeline`.

---

### Task 5: PdfRenderer

**Files:** Create `src/blocks/media/PdfRenderer.tsx`, `src/media/pdfEngine.ts`. Test `test/media/pdf.test.tsx`. Register `pdf`.

**Spec:** §9.3, §17.9.

Behavior:
- Renders the PDF within its slot via a `PdfEngine` seam (`{ render(container, url, page); goToPage; totalPages }`), honoring `initialPage`; exposes page nav + a download action (anchor to the resolved source). Default engine can be a simple `<embed>`/`<iframe src=pdf>` for the first release (the heavy lib is a later swap); tests use a `FakePdfEngine`.
- Emits `pdf.opened` (on first view), `pdf.pageChanged`, `pdf.downloaded`. Never emits `block.completed` (§9.3 — page views are not learning completion).

- [ ] **Step 1: Failing tests** — mounting emits `pdf.opened`; clicking next page emits `pdf.pageChanged`; clicking download emits `pdf.downloaded`; NO `block.completed` ever emitted; `initialPage` honored (engine asked to render that page).
- [ ] **Step 2: Run → FAIL.** — [ ] **Step 3: Implement.** — [ ] **Step 4: Run → PASS.** Run FULL renderer suite + typecheck, and course-contract suite (Task 1 touched it). — [ ] **Step 5: Commit** — `feat(course-renderer): pdf renderer`.

---

## Self-Review Notes

- **One video, not two engines:** the Slice Workflow observes only the video block's standardized Events and its final `block.completed`; the cue timeline lives entirely inside `VideoInteractionController` and never crosses into workflow branching (§14, §17.10).
- **Assessment reuse:** cue activities render Slice 4's assessment widgets in an inline mode — no duplicate grading logic.
- **PDF honesty:** page/download Events may be recorded as behavior but never mark completion (§9.3). The renderer emits no `block.completed`.
- **Engine seams** (`VideoEngine`/`PdfEngine`) keep every test deterministic under jsdom and leave the production PDF library / media element as a later, isolated implementation choice (§17.9, §22).
