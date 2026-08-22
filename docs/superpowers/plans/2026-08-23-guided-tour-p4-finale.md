# Guided Tour — P4 (Full Journey + Accent Finale + Polish) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Finish the guided tour — the welcome modal's 课程/项目 choice reorders the journey (courses-first vs projects-first), a settings/accent finale segment closes the tour, and two small engine-polish items from the P2/P3 reviews are cleaned up.

**Architecture:** Reuse the P1 engine + the courses (P1) and projects (P3) segments. Add `journeyStarting(start)` to compose the ordered journey + a `settings-accent` finale segment (reuses the existing `[data-testid="accent-swatch"]` in SettingsView). Wire the welcome modal's pick to play the correctly-ordered journey. Fix the TourRunner stale-spotlight (clear the previous rect before resolving the next anchor).

**Spec:** `docs/superpowers/specs/2026-08-22-new-user-guided-tour-design.md` (§4.1 welcome branching, §7 settings finale, §8 P4).

## Global Constraints

- Reuse the P1 engine; no new dependency; tour config frontend-local in `apps/web/src/tour`.
- mk alpha trap: solid `bg-mk-*` / alpha on real colors only.
- Frontend tests: `npx vitest run <path>`; full `npm run test`; `npm run typecheck`.
- Git: stage specific files (never `git add -A`); untracked `docs/*.md` are not ours; commit per task with trailer `Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>`; controller pushes.
- Boundary hook blocks `/dev/null` redirects (use `2>&1`) + `cd` in compound commands.

---

## Task 1: Journey branching + settings/accent finale

**Files:**
- Create: `apps/web/src/tour/segments/settings.ts` (the `settings-accent` finale segment)
- Modify: `apps/web/src/tour/journey.ts` (`journeyStarting(start)` + `fullJourney`)
- Modify: `apps/web/src/shell/StudentApp.tsx` (welcome `onPick` plays `journeyStarting(start)`)
- Test: `apps/web/test/tour/journeyOrdering.test.ts`, extend `apps/web/test/shell/StudentApp.tour.test.tsx`

**Interfaces:**
- Produces: `journeyStarting(start: "courses"|"projects"): TourJourney`; `settingsSegment: TourSegment`; `fullJourney` = courses-first + finale (default for the nav-footer restart).

- [ ] **Step 1: Author the settings finale segment**

Create `apps/web/src/tour/segments/settings.ts`:
```ts
import type { TourSegment } from "../types";

export const settingsSegment: TourSegment = {
  id: "settings-accent",
  name: "个性化",
  steps: [
    {
      id: "settings-accent-0",
      onEnter: (nav) => nav.setTab("me"),
      placement: "center",
      title: "最后一件小事",
      text: "这些都逛完啦！在「我」这里，你可以把整个界面换成你喜欢的主题色。",
      advance: "next",
    },
    {
      id: "settings-accent-1",
      anchor: '[data-testid="accent-swatch"]',
      placement: "top",
      text: "挑一个你喜欢的颜色，界面会立刻跟着变。祝你在思维印记玩得开心 —— 有需要随时叫我。",
      advance: "next",
    },
  ],
};
```
(`setTab("me")` renders `SettingsView`, which has the `data-testid="accent-swatch"` swatches — confirmed in the P1 exploration. `querySelector` resolves the first swatch.)

- [ ] **Step 2: Failing ordering test**

Create `apps/web/test/tour/journeyOrdering.test.ts`:
```ts
import { describe, it, expect } from "vitest";
import { journeyStarting } from "@/tour/journey";
import { coursesSegments } from "@/tour/segments/courses";
import { projectsSegments } from "@/tour/segments/projects";
import { settingsSegment } from "@/tour/segments/settings";

describe("journeyStarting", () => {
  it("courses-first: courses, then projects, then settings finale", () => {
    const j = journeyStarting("courses");
    expect(j[0].id).toBe(coursesSegments[0].id);
    expect(j.some((s) => s.id === projectsSegments[0].id)).toBe(true);
    expect(j[j.length - 1].id).toBe(settingsSegment.id);
  });
  it("projects-first: projects, then courses, then settings finale", () => {
    const j = journeyStarting("projects");
    expect(j[0].id).toBe(projectsSegments[0].id);
    expect(j[j.length - 1].id).toBe(settingsSegment.id);
    // courses group present after projects
    const ci = j.findIndex((s) => s.id === coursesSegments[0].id);
    const pi = j.findIndex((s) => s.id === projectsSegments[0].id);
    expect(pi).toBeLessThan(ci);
  });
});
```

- [ ] **Step 3: Run to verify it fails**

Run: `cd apps/web && npx vitest run test/tour/journeyOrdering.test.ts`
Expected: FAIL — `journeyStarting` not exported.

- [ ] **Step 4: Implement `journeyStarting`**

In `apps/web/src/tour/journey.ts`:
```ts
import type { TourJourney } from "./types";
import { coursesSegments } from "./segments/courses";
import { projectsSegments } from "./segments/projects";
import { settingsSegment } from "./segments/settings";

export function journeyStarting(start: "courses" | "projects"): TourJourney {
  const groups = start === "projects"
    ? [...projectsSegments, ...coursesSegments]
    : [...coursesSegments, ...projectsSegments];
  return [...groups, settingsSegment];
}

export const coursesJourney: TourJourney = coursesSegments;
export const fullJourney: TourJourney = journeyStarting("courses"); // nav-footer restart default
```

- [ ] **Step 5: Wire the welcome pick + run**

In `StudentApp.tsx`, the welcome modal's `onPick(start)` (currently `tour.play(fullJourney)`) → `tour.play(journeyStarting(start))` (import `journeyStarting`). Keep the nav-footer restart on `fullJourney`. Extend `apps/web/test/shell/StudentApp.tour.test.tsx`: picking 项目 plays a journey whose first segment is the projects group (assert via the mock — e.g. the first onEnter opens the demo project, or the first navigated tab is projects). Keep existing cases.

Run: `cd apps/web && npx vitest run test/tour/journeyOrdering.test.ts test/shell/StudentApp.tour.test.tsx && npm run typecheck`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add apps/web/src/tour/segments/settings.ts apps/web/src/tour/journey.ts apps/web/src/shell/StudentApp.tsx apps/web/test/tour/journeyOrdering.test.ts apps/web/test/shell/StudentApp.tour.test.tsx
git commit -m "feat(tour): courses/projects-first branching + settings/accent finale" -m "Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 2: TourRunner stale-spotlight polish

Clear the previous spotlight rect before resolving the next anchor, so moving from an anchored step to a centered/absent-anchor step doesn't leave the old spotlight hole visible for the ~2s poll (P3 review M1).

**Files:**
- Modify: `apps/web/src/tour/TourRunner.tsx`
- Test: extend `apps/web/test/tour/TourRunner.test.tsx`

- [ ] **Step 1: Failing test**

Extend `TourRunner.test.tsx`: a segment `[{anchored step targeting a present element}, {center step}]` — after advancing from the anchored step to the center step, assert the spotlight cutout is gone (the popover is centered). (In jsdom, assert that on the center step the runner is in its centered branch — e.g. no element has the spotlight cutout style / the bubble is centered. Use whatever observable the existing tests use for centered vs anchored.) If hard to assert precisely in jsdom, assert that `rect` is reset — e.g. by checking the popover no longer carries the anchored-position style. Keep it minimal.

- [ ] **Step 2: Run to verify it fails (or is flaky on the stale rect)**

Run: `cd apps/web && npx vitest run test/tour/TourRunner.test.tsx`

- [ ] **Step 3: Fix the effect**

In `TourRunner.tsx`, the anchor-resolution effect: set `setRect(null)` at the START of the effect (before resolving), so the previous rect never lingers while the next anchor resolves. For a center step (`!anchor || placement === "center"`), it already sets null immediately. For an anchored step, clear first, then `resolveAnchor(...).then(el => setRect(el ? el.getBoundingClientRect() : null))`. This removes the stale-spotlight window.

- [ ] **Step 4: Run to verify + full suite + typecheck**

Run: `cd apps/web && npx vitest run test/tour/TourRunner.test.tsx && npm run test && npm run typecheck`
Expected: PASS, full suite green.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/tour/TourRunner.tsx apps/web/test/tour/TourRunner.test.tsx
git commit -m "fix(tour): clear spotlight before resolving next anchor (no stale hole)" -m "Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Self-Review (author's pass)

- **Spec coverage:** welcome courses/projects-first branching (§4.1) ✓; settings/accent finale (§7) ✓; P4 polish (§8) — M1 stale-spotlight ✓.
- **Deferred (documented, not in P4):** driving the reading list-view for `reading-library` (centered honestly instead — acceptable); opening a live source in the reading-room section (narration density); backward-nav from the eval report (M2, forward-only tour is the norm); seeding a finished-looking `studio_state.openTool`. These are polish beyond the finale and don't block ship.
- **Type consistency:** `journeyStarting` returns `TourJourney`; `settingsSegment` is a `TourSegment`; welcome `onPick` type `"courses"|"projects"` matches.
- **No live LLM; read-only demo unaffected.**
