# Slice 4 — NavigationDefinition + Focus

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development. `- [ ]` steps.

**Goal:** Implement `NavigationDefinition` (previous / manualNext / autoNext / revisit) with course-owned nav controls, make completion state the gating authority, add an accessible `student.continue` producer, and make workflow `focus` a real accessible focus (DOM focus + scroll + focus ring). Fixes review **P1-05, P2-06**.

**Requirements source:** `docs/2026-08-16-student-course-runtime-code-review.md` findings P1-05 (NavigationDefinition unimplemented) + P2-06 (focus is metadata-only). Contract: `packages/course-contract/src/navigation.ts` — `{ previous:"allowed", manualNext:"after-completion"|"allowed", autoNext:boolean, revisit:"restore-completed-state" }`.

**Depends on:** Slice 1 (persistence/resume — revisit reads restored slice state), Slice 2 (Closing lifecycle).

## Global Constraints
- Determinism preserved (no Date.now/Math.random in pure packages).
- Don't break the green suites (renderer 134 / web 1131 / runtime 38 / contract 56).
- `completeSlice` (workflow effect) must be SEPARATE from the navigate decision — completion state gates navigation; a slice that completes without an explicit `navigate` must still be advanceable (autoNext or manual Next), not stuck.
- Nav controls are owned by `CoursePlayer`/`SlicePlayer` (renderer), styled with existing tokens; the full course stylesheet is Slice 5.

---

### Task 1 — Navigation controls + policy (course-renderer)

**Files:** `packages/course-renderer/src/course/CoursePlayer.tsx`, `packages/course-renderer/src/slice/SlicePlayer.tsx`, maybe a new `packages/course-renderer/src/course/CourseNav.tsx`; tests under `packages/course-renderer/test/`.

- [ ] **Read `slice.navigation`** (no renderer reads it today). Drive a course nav bar from it + the current slice's completion state.
- [ ] **Previous (`previous:"allowed"`):** a "上一步" control that navigates back to an already-REACHED slice (index < current max reached). Revisiting restores that slice's completed/answer/cue state (`revisit:"restore-completed-state"` — reuse Slice 1's `restoreState`/`restoreStepId` from the persisted sliceState). Not available before the first slice.
- [ ] **Manual next (`manualNext`):** a "下一步" control. `after-completion` → disabled until the current slice's completion state is met (the completion authority, NOT requiring a workflow `navigate` effect); `allowed` → always enabled. On click → advance.
- [ ] **Auto next (`autoNext:true`):** when the slice reaches completion, advance automatically (no manual click needed). When `false`, wait for manual Next. Separate `completeSlice` (marks complete) from the advance decision.
- [ ] **`student.continue` producer (P1-05):** an accessible visible control that emits the `student.continue` workflow event (some workflows transition on it). Wire it so a slice whose workflow waits on `student.continue` has a real producer.
- [ ] **Revisit (`revisit:"restore-completed-state"`):** navigating back to a completed slice shows it in its completed state and offers an explicit replay path (re-run the workflow from initial) rather than being frozen.
- [ ] Tests: previous navigates to a reached slice + restores its state; manualNext `after-completion` is disabled until completion then advances; `allowed` always advances; autoNext advances on completion without a click; a workflow waiting on `student.continue` advances when the producer is used; revisit restores completed state + replay works. `pnpm --filter @mind-imprint/course-renderer test` + `typecheck`.
- [ ] Commit.

---

### Task 2 — Real accessible focus (course-renderer)

**Files:** `packages/course-renderer/src/focus/FocusManager.tsx`, a minimal focus-ring style (inline or a small style block — the full stylesheet is Slice 5); tests under `packages/course-renderer/test/`.

- [ ] **Real focus (P2-06):** `FocusTarget` currently only sets `data-focused` + a class and never calls DOM focus. When the workflow `focus` action targets a block/item, MOVE keyboard focus to it (`element.focus()` — make the target focusable via `tabIndex={-1}` when needed), scroll it into view (`scrollIntoView({block:"nearest"})`, respecting `prefers-reduced-motion`), and ensure a screen reader announces the change (e.g. focus move + an accessible name). `clearFocus` removes it.
- [ ] **Focus ring:** define a visible `.course-focus-ring` style so focused targets are clearly emphasized (minimal now; Slice 5 folds it into the course stylesheet).
- [ ] Tests: a `focus` action moves DOM focus to the target element (`document.activeElement`), scrolls it into view (spy on scrollIntoView), and applies the focus-ring class; `clearFocus` restores. Respect reduced-motion. `pnpm --filter @mind-imprint/course-renderer test` + `typecheck`.
- [ ] Commit.

---

## Final verification
- [ ] `pnpm --filter @mind-imprint/course-renderer --filter web -r test` + `-r typecheck` green (known pre-existing contracts typecheck error excepted).
- [ ] Logic check: a slice that completes without an explicit workflow `navigate` is still advanceable (autoNext or manual Next); previous/replay work; `focus` moves real DOM focus.
