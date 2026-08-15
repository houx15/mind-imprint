# Course Runtime — Slice 2: Runtime Core (`course-runtime`)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create `packages/course-runtime` — the framework-free runtime core: host-supplied adapter interfaces, a typed runtime Event Bus, a pure deterministic WorkflowRuntime interpreter, and a session-state reducer. No React. Fully unit-tested against Slice 1's golden fixtures.

**Architecture:** The WorkflowRuntime is a pure state machine over a validated `SliceWorkflow`: entering a step **arms its transition matchers first, then returns its `enterActions` as an ordered effect list** (§12.3). The host (the renderer, Slice 3) applies effects to real blocks/players and feeds runtime Events back via `send()`. Returning effects (instead of calling a host object) keeps the interpreter pure and replayable: the same ordered event stream always yields the same effect log and final step. A `SliceSessionState` reducer folds effects + events into `BlockSessionState`/`SliceSessionState` (visible/enabled/completed/attempts). The Event Bus normalizes envelopes and binds each event to course/session/part/slice/source ids, and prevents one slice from consuming another slice's events.

**Tech Stack:** TypeScript 5.4, depends on `@mind-imprint/course-contract` (`workspace:*`), Vitest ^1.6 (node). No DOM, no React, no Zod runtime beyond re-using the contract's types.

**Authoritative spec:** `docs/2026-08-15-student-course-runtime-data-and-renderer-design.md`, especially §12 (Workflow model), §16 (CourseSession), §17.14 (WorkflowRuntime), §17.15 (Event Bus), §17.16 (Adapters).

## Global Constraints

- Package `@mind-imprint/course-runtime`, `"type": "module"`, source-only (no build), mirroring `packages/course-contract`.
- Pure & deterministic: no `Date.now()`/`Math.random()`/`new Date()` inside the interpreter or reducer. Timestamps and ids for emitted events are supplied by the host through an injected `clock`/`idFactory` on the Event Bus (so tests pass fixed values). The interpreter itself never stamps time.
- The interpreter consumes **already-validated** workflows (Slice 1 guarantees graph correctness); it must still fail loudly (throw a typed `WorkflowRuntimeError`) on an impossible runtime state (event for an unknown step, effect for an unknown target) rather than silently advancing (§12.3 last rule).
- Reuse contract types verbatim: `SliceWorkflow`, `WorkflowAction`, `WorkflowEventType`, `TargetRef`, `SliceDefinition`, `CourseSession`, `SliceSessionState`, `BlockSessionState`, `CourseRuntimeEvent`. Do not redeclare them.
- Environment: pnpm workspace; run `pnpm install` after adding the package. No `/dev/null` or `2>/dev/null` redirects (boundary hook blocks them). Never `git add -A`; stage explicit paths only.

---

### Task 1: Scaffold the package

**Files:**
- Create: `packages/course-runtime/package.json`, `tsconfig.json`, `vitest.config.ts`, `src/index.ts`
- Test: `packages/course-runtime/test/smoke.test.ts`

**Interfaces:**
- Produces: importable `@mind-imprint/course-runtime`.

- [ ] **Step 1: Write the smoke test** — assert `COURSE_RUNTIME_VERSION === "0.0.0"`.
- [ ] **Step 2: package.json** — mirror course-contract's, add `"dependencies": { "@mind-imprint/course-contract": "workspace:*" }` and keep `zod`/`typescript`/`vitest` dev/deps as needed (zod not required at runtime here; include only if a type import needs it — prefer type-only imports).
- [ ] **Step 3: tsconfig.json + vitest.config.ts** — copy course-contract's verbatim.
- [ ] **Step 4: src/index.ts** — `export const COURSE_RUNTIME_VERSION = "0.0.0";`
- [ ] **Step 5: Run** `pnpm install` (root) then `pnpm --filter @mind-imprint/course-runtime test` and `typecheck` → PASS.
- [ ] **Step 6: Commit** — stage the five files + `pnpm-lock.yaml`; `feat(course-runtime): scaffold package`.

---

### Task 2: Adapter interfaces

**Files:**
- Create: `packages/course-runtime/src/adapters.ts`
- Test: `packages/course-runtime/test/adapters.test.ts` (a compile/shape test — construct in-memory stubs and assert they satisfy the interfaces)

**Spec:** §4 (AssetResolver), §17.16 (CourseRuntimeAdapters), §6.3 (RuntimeSceneResult), §16 (SessionAdapter persistence surface).

**Interfaces:**
- Produces:
  - `AssetResolver` — `resolve(relativePath: string): string`.
  - `SessionAdapter` — `load(sessionId): Promise<CourseSession | null>`; `create(input): Promise<CourseSession>`; `appendEvent(sessionId, event): Promise<void>`; `saveSliceState(sessionId, sliceId, state): Promise<void>`; `saveScene(sessionId, which: "opening"|"closing", result): Promise<void>`; `setStatus(sessionId, status): Promise<void>`. (Exact method set — keep it minimal but sufficient for Slice 3/8.)
  - `RuntimeSceneGenerator` — `generate(input: SceneGenerationInput): Promise<RuntimeSceneResult>`. Define `SceneGenerationInput` carrying the approved facts + permitted signals (opening: title/estimatedMinutes/objectives/learningPreview/allowedSignals+signalValues; closing: preparedSummary/takeaways/transferApplications/allowedSignals+session evidence). Generator returns text (+ optional audioUrl) or a fallback-flagged result.
  - `CourseRuntimeAdapters` — `{ assetResolver; sessionAdapter; openingGenerator; closingGenerator }`.
  - `InMemorySessionAdapter` — a concrete test/preview implementation (a `Map<string, CourseSession>`); used by later slices' tests and (later) preview. Deterministic; ids/timestamps injected.

- [ ] **Step 1: Write the failing test** — build an `InMemorySessionAdapter`, `create` a session, `appendEvent`, `load` it back, assert the event is present and status transitions persist.
- [ ] **Step 2: Run → FAIL.**
- [ ] **Step 3: Implement `adapters.ts`** — the interfaces (type-only) + `InMemorySessionAdapter` (takes an `{ idFactory, clock }` so it's deterministic; `create` seeds `status: "created"`, empty `sliceStates`/`events`).
- [ ] **Step 4: Run → PASS.**
- [ ] **Step 5: Export + commit** — `feat(course-runtime): adapter interfaces + in-memory session adapter`.

---

### Task 3: Typed Event Bus

**Files:**
- Create: `packages/course-runtime/src/eventBus.ts`
- Test: `packages/course-runtime/test/eventBus.test.ts`

**Spec:** §17.15. The bus normalizes envelopes, binds course/session/part/slice/source ids, forwards to subscribers, and refuses cross-slice delivery.

**Interfaces:**
- Produces: `RuntimeEventBus` with:
  - constructor `({ courseId, sessionId, idFactory, clock })`.
  - `bindSlice(partId, sliceId)` → returns a scoped emitter `emit(sourceId, type, payload?)` that stamps a full `CourseRuntimeEvent` (id from idFactory, occurredAt from clock) and publishes it.
  - `subscribe(handler: (e: CourseRuntimeEvent) => void): () => void` (returns unsubscribe).
  - `currentSliceId` guard: events emitted for a slice other than the active one are dropped (logged via an injectable `onDropped`), never delivered — enforces "prevents one Slice from consuming another Slice's Events".
  - `setActiveSlice(sliceId)`.

- [ ] **Step 1: Write the failing test** — bind a slice, subscribe, emit `answer.correct` from `source "q1"`, assert the handler received a fully-stamped envelope (fixed id/occurredAt from injected factories). Emit an event for an inactive slice → handler NOT called, `onDropped` called.
- [ ] **Step 2: Run → FAIL.**
- [ ] **Step 3: Implement `eventBus.ts`.**
- [ ] **Step 4: Run → PASS.**
- [ ] **Step 5: Export + commit** — `feat(course-runtime): typed runtime event bus`.

---

### Task 4: WorkflowRuntime interpreter

**Files:**
- Create: `packages/course-runtime/src/workflowRuntime.ts`
- Test: `packages/course-runtime/test/workflowRuntime.test.ts`

**Spec:** §12 (all), §17.14. Pure interpreter over one slice's `SliceWorkflow`.

**Interfaces:**
- Produces:
  - `type WorkflowEffect = WorkflowAction` (re-export alias; the effect list is exactly the actions to perform, in order).
  - `class WorkflowRuntime`:
    - `constructor(workflow: SliceWorkflow, opts?: { restoreStepId?: WorkflowStepId })`.
    - `start(): WorkflowEffect[]` — enters `restoreStepId ?? initialStepId`: sets `currentStepId` (arming its transitions), then returns that step's `enterActions`. Idempotent-guarded (throws if called twice).
    - `send(event: { type: WorkflowEventType; sourceId?: string; interactionId?: string; timerId?: string }): WorkflowEffect[]` — finds the FIRST transition on the current step whose matcher matches (Slice 1 guarantees non-ambiguous, so first-match is deterministic); if matched, sets `currentStepId = to` and returns the new step's `enterActions`; if none match, returns `[]` (waiting). Throws `WorkflowRuntimeError` if `currentStepId` is unset (start not called).
    - `get currentStepId(): WorkflowStepId`.
    - `get isTerminal(): boolean` — current step's enterActions include `completeSlice` or `navigate`.
    - matcher semantics: `type` must equal; if matcher declares `sourceId`/`interactionId`/`timerId`, the event's field must equal; a matcher field omitted matches any.
  - `replay(workflow, events): { finalStepId; effects: WorkflowEffect[] }` — a helper that `start()`s and `send()`s each event, concatenating effects. Used to prove determinism.

- [ ] **Step 1: Write the failing tests** using the golden slice from Slice 1 (`import { validCourse } from "@mind-imprint/course-contract/…"` — actually import the fixture via a local copy or re-export; if the fixture is test-only in course-contract, construct a minimal slice inline here). Cover:
  - `start()` returns the initial step's enterActions in order.
  - sending the matching `narration.ended` advances to the next step and returns its actions.
  - a non-matching event returns `[]` and leaves `currentStepId` unchanged.
  - the branch: `answer.incorrect` → remediate; `answer.correct` → summarize (use the §15 golden workflow shape).
  - `replay` of a fixed event list is deterministic (call twice, deep-equal).
  - `send` before `start` throws `WorkflowRuntimeError`.
- [ ] **Step 2: Run → FAIL.**
- [ ] **Step 3: Implement `workflowRuntime.ts`.** Keep it a pure class; no timers, no async. Build a `Map<stepId, WorkflowStep>` once. The "arm transitions before enterActions" rule is satisfied structurally: set `currentStepId` (transitions now queryable) THEN return enterActions — the host applies them afterward, and any event they cause arrives via a later `send()` that already sees the armed step.
- [ ] **Step 4: Run → PASS.**
- [ ] **Step 5: Export + commit** — `feat(course-runtime): workflow interpreter`.

Note on fixtures: if importing course-contract's test fixture is awkward across packages, add a tiny exported `sampleSlice` builder in `course-runtime/test/support.ts` that constructs the §15 golden slice inline. Do NOT export test fixtures from a package's public `src`.

---

### Task 5: Slice session-state reducer

**Files:**
- Create: `packages/course-runtime/src/sessionState.ts`
- Test: `packages/course-runtime/test/sessionState.test.ts`

**Spec:** §12.1 (initialState defaults), §16 (SliceSessionState / BlockSessionState).

**Interfaces:**
- Produces:
  - `initSliceState(slice: SliceDefinition): SliceSessionState` — applies §12.1 defaults: with no `initialState`, all blocks `visible:true, enabled:true`; with `visibleBlockIds` present, listed visible / unlisted hidden; with `enabledBlockIds` present, listed enabled / unlisted disabled; omitting a list preserves that default. `completed:false`, `attempts:0` for assessment blocks. `status:"not-started"`, `elapsedSeconds:0`.
  - `applyEffect(state, effect): SliceSessionState` — pure fold of a single `WorkflowEffect` onto block state: `show/hide` → `visible`; `enable/disable` → `enabled`; `resetBlock` → `completed:false, attempts:0, answer:undefined`; `completeSlice` → `status:"completed"`. Focus/narration/timer/play effects don't change persisted block state (they're transient UI) → return state unchanged. Never mutate; return a new object.
  - `applyEvent(state, event): SliceSessionState` — folds a runtime event: `answer.submitted` → `attempts += 1`, store `answer` from payload; `block.completed`/`answer.correct` (per the block's completion rule the renderer decides; here just set `completed:true` on the sourceId block); `video.paused`/`video.ended` → `mediaPositionSeconds` from payload. Keep the mapping small and spec-anchored; document each.
- These are pure reducers the renderer uses to keep `CourseSession` in sync and hand to `SessionAdapter.saveSliceState`.

- [ ] **Step 1: Write the failing tests** — `initSliceState` for a slice with `initialState.visibleBlockIds:["a"], enabledBlockIds:[]` yields `a` visible, others hidden, all disabled. `applyEffect(state, {type:"show",targetId:"b"})` flips `b.visible`. `applyEvent(state, {type:"answer.submitted", sourceId:"q", payload:...})` increments attempts. Immutability: original state object unchanged.
- [ ] **Step 2: Run → FAIL.**
- [ ] **Step 3: Implement `sessionState.ts`.**
- [ ] **Step 4: Run → PASS.**
- [ ] **Step 5: Export, full package test + typecheck, commit** — `feat(course-runtime): slice session-state reducer`.

---

## Self-Review Notes

- **Determinism:** all time/id generation is injected (`clock`, `idFactory`) on the Event Bus and adapters; the interpreter and reducers are pure. This is what makes §12.6 "replayable deterministically from CourseSession events" hold.
- **Boundary:** the runtime never renders and never touches the DOM. Effects are data; the renderer (Slice 3) is the only thing that turns `show`/`playNarration` into pixels/audio. This keeps the module reusable by the generator/preview side exactly as the user asked.
- **Deferred:** real (backend-backed) `RuntimeSceneGenerator` implementations are Slice 7; here only the interface + an in-memory/fallback stub exist. `SessionAdapter` backed by the API is Slice 8; here only the interface + `InMemorySessionAdapter`.
