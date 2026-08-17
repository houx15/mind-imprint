# Slice 8 — Validation Completeness + Definition Revision

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development. `- [ ]` steps.

**Goal:** Make `validateCourseDefinition` actually guarantee "a course that validates plays to a valid completion" (close the workflow-validation gaps), add the missing contract-quality invariants, and add a definition-revision policy so an in-place course edit doesn't silently corrupt existing sessions. Fixes review **P2-01, P2-09, P2-08**.

**Requirements source:** `docs/2026-08-16-student-course-runtime-code-review.md` findings P2-01 (workflow validation gaps), P2-09 (contract-quality gaps), P2-08 (no session compat/version policy). **Product decision D5:** content-hash on stored definition + CourseSession; on load, hash mismatch → RESET the session (no migrate/fork), with a visible notice.

**Depends on:** Slices 1–7 (the runtime the validator promises to play; Slice 1's CourseSession).

## Global Constraints
- Determinism preserved (no Date.now/Math.random in the pure packages; the hash is a pure function of the definition bytes).
- Don't break the green suites (contract 56 / runtime 38 / renderer 201 / web 1132). The golden `coverage-course.json` + all existing fixtures MUST still validate (these changes tighten validation — do NOT reject currently-valid real courses; add fixtures proving they still pass).
- Every new rule is a deterministic CONTRACT invariant (validator), not a pedagogical preference — genuine "is this playable/well-formed" checks only. Where the review says "warn", make it a non-blocking issue (a distinct `layer`/severity), not a hard reject, so authoring-quality nudges don't break valid courses.

---

### Task 1 — Workflow validation completeness (contract)

**Files:** `packages/course-contract/src/validate/workflow.ts` + `validate/index.ts`/`referential.ts` as needed; tests under `packages/course-contract/test/`.

Address the P2-01 gaps (each gets a POSITIVE fixture that passes + a NEGATIVE fixture that fails with a precise path):
- [ ] **Initial-state references:** validate `workflow.initialState.visibleBlockIds` / `enabledBlockIds` / `focusedTarget` reference real blocks in the slice (currently unchecked).
- [ ] **Event source / interaction / timer references:** a transition's `on.sourceId` must reference a block that can PRODUCE that event type (producer-aware — e.g. `answer.submitted` from an assessment block, `video.*` from a video block); `interactionId`/`timerId` matchers reference something real (a started timer id, a video interaction).
- [ ] **Matcher overlap (fix false positives + false negatives):** ambiguity detection currently compares only `type`+`sourceId`, ignoring `interactionId`/`timerId` — so it wrongly rejects valid disjoint cue transitions AND misses real overlaps. Model the full matcher intersection (type + sourceId + interactionId + timerId) so two transitions are ambiguous only if their matchers can actually both match the same event.
- [ ] **Terminal ordering / required-completion dominance:** a required completion (a graded block that must be completed) must not be bypassable before `navigate`. Prove that on every path to a `navigate`, the required completions dominate (are reached first). A step with `navigate` that can be reached before a required `completeSlice` fails.
- [ ] **Bounded cycles:** the cycle check treats some event labels as boundedness evidence but doesn't prove a finite attempt limit — a `submit-correct` remediation loop can be infinite. Require that any cycle has a bounded exit (e.g. an attempt cap / `submit-correct-or-exhausted`), else flag unbounded.
- [ ] Tests: every design §12.6 rule has a +/- fixture; the golden course + the existing branching fixtures PASS; workflows with unknown event sources, bypassed required completions, unbounded remediation, or navigate-before-completion FAIL with precise paths. `pnpm --filter @mind-imprint/course-contract test` + `typecheck` (+ run `--filter @mind-imprint/course-renderer test` to confirm no fixture the renderer relies on regressed).
- [ ] Commit.

---

### Task 2 — Contract quality checks + definition revision policy (contract + host)

**Files:** `packages/course-contract/src/{blocks.ts,course.ts,validate/referential.ts,session.ts}`, `packages/course-contract/src/assets.ts` (asset cap), a hash helper; `apps/api/internal/api/course_definition.go` (return a content hash), `apps/web/src/course/courseDefinition.ts` + `apps/web/src/shell/courses/RuntimeCoursePlayer.tsx` (compare/reset), `packages/course-renderer/src/course/CoursePlayer.tsx` (reset-on-mismatch); tests.

**P2-09 — contract quality (deterministic invariants; "warn" ones are non-blocking issues, not rejects):**
- [ ] Image item id + single-choice option id uniqueness (currently unchecked → ambiguous answer ids).
- [ ] `presentation:"single"` images block must have exactly ONE item (the renderer silently shows only the first — reject the mismatch OR make the renderer show all; per review, don't silently discard — validate to exactly one for `single`).
- [ ] Non-empty text where a real invariant (e.g. `text` block content — the review flags empty text may render nothing; make it a warn or a min-length per the design).
- [ ] Enforce the server's 256-asset cap as a contract check (`collectAssetPaths(doc).length <= 256`) so a course that would exceed the serving API is caught at validation.
- [ ] (warn) `estimatedMinutes` vs the sum of slice `estimatedSeconds` (flag a large mismatch, non-blocking).
- [ ] (warn) objective `evidenceBlockIds` should point at blocks that produce evidence (assessment/interactive), not static text/image/pdf (non-blocking).

**P2-08 / D5 — definition revision:**
- [ ] **Content hash:** a pure `courseDefinitionHash(documentBytes|document)` (stable, e.g. sha256 of canonical JSON) in `course-contract`. Add `courseDefinitionHash: string` to the `CourseSession` schema (`session.ts`).
- [ ] **Server returns the hash:** `GET /courses/{slug}/definition` returns `{ definition, hash }` (sha256 of the stored `course_definition` bytes) — small Go addition.
- [ ] **Reset on mismatch:** when a session is loaded/resumed, if `session.courseDefinitionHash` ≠ the current definition's hash → treat as a FRESH session (do NOT restore stale slice/step/block state that may reference removed ids), and stamp the new hash on the fresh session; show a brief visible "课程已更新，进度已重置" notice. A matching hash restores normally (Slice 1). A new session stamps the current hash on create.
- [ ] Tests: contract quality +/- fixtures (dup ids, single-with-many, over-256-assets, warns); `courseDefinitionHash` is stable + differs when the definition changes; a session whose hash ≠ current def hash resets to fresh (renderer test); a matching hash resumes; the Go handler returns a stable hash for the same bytes. `pnpm --filter @mind-imprint/course-contract --filter @mind-imprint/course-renderer --filter web -r test` + `-r typecheck`; `cd apps/api && go build ./... && go test ./internal/api/ -run CourseDefinition -timeout 1800s` FOREGROUND.
- [ ] Commit.

---

## Final verification
- [ ] `pnpm --filter @mind-imprint/course-contract --filter @mind-imprint/course-runtime --filter @mind-imprint/course-renderer --filter web -r test` + `-r typecheck` green; `cd apps/api && go build ./... && go test ./internal/api/ -run 'CourseDefinition|CourseSession' -timeout 1800s`.
- [ ] The golden `coverage-course.json` still validates (positive fixture); a course exceeding an invariant fails with a precise path.
