# Slice 5 — Course Stylesheet / One-Screen Layout

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development. `- [ ]` steps.

**Goal:** Ship a renderer-owned course stylesheet with an explicit desktop one-Slice/one-screen contract: the 4 layout presets become real compositions (not bare grid tracks), the shell stops page-scrolling, hidden blocks reserve their box, slots contain overflow/media, and the focus ring is folded in. Fixes review **P1-06** (and folds the Slice-4 focus-ring/a11y nits from **P2-06**).

**Requirements source:** `docs/2026-08-16-student-course-runtime-code-review.md` finding P1-06 (one-screen layout not implemented) + the deferred Slice-4 focus nits. Design viewport contract: `docs/2026-08-15-student-course-runtime-data-and-renderer-design.md` §~1651–1674, §~1927–1937. **Product decision D6**: one Slice per desktop screen, supported matrix ≥1280×720 / ≥1440×900, renderer-owned stylesheet, shell `overflow:hidden` (per-slot `overflow:auto` only where a slot opts in).

**Depends on:** Slices 1–4 (the players/nav/focus that the stylesheet dresses).

**Scope note:** true VISUAL verification at the viewport matrix is a real-browser concern → Slice 10's Playwright journey. This slice ships the stylesheet + STRUCTURAL guarantees that ARE unit-testable in jsdom (applied classes, computed overflow/`minmax`/height rules present, hidden-block placeholder reserves layout, no page-level scroll container). The audio arbiter (P1-09) is DEFERRED to Slice 7 (HTML audio lands there).

## Global Constraints
- Determinism preserved (no Date.now/Math.random in pure packages).
- Don't break the green suites (renderer 148 / web ~1131 / runtime 38 / contract 56).
- The stylesheet is RENDERER-OWNED (shipped from `@mind-imprint/course-renderer`, imported by the host) — self-contained, no external CSS/CDN. Use the existing `--mk-*` design tokens where the host already defines them; otherwise define course-scoped CSS custom properties with sane fallbacks so the package renders standalone.
- Theme-aware where the host is (respect the existing token system); don't hardcode a single palette that breaks dark mode.

---

### Task 1 — Course stylesheet + layout composition + hidden-block stability (course-renderer)

**Files:** a new `packages/course-renderer/src/styles/course.css` (+ its import wiring, e.g. `src/index.ts` or a `CoursePlayer` import), `packages/course-renderer/src/layout/LayoutRenderer.tsx`, `packages/course-renderer/src/slice/SlicePlayer.tsx` (hidden-block placeholder), `packages/course-renderer/src/course/CoursePlayer.tsx` (shell classes), `packages/course-renderer/src/focus/FocusManager.tsx` (fold the ring + fix the id aria-label nit); tests under `packages/course-renderer/test/`.

- [ ] **Course shell / one-screen:** a shell class that fills the viewport height and does NOT page-scroll (`height:100%`/`100dvh` region, `overflow:hidden` at the shell). Slice header/narration allocation + the layout region get explicit height so a Slice is one screen; overflow is constrained to intentional per-slot viewers only.
- [ ] **Layout presets (P1-06):** `LayoutRenderer` currently sets only `display:grid` + tracks. Add real composition: slot gaps, `minmax(0, …)` tracks (so a slot can shrink), per-slot `overflow:auto` container (a slot's content scrolls WITHIN the slot, never the page), media containment (`max-width/height:100%`, `object-fit`) so video/pdf/image stay inside their slot. All 4 presets (full / split-horizontal / split-vertical / grid) get a coherent stylesheet.
- [ ] **Hidden-block stability (P1-06):** blocks currently use the HTML `hidden` attribute which removes the box → reveal/hide reflows the layout. Give a hidden block a placeholder that RESERVES its slot box (e.g. `visibility:hidden`/an occupying placeholder rather than `hidden`), so reveal/hide does not cause global reflow. Keep it out of the a11y tree while hidden.
- [ ] **Focus ring + a11y (fold Slice-4 P2-06 nits):** move the injected `.course-focus-ring` into the stylesheet; fix the Slice-4 review nit — don't put `tabIndex={-1}` + a machine-id `aria-label` on EVERY block wrapper; only the actively-focused target gets programmatic focusability + a meaningful accessible name (derive from block content/title where available, else omit rather than announce an internal id).
- [ ] Tests (jsdom-level, structural): the shell region is not a page-scroll container (its overflow class/style is hidden); each preset applies its grid/gap/minmax classes; a slot is an `overflow:auto` container; a hidden block reserves layout (placeholder present, not removed from box) and is aria-hidden; the focus ring class comes from the stylesheet; only the focused block carries the focus tabIndex/label. `pnpm --filter @mind-imprint/course-renderer test` + `typecheck`.
- [ ] Commit.

---

### Task 2 — Host shell stops page scrolling (host)

**Files:** `apps/web/src/shell/courses/RuntimeCoursePlayer.tsx`; tests under `apps/web/test/`.

- [ ] The host viewport currently sets `overflowY:"auto"` on the player region (review P1-06 evidence) → a Slice can become a scrolling page. Constrain the course shell so it does NOT page-scroll during a normal Slice; overflow is the renderer's per-slot concern. Keep the header/back-button chrome; the course region fills the remaining height with the renderer's one-screen contract.
- [ ] Tests: the RuntimeCoursePlayer course region no longer sets a page-level vertical scroll on the slice area (assert the container style/class). `pnpm --filter web test -- RuntimeCoursePlayer` + `typecheck`.
- [ ] Commit.

---

## Final verification
- [ ] `pnpm --filter @mind-imprint/course-renderer --filter web -r test` + `-r typecheck` green (known pre-existing contracts typecheck error excepted).
- [ ] Note in the slice report: full visual verification at the ≥1280×720 / ≥1440×900 matrix is Slice 10's browser journey; this slice ships the stylesheet + structural guarantees.
