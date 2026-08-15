# Course Runtime — Slice 4: Assessment Block Renderers

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the two assessment block renderers — `FillBlankRenderer` and `SingleChoiceRenderer` — to `packages/course-renderer`, with full graded / survey / reflection behavior, attempt counting, per-rule completion, configured feedback, and the standardized answer/completion Events that drive workflow branching.

**Architecture:** Each assessment renderer is a pure `BlockRenderer` selected by `type` from the registry (Slice 3). It owns input state, submission, deterministic graded evaluation, attempt counting, and completion, and reports outcomes ONLY as runtime Events through the injected `emit`. It never decides branching or remediation — the `WorkflowRuntime` (Slice 2) owns that; the renderer just emits `answer.submitted` / `answer.correct` / `answer.incorrect` / `answer.attemptsExhausted` / `block.completed` per §17.12. The **final failed attempt** under `submit-correct-or-exhausted` emits `answer.attemptsExhausted` (NOT `answer.incorrect`) — the Event producer resolves that ambiguity (§12.5).

**Tech Stack:** React 18, `@mind-imprint/course-contract` + `@mind-imprint/course-runtime`, Vitest + jsdom + RTL + user-event.

**Authoritative spec:** §9.6 (FillBlank), §9.7 (SingleChoice), §17.12 (Assessment Renderers), §12.5 (branching / final-attempt ambiguity).

## Global Constraints

- Consume the exact `BlockRendererProps<TBlock>` from Slice 3 (`src/blocks/types.ts`): `{ block, assetResolver, state, visible, enabled, focusedItemId?, emit }` where `emit(sourceId, type, payload?)`. The renderer passes its own `block.id` as `sourceId`.
- Register both renderers in `src/blocks/registry.ts`, replacing the `NotImplementedRenderer` entries for `fillBlank` and `singleChoice`.
- Deterministic grading only — no LLM. Reflection mode captures the answer and completes on `submit-any`; it does not grade at runtime (§9.6: rubrics don't authorize runtime AI grading in the first release).
- When `enabled` is false, inputs and the submit control are disabled (the workflow disables a block between attempts / after completion via `disable` effects).
- Hidden (`visible=false`) blocks keep their slot (render `hidden`/`aria-hidden`), consistent with the other renderers.
- Event payloads: `answer.submitted` payload `{ value }` (string for fill-blank, optionId for single-choice); `answer.correct`/`answer.incorrect` payload `{ value, attempt }`; `answer.attemptsExhausted` payload `{ value, attempt }`; `block.completed` payload `{}`.
- No `/dev/null` redirects; never `git add -A`; pnpm workspace test via `pnpm --filter @mind-imprint/course-renderer test`.

---

### Task 1: Completion-rule engine (shared, pure)

**Files:** Create `src/blocks/assessment/completion.ts`. Test `test/assessment/completion.test.ts`.

Extract the per-rule decision so both renderers share it and it's unit-testable without the DOM.

**Interfaces:**
- Produces: `evaluateSubmission(input): SubmissionOutcome` where
  - `input = { correct: boolean; attemptNumber: number; rule: FillBlankCompletionRule | SingleChoiceCompletionRule }` (attemptNumber is 1-based, the attempt just made).
  - `SubmissionOutcome = { events: ("answer.correct"|"answer.incorrect"|"answer.attemptsExhausted")[]; completed: boolean; locked: boolean }`.
  - Rules (§9.6):
    - `submit-any`: any submission → `completed:true`, `locked:true`. Emit `answer.correct` if `correct` else `answer.incorrect` (for graded); for survey/reflection there is no correctness — caller passes `correct` as `true`-equivalent? No: for non-graded, the caller emits only `answer.submitted` + `block.completed` and does NOT call this correctness path. Keep `evaluateSubmission` for graded/attempt-bearing flows; document that survey/reflection bypass it.
    - `submit-correct`: `correct` → completed+locked, emit `answer.correct`; else emit `answer.incorrect`, not completed, not locked (unlimited attempts).
    - `submit-correct-or-exhausted { maxAttempts }`: `correct` → completed+locked, `answer.correct`. Wrong AND `attemptNumber >= maxAttempts` → completed+locked, emit `answer.attemptsExhausted` (NOT incorrect). Wrong AND `attemptNumber < maxAttempts` → `answer.incorrect`, not completed, not locked.

- [ ] **Step 1: Failing test** — table-driven over the three rules × {correct, wrong-with-attempts-left, wrong-final-attempt}. Assert the exact event list, `completed`, `locked`. Key case: `submit-correct-or-exhausted maxAttempts:2`, wrong on attempt 2 → `["answer.attemptsExhausted"]`, completed, locked (no `answer.incorrect`).
- [ ] **Step 2: Run → FAIL.** — [ ] **Step 3: Implement.** — [ ] **Step 4: Run → PASS.** — [ ] **Step 5: Commit** — `feat(course-renderer): assessment completion-rule engine`.

---

### Task 2: FillBlankRenderer

**Files:** Create `src/blocks/assessment/FillBlankRenderer.tsx`. Test `test/assessment/fillBlank.test.tsx`. Register `fillBlank`.

**Spec:** §9.6, §17.12.

Behavior:
- Renders `prompt`, a text input (`placeholder` if present), and a submit button. `aria-label` the input by the prompt.
- On submit: emit `answer.submitted { value }` first. Then:
  - **graded**: compare `value` against `acceptedAnswers` (trim; case-insensitive unless `caseSensitive`); compute `correct`; call `evaluateSubmission`; emit its events; show `correctFeedback`/`incorrectFeedback` accordingly; on `completed` emit `block.completed` and lock the input.
  - **reflection**: no grading; emit `answer.submitted { value }` then (rule is `submit-any`) `block.completed`; show a neutral acknowledgement; lock.
- Attempt counter is local (1-based), seeded from `state.attempts ?? 0`.
- When `enabled` is false OR locked, input + button disabled.

- [ ] **Step 1: Failing tests** (RTL + user-event, `emit` = `vi.fn()`):
  - graded `submit-correct-or-exhausted maxAttempts:2`: type a wrong answer → emits `answer.submitted` then `answer.incorrect`; shows incorrectFeedback; input still enabled. Type wrong again (attempt 2) → emits `answer.attemptsExhausted` then `block.completed`; input locked.
  - graded correct on attempt 1 → `answer.submitted`, `answer.correct`, `block.completed`; shows correctFeedback; locked.
  - case-insensitive match by default; case-sensitive rejects a case-mismatch.
  - reflection: submit any text → `answer.submitted` + `block.completed`, no correctness events.
  - `enabled=false` → button disabled, no emit on click.
- [ ] **Step 2: Run → FAIL.** — [ ] **Step 3: Implement.** — [ ] **Step 4: Run → PASS.** — [ ] **Step 5: Commit** — `feat(course-renderer): fill-blank renderer`.

---

### Task 3: SingleChoiceRenderer

**Files:** Create `src/blocks/assessment/SingleChoiceRenderer.tsx`. Test `test/assessment/singleChoice.test.tsx`. Register `singleChoice`.

**Spec:** §9.7, §17.12.

Behavior:
- Renders `prompt` + a radio group of `options` (accessible: `role="radiogroup"`, each option a labelled radio). Submit button.
- On submit with a selected option: emit `answer.submitted { value: optionId }`. Then:
  - **graded**: `correct = optionId === correctOptionId`; `evaluateSubmission`; emit events; show `correctFeedback`/`incorrectFeedback`; on completed → `block.completed` + lock.
  - **survey**: no correctness; emit `answer.submitted { value }` then `block.completed`; lock. (`submit-any`.)
- Submit disabled until an option is selected; whole control disabled when `enabled=false` or locked.

- [ ] **Step 1: Failing tests** — mirror Task 2 for choices: graded correct → correct events + completed; graded `submit-correct-or-exhausted maxAttempts:3` wrong twice then correct → incorrect, incorrect, then correct+completed; survey → submitted + completed only; can't submit without a selection; `enabled=false` disables.
- [ ] **Step 2: Run → FAIL.** — [ ] **Step 3: Implement.** — [ ] **Step 4: Run → PASS.** — [ ] **Step 5: Commit** — `feat(course-renderer): single-choice renderer`.

---

### Task 4: Branching integration test (assessment drives the workflow)

**Files:** Test `test/assessment/branching.test.tsx`; extend `test/support/staticCourse.ts` (from Slice 3) with an assessment slice mirroring the §15 golden workflow (`wait-for-answer` → `answer.correct`→summarize / `answer.incorrect`→remediate→back / `answer.attemptsExhausted`→summarize).

**Spec:** §12.5 + §15 golden workflow. Proves the renderer's Events actually drive `WorkflowRuntime` branches through `SlicePlayer` (Slice 3).

- [ ] **Step 1: Failing test** — mount `SlicePlayer` with the assessment slice. Answer wrong once → assert the remediation narration path is entered (remediation block shown / focus moved) and the question is re-enabled. Answer correct → assert the slice reaches `completeSlice` (`onSliceComplete` fires). Separately: exhaust attempts → assert it routes to summarize/complete, not an infinite loop.
- [ ] **Step 2: Run → FAIL.** — [ ] **Step 3:** Fix any wiring gaps (e.g. the renderer must read `state.attempts` so a re-enabled question continues its attempt count; SlicePlayer must pass updated `state` after `applyEvent` increments attempts). — [ ] **Step 4: Run → PASS.** Run FULL package suite + typecheck. — [ ] **Step 5: Commit** — `feat(course-renderer): assessment-driven workflow branching`.

---

## Self-Review Notes

- **Ambiguity ownership:** the completion engine is the single place that decides `answer.incorrect` vs `answer.attemptsExhausted` for the final failed attempt — matching §12.5's "the Event producer decides." Both renderers delegate to it, so the two block types can never diverge.
- **Attempt continuity across remediation:** when the workflow disables → re-enables a question for another try, the attempt count must persist. It lives in `BlockSessionState.attempts` (Slice 2 `applyEvent` increments on `answer.submitted`); the renderer seeds its local counter from `state.attempts`. Task 4 guards this.
- **No runtime AI grading:** reflection/survey capture only; deterministic grading for graded mode. Rubric-based evaluation stays an authoring/review concern (§9.6).
