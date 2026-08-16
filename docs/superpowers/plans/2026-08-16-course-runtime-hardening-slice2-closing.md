# Slice 2 — Closing Lifecycle + Scene Audio + Personalization

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development. `- [ ]` steps.

**Goal:** Students actually SEE/HEAR the Closing scene before the route changes; the session transitions `in-progress → closing → completed` in that order; Opening/Closing scene audio plays (with a learner-start fallback when autoplay is blocked); Opening renders its objectives; the real-time Opening/Closing are personalized from real signals/evidence (not `{}`). Fixes review **P1-03, P2-04, P2-05**.

**Requirements source:** `docs/2026-08-16-student-course-runtime-code-review.md` findings P1-03 (Closing skipped), P2-04 (audio + scene audio incomplete), P2-05 (personalization placeholders). Program: `docs/superpowers/specs/2026-08-16-course-runtime-hardening-program-design.md`.

**Depends on Slice 1** (merged): typed payloads, persistence, resume; `CoursePlayer` already restores completed→closing.

## Global Constraints
- Determinism preserved (no Date.now/Math.random in pure packages).
- Don't break Slice 1's green suites (runtime 38 / renderer 101 / web 1122 / Go).
- Audio playback lives behind the existing injectable `AudioEngine` seam (`narration/audioEngine.ts`) — keep it host-injectable/testable; a play-promise REJECTION must surface, never hang a workflow waiting on `narration.ended`.
- The single-audible-source arbiter is Slice 5 — here, just make scene/narration audio PLAY + fail-safe; full ducking/priority is Slice 5.

---

### Task 1 — Closing as an explicit lifecycle phase + `onComplete` + scene audio (course-renderer)

**Files:** `packages/course-renderer/src/course/CoursePlayer.tsx`, `packages/course-renderer/src/scenes/OpeningScene.tsx` + `ClosingScene.tsx`, `packages/course-renderer/src/narration/NarrationPlayer.tsx` (target-id semantics), `packages/course-renderer/src/narration/audioEngine.ts` (play-rejection surface); tests under `packages/course-renderer/test/`.

- [ ] **Closing lifecycle (P1-03):** currently `CoursePlayer` saves Closing → `setStatus("completed")` → then setClosing/enter closing (so the production `onFinish`-on-completed wrapper unmounts before the learner sees Closing). Reorder so the session goes **`in-progress → closing` (render+play the Closing scene) → and only AFTER the Closing completion policy is met → `completed`**. Add a dedicated **`onComplete?: () => void`** prop to `CoursePlayerProps` that fires when the Closing completes — do NOT rely on the host wrapping `setStatus("completed")` to infer UI completion.
- [ ] **Closing completion policy:** the learner dismisses/finishes the Closing (an explicit accessible "完成课程" control, or scene-audio-ended + a continue) → then `setStatus("completed")` + `onComplete()`. Define it simply and test it.
- [ ] **Scene audio (P2-04):** `OpeningScene`/`ClosingScene` currently render text only and ignore `RuntimeSceneResult.audioUrl`. Play the scene audio through the injected `AudioEngine` (resolve the audioUrl via the assetResolver if relative). If `audio.play()` rejects (autoplay blocked), show a visible one-click "播放" fallback and DO NOT block the phase.
- [ ] **Opening objectives (P2-05 partial):** `OpeningScene` declares `objectives` but doesn't render them — render the approved opening facts (learningPreview + objectives) consistently.
- [ ] **Narration target semantics (P2-04):** `SlicePlayer`/`NarrationPlayer` `pauseNarration`/`stopNarration` currently act on whichever track is active — honor the narration ID in the action so a stale/other track isn't stopped. `audioEngine` exposes play status/error so a `narration.ended`-gated workflow can't wait forever on a blocked play.
- [ ] Tests: the Closing scene is rendered/visible and `onComplete` fires ONLY after the Closing is dismissed (not on entering completed); scene audioUrl is played through the injected engine; a rejected play shows the fallback and doesn't hang; Opening renders objectives; pauseNarration(id) doesn't stop a different id. `pnpm --filter @mind-imprint/course-renderer test` + `typecheck`.
- [ ] Commit.

---

### Task 2 — Host uses `onComplete` + real personalization inputs (host)

**Files:** `apps/web/src/shell/courses/RuntimeCoursePlayer.tsx`, `apps/web/src/course/apiSceneGenerator.ts`, and the Opening/Closing scene-generation input wiring (`packages/course-runtime` `RuntimeSceneGenerator` input types if a field is missing); tests under `apps/web/test/`.

- [ ] **Use `onComplete` (P1-03):** `RuntimeCoursePlayer` currently converts `setStatus("completed")` into `onFinish()` by wrapping the session adapter. Replace with the new `CoursePlayer.onComplete` prop → `onFinish()`. The `setStatus` wrapper's completion-inference is removed. (Keep persisting status via the normal adapter path.)
- [ ] **Opening `signalValues` (P2-05):** `CoursePlayer` passes `signalValues: {}` to the opening generator. Resolve the **allowed** history signals (`recent-course-topics`, `prior-objective-performance`) at the authenticated host/API boundary and feed them in. If no server endpoint exists yet, wire the seam + pass what's available (e.g. from the validated session / a host-provided callback) and leave a typed extension point; do not fabricate.
- [ ] **Closing `sessionEvidence` (P2-05):** `CoursePlayer` passes `sessionEvidence: {}` to the closing generator. Derive the allowed evidence from the **validated CourseSession** (answers, attempts, time-on-slice, interaction results — the fields Slice 1 now persists) and feed the closing generator, so the Closing reflects what happened.
- [ ] Tests: `RuntimeCoursePlayer` fires `onFinish` via `onComplete` (not the setStatus wrapper); the closing generator receives non-empty evidence derived from a session with recorded answers/results; opening receives the resolved signals. `pnpm --filter web test -- RuntimeCoursePlayer apiSceneGenerator` + `typecheck`.
- [ ] Commit.

---

## Final verification
- [ ] `pnpm --filter @mind-imprint/course-renderer --filter web -r test` + `-r typecheck` green (course-renderer + web; known pre-existing `packages/contracts/test/library.test.ts` typecheck error excepted).
- [ ] Manual/logic check: Closing is visible before the parent route changes; `in-progress → closing → completed` order holds; scene audio plays or offers a fallback.
