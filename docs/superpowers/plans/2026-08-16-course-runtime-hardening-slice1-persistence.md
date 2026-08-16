# Slice 1 — Typed Payloads + Persistence + Resume

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development. Steps use `- [ ]`.

**Goal:** Runtime events, workflow/slice position, elapsed time, media position, answers, and interaction results are correctly persisted to the `CourseSession`, and an existing session truly resumes (not restart). Fixes review findings **P1-04**, **P2-02**, and the client-durability core of **P2-07**.

**Requirements source (read the cited evidence):** `docs/2026-08-16-student-course-runtime-code-review.md` — findings P1-04, P2-02, P2-07. Program design: `docs/superpowers/specs/2026-08-16-course-runtime-hardening-program-design.md`.

**Architecture:** typed event payloads defined once in `course-runtime`; producers (renderer emitters) and reducers consume the SAME types; `SlicePlayer` persists every accepted event via `sessionAdapter.appendEvent`; `CoursePlayer` restores from a validated existing `CourseSession`; the web `apiSessionAdapter` never loses dirty state.

## Global Constraints
- Determinism preserved: no `Date.now()`/`Math.random()` in `course-runtime`/`course-renderer` pure paths — time/ids stay injected (`clock`/`idFactory`).
- One shared typed-payload module in `course-runtime`; do NOT hand-write separate payload assumptions in producers/reducers (P2-02 root cause).
- Do NOT break existing green tests (course-contract 56 / course-runtime 28 / course-renderer 97 / web). Run each package's vitest in the FOREGROUND (fast; no testcontainers).
- Restore must validate the incoming `CourseSession` (via the contract) before trusting it; a malformed/incompatible session degrades to a fresh start, never a crash.

---

### Task 1 — Typed event payloads + reducer correctness (course-runtime)

**Files:** `packages/course-runtime/src/eventBus.ts`, `packages/course-runtime/src/sessionState.ts`, a new `packages/course-runtime/src/eventPayloads.ts`, `packages/course-runtime/src/index.ts`; tests under `packages/course-runtime/test/`.

- [ ] Define typed payloads (new `eventPayloads.ts`, exported): at minimum `answer.submitted {value: string}`, `video.paused/video.ended {positionSeconds: number}`, `video.started {}`, `interaction.completed {interactionId: string, result: <typed>}`, `video.interaction.completed {interactionId: string, result: <typed>}`, `narration.ended {narrationId?}`, `timer.elapsed {timerId}`, `answer.correct/incorrect/attemptsExhausted`, `block.completed {blockId?}`. Mirror the event names in `course-contract`'s `WorkflowEventType`.
- [ ] Fix reducers in `sessionState.ts` (review P2-02 evidence, lines ~114–141): `answer.submitted` must read `payload.value` (currently reads `payload.answer` → undefined); `video.paused/ended` read `payload.positionSeconds`; ADD reducer cases for `interaction.completed` and `video.interaction.completed` populating `BlockSessionState.interactionResult`.
- [ ] Maintain session position/time in the reducer/session state: current part/slice/workflow-step, status timestamps, elapsed time, media position, answers, attempt state, results — per `course-contract/src/session.ts` `CourseSession` shape (lines 15–66). Add reducer handling / helpers so a snapshot captures these.
- [ ] Tests: reducer round-trips for each event type using the TYPED payloads; assert `answer.submitted {value:"x"}` stores `x` (not undefined); video position stored; interaction result stored. Run `pnpm --filter @mind-imprint/course-runtime test` + `typecheck` → green.
- [ ] Commit.

---

### Task 2 — appendEvent wiring + resume/restore (course-renderer)

**Files:** `packages/course-renderer/src/slice/SlicePlayer.tsx`, `packages/course-renderer/src/course/CoursePlayer.tsx`, the emitters `blocks/assessment/SingleChoiceRenderer.tsx` + `FillBlankRenderer.tsx` + `blocks/media/VideoRenderer.tsx`; tests under `packages/course-renderer/test/`.

**Interfaces from Task 1:** the typed payload types + the corrected reducer/session-state helpers.

- [ ] **Persist events:** in `SlicePlayer` (review P1-04, lines ~198–224 subscribe-and-fold) call `sessionAdapter.appendEvent(event)` for every accepted event, before/atomically with the local fold. Provenance already stamped by the event bus — pass the full envelope.
- [ ] **Align emitters to typed payloads:** `SingleChoiceRenderer`/`FillBlankRenderer` emit `answer.submitted {value}` (they already do — confirm the reducer now matches); `VideoRenderer` emit `video.paused/ended {positionSeconds}` (currently emits WITHOUT position — add it from the video engine's current time). No handwritten payloads that diverge from Task 1's types.
- [ ] **Restore:** `CoursePlayer` (review P1-04, lines ~84–137 + 205–217) — when a non-null `sessionId`/existing `CourseSession` is provided, VALIDATE it (contract), then restore opening/closing, current Slice index, and pass `restoreStepId`/`restoreState` into `SlicePlayer` (which already accepts them, lines ~40–56) instead of resetting to opening + index 0 + regenerating opening. A course with no/invalid prior session still starts fresh at opening.
- [ ] Tests: mounting with an existing mid-course session restores the same Slice + workflow step + block state (not opening/0); an accepted answer/video event calls `appendEvent` with the typed payload. Run `pnpm --filter @mind-imprint/course-renderer test` + `typecheck` → green.
- [ ] Commit.

---

### Task 3 — Durable client persistence + server snapshot validation (host)

**Files:** `apps/web/src/course/apiSessionAdapter.ts`, `apps/web/src/shell/courses/RuntimeCoursePlayer.tsx`, `apps/api/internal/api/course_session.go`; tests under `apps/web/test/` (+ a Go test).

**Requirements source:** review P2-07 (client evidence lines ~47–64 of apiSessionAdapter; host flush ~60–75 of RuntimeCoursePlayer; server ~116–146 of course_session.go).

- [ ] **Retain dirty state until save succeeds:** `apiSessionAdapter` must NOT clear `pending` before the save resolves; on rejection, keep/merge the dirty snapshot and retry with bounded backoff. The debounce path must handle errors (no bare `void flush()`).
- [ ] **Flush on lifecycle exits:** `RuntimeCoursePlayer` keeps the `flush` surface (don't spread it away into the narrower `SessionAdapter`) and flushes on `pagehide`/`visibilitychange:hidden`/unmount; expose a save-status signal (saving/saved/error) for a later UI.
- [ ] **Server-side validation:** `course_session.go` PUT validates the snapshot is a well-formed `CourseSession` at the border (status enum, required top-level fields present, JSON object) before storing — reject malformed with 400 instead of best-effort status extraction. (Deep contract validation stays client-side; the definition content-hash/revision guard is Slice 8 — out of scope here.)
- [ ] Tests: a rejected save keeps the snapshot dirty and a later flush re-sends it; pagehide triggers a flush; the Go PUT rejects a non-object / missing-status body with 400 and accepts a valid one. Run `pnpm --filter web test -- apiSessionAdapter RuntimeCoursePlayer` + `typecheck`; `go test ./internal/api/ -run CourseSession -timeout 1800s` FOREGROUND.
- [ ] Commit.

---

## Final verification
- [ ] `pnpm --filter @mind-imprint/course-runtime --filter @mind-imprint/course-renderer --filter web -r test` + `-r typecheck` (course packages + web) green (except the known pre-existing `packages/contracts/test/library.test.ts` typecheck error — not ours).
- [ ] `cd apps/api && go build ./... && go test ./internal/api/ -run CourseSession -count=1 -timeout 1800s`.
