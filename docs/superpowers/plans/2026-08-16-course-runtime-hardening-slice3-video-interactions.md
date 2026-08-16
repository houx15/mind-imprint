# Slice 3 — Video Interaction Loading + Cue Modal + Typed Result

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development. `- [ ]` steps.

**Goal:** In the PRODUCTION host, a video's referenced interaction document is actually loaded, its cues auto-pause at their timestamps, appear as an accessible **modal**, record a **typed result** (fixing the Slice-1 hollow-`result` deferral), resume per policy, and gate video completion — so the golden course's `video-ended-and-interactions-completed` no longer deadlocks. Fixes review **P1-01, P1-02, P2-03**.

**Requirements source:** `docs/2026-08-16-student-course-runtime-code-review.md` findings P1-01 (interactions not wired), P1-02 (inline not modal, loses evidence), P2-03 (video control/reset/disabled/optional-cue). **Product decision D1** (program design doc): cues render as an accessible **modal dialog** (portal + backdrop + focus trap + focus restore); required cues block dismissal until complete; optional cues (`required:false`) get an explicit "跳过" (skip) control.

**Depends on:** Slice 1 (typed payloads — `video.interaction.completed {interactionId, result}` reducer already exists and populates `interactionResult`; this slice makes the producer send a real `result`). Slice 2 (audio fail-safe patterns).

## Global Constraints
- Determinism preserved (no Date.now/Math.random in pure packages; timers/positions via injected engine/clock).
- Don't break the green suites (course-renderer 123 / web 1124 / runtime 38 / contract 56).
- The renderer stays pure/host-injectable: it DEFINES the async loader interface it needs; the HOST implements the network fetch. No network in the renderer package.
- A missing/malformed/mismatched/expired interaction asset must produce a **visible recoverable error**, never a silent `null` or a deadlock (the current failure).

---

### Task 1 — Async interaction loader boundary + cue modal + typed result + video control (course-renderer)

**Files:** `packages/course-renderer/src/blocks/media/VideoInteractionController.tsx`, `packages/course-renderer/src/blocks/media/VideoRenderer.tsx`, `packages/course-renderer/src/media/videoEngine.ts`, `packages/course-renderer/src/index.ts` (export the loader API), maybe a new `packages/course-renderer/src/blocks/media/InteractionModal.tsx`; tests under `packages/course-renderer/test/media/`.

- [ ] **Async loader boundary (P1-01):** replace the current synchronous `InteractionLoader`/context (review lines 19–24, returns `null` at 58–66 when no loader injected) with an **async resource boundary**: define an interface like `InteractionLoader = (source: string) => Promise<VideoInteractionDocument>` (or a preloaded `Record<source, VideoInteractionDocument>` map) surfaced via a provider that exposes `{loading | ready(doc) | error(kind)}` states. Export the provider + interface from `src/index.ts` (currently NOT exported — review line 60). `VideoRenderer` (review 108–116) mounts the controller for an interaction ref and supplies the loader; while loading show a spinner, on error show a recoverable error, on ready run the cue timeline.
- [ ] **Cue modal (P1-02, D1):** the cue activity renders as an accessible **dialog** (portal, backdrop, `role="dialog"`/`aria-modal`, focus trap, focus restore on close) — not the current bare `div role="group"` (review 130–135). Required cue: no dismiss until completed. Optional cue (`required:false`): an explicit "跳过" control that resumes without completing.
- [ ] **Typed cue result (P1-02, closes Slice-1 deferral):** the controller currently discards inner answer events (review 124–128) and emits only `{interactionId}` (review 113–119). Capture the inner activity's answer/correctness and emit `video.interaction.completed` with a **validated `{interactionId, result: InteractionResult}`** (the `InteractionResult` type Slice 1 defined: `{correct?, value?}`). The Slice-1 reducer already stores it into `interactionResult` — verify the round-trip now carries real evidence.
- [ ] **Video control completeness (P2-03):** unify native `<video>` play/pause into the event stream (observe the element's play/pause, not just the custom buttons — review 391–395); real `reset` clears the controller's fired/completed sets + required gate (review 393); honor `enabled` (VideoRenderer ignores it — review 391); surface play-promise rejection (`videoEngine.ts` 29–35 ignores it) with a learner-recoverable affordance.
- [ ] Tests: production-style mount (no test-only provider — inject the async loader) loads a doc and runs a required cue that auto-pauses/shows-modal/records-result/resumes/gates completion; missing/malformed/mismatched/expired doc → visible error, no deadlock; optional cue skippable; reset restores cue state; disabled video is non-interactive; modal has dialog semantics + focus trap. `pnpm --filter @mind-imprint/course-renderer test` + `typecheck`.
- [ ] Commit.

---

### Task 2 — Production interaction-document adapter (host)

**Files:** a new `apps/web/src/course/interactionLoader.ts`, `apps/web/src/shell/courses/RuntimeCoursePlayer.tsx`; `apps/web/src/api/*` if a fetch helper is needed; tests under `apps/web/test/`.

**Interface from Task 1:** the exported async loader interface the renderer expects.

- [ ] **Production adapter (P1-01):** implement the loader — given an interaction `source` (a relative asset path), resolve its signed CDN URL (via the same asset-url map `RuntimeCoursePlayer` already holds), `fetch` the JSON, parse with `VideoInteractionDocument` (course-contract), run `validateVideoInteraction`, and cache by source. Expose typed error states: `not-found` (fetch 404/expired-403), `malformed` (JSON/parse fail), `invalid` (validateVideoInteraction issues), `mismatch` (doc's `video.source` ≠ the block's source, per the contract's referential rule). Never throw raw — return the typed error the renderer surfaces.
- [ ] **Wire into `RuntimeCoursePlayer`:** provide the loader to the `CoursePlayer`/renderer via the exported provider. The interaction JSON is one of the course's assets (`collectAssetPaths` already includes `video.interaction.source`), so its signed URL is in the asset map — reuse it; handle URL refresh (a re-signed URL should be usable on a retry).
- [ ] Tests: the adapter returns a parsed+validated doc for a good asset; returns the typed error (not a throw) for 404 / malformed JSON / failing validation / video-source mismatch; caches by source (second call doesn't refetch). `pnpm --filter web test -- interactionLoader RuntimeCoursePlayer` + `typecheck`.
- [ ] Commit.

---

## Final verification
- [ ] `pnpm --filter @mind-imprint/course-renderer --filter web -r test` + `-r typecheck` green (known pre-existing contracts typecheck error excepted).
- [ ] Logic check against the golden course (`coverage-course.json`): its `video-ended-and-interactions-completed` block + external interaction doc now loads + gates completion in the production host (no deadlock).
