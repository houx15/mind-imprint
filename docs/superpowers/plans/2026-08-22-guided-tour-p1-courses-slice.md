# Guided Tour — P1 (Courses Slice) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship a real, working new-user onboarding for the **courses** surface: a custom
config-driven tour engine, a first-run welcome modal, a nav-rail footer to re-trigger it, a
server-persisted `onboarded_at` flag, the Courses group of tour segments, and a frozen
example course report — with no database seed.

**Architecture:** A frontend-local custom engine under `apps/web/src/tour/`: a `TourProvider`
context (mirrors `ui/accent.tsx`) drives a declarative `Journey → Segment → Step` config; a
`TourRunner` paints a dim overlay + spotlight cutout + 印记 popover and advances on 下一步 or a
real user action; steps navigate the app through a `TourNavContext` assembled in `StudentApp`
and anchor onto `[data-tour="…"]` attributes. First-run is keyed on a new nullable
`users.onboarded_at` column exposed on `/me` (mirrors the accent/background preference pattern).

**Tech Stack:** React 18 + Vite + TypeScript + Tailwind (`mk-*` tokens); Go `net/http` +
`pgx`/`sqlc` v1.27.0 + `goose`; vitest + @testing-library/react; Go testcontainers.

**Spec:** `docs/superpowers/specs/2026-08-22-new-user-guided-tour-design.md`

## Global Constraints

- **No live LLM during onboarding** — every AI-flavored thing the tour shows is frozen/mock fixture data.
- **Non-blocking** — every step is skippable; a global 跳过本节 / 结束 is always present. (UX choice, not 铁律②.)
- **Custom engine, no new dependency** — do not add driver.js/react-joyride/etc.
- **Tour config types are frontend-local** — live in `apps/web/src/tour`, never `packages/contracts` (backend never reads tour config).
- **Anchors use `[data-tour="<id>"]`** — missing anchors degrade gracefully (centered bubble), never hard-stop the tour.
- **Courses slice needs NO database seed** — the example report is a frozen fixture; 学习记录/图鉴 are taught honestly on their real (empty) states.
- **mk-* alpha trap** — `bg-mk-accent/NN` emits NO CSS (mk tokens are bare CSS vars). Use solid `bg-mk-accent`, inline `linear-gradient`/`color-mix`, or alpha on REAL colors (`bg-white/15`, `text-white/70`). Verify color in a real browser.
- **sqlc** — regen with `cd apps/api && make sqlc` (`CGO_ENABLED=0 go tool sqlc generate`), pin v1.27.0; a nullable `timestamptz` maps to `pgtype.Timestamptz` (not `time.Time`).
- **Go tests** — `cd apps/api && go test ./... -timeout 1800s` with Docker running (testcontainers, foreground). Scoped dev run: `cd apps/api && make sqlc && go build ./... && go test ./internal/api/ -run <Name> -timeout 900s`. Run FULL packages for the final check.
- **Frontend tests** — single file `npx vitest run test/<path>.test.tsx`; full `npm run test`; types `npm run typecheck`. Test files live at `apps/web/test/**` mirroring `src/**`.
- **Git** — stage specific files (never `git add -A`); commit per task; push to `main` (rebase if rejected). Commit trailer: `Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>`.

---

## File Structure

**New (engine + tour):**
- `apps/web/src/tour/types.ts` — `TourStep`, `TourSegment`, `TourJourney`, `TourNavContext`, `TourController`.
- `apps/web/src/tour/anchors.ts` — `resolveAnchor(selector, opts)`.
- `apps/web/src/tour/TourProvider.tsx` — context provider + state machine + `useTour`.
- `apps/web/src/tour/TourRunner.tsx` — overlay + spotlight + 印记 popover + keyboard/action handling.
- `apps/web/src/tour/WelcomeModal.tsx` — first-run greeting + 课程/项目/稍后 branch.
- `apps/web/src/tour/segments/courses.ts` — the Courses group segments.
- `apps/web/src/tour/journey.ts` — journey composition (`coursesJourney`, `fullJourney` placeholder for P1).
- `apps/web/src/tour/fixtures/exampleCourseReport.ts` — frozen example `CourseReport` + card info.
- Tests under `apps/web/test/tour/**`.

**Modified (frontend):**
- `apps/web/src/api/auth.ts`, `apps/web/src/api/index.ts` — `MeUser.onboarded_at`, `putOnboarding()`.
- `apps/web/src/shell/StudentApp.tsx` — mount `TourProvider`, assemble `TourNavContext`, welcome trigger, thread nav-footer props, `pendingCoursesSub`.
- `apps/web/src/shell/Nav.tsx` — footer (重新开始引导 / 反馈 / 退出登录) + `data-tour`.
- `apps/web/src/shell/CoursesTab.tsx` — `pendingSub` one-shot; `data-tour` wrapper on the Segmented.
- `apps/web/src/shell/courses/CoursesView.tsx` — `data-tour` on category chips + course grid.
- `apps/web/src/shell/courses/CoursesContainer.tsx` — allow tour to open a course; (player root reuses existing `data-testid="course-region"`).
- `apps/web/src/shell/courses/CourseReport.tsx` — `exampleReport?` prop (short-circuit fetch) + section `data-tour`.
- `apps/web/src/shell/courses/LearningHistory.tsx`, `apps/web/src/shell/growth/ToolkitCards.tsx`, `apps/web/src/shell/growth/CardDetailModal.tsx` — `data-tour` anchors on empty states / grids / tabs.
- `apps/web/src/shell/growth/CardDetailModal.tsx` — export `CardMdInline` OR (chosen) route bubble text through already-exported `@/cards/Markdown` (no change needed there).

**Modified/new (backend):**
- `apps/api/internal/store/migrations/0079_user_onboarded_at.sql` (new).
- `apps/api/internal/store/queries/users.sql` — `SetUserOnboardedAt`.
- `apps/api/internal/api/users_onboarding.go` (new) + `apps/api/internal/api/users_onboarding_test.go` (new).
- `apps/api/internal/api/api.go` — route registration.
- `apps/api/internal/api/dto.go` — `meUserDTO.OnboardedAt` + `buildMeUser`.
- Feedback (Task 10, droppable): `0080_feedback.sql`, `queries/feedback.sql`, `api/feedback.go` (+ test), `api.go` route; `apps/web/src/api/*` client; feedback modal.

**Anchor id vocabulary** (added as `data-tour` in Task 5, referenced by segments in Task 8):
`courses-subswitcher`, `courses-categories`, `courses-grid`, `course-report`,
`course-report-stats`, `course-report-quiz`, `course-report-cards`, `courses-history`,
`tujian-filters`, `tujian-grid`, `card-detail-tabs`. In-player reuses existing hooks:
`[data-testid="course-region"]`, `.course-nav__next`, `[data-testid="ask-bubble"]`.

---

## Task 1: `onboarded_at` round-trip (backend + TS client)

Adds the first-run flag end-to-end: a nullable column, a `now()`-stamp writer, exposure on
`/me`, and the TS client. Mirrors the accent/background preference pattern exactly.

**Files:**
- Create: `apps/api/internal/store/migrations/0079_user_onboarded_at.sql`
- Modify: `apps/api/internal/store/queries/users.sql` (add `SetUserOnboardedAt`)
- Regenerate: `apps/api/internal/store/sqlc/users.sql.go`, `models.go` (via `make sqlc`)
- Create: `apps/api/internal/api/users_onboarding.go`
- Modify: `apps/api/internal/api/api.go:81` (route), `apps/api/internal/api/dto.go:51-90` (DTO + builder)
- Test: `apps/api/internal/api/users_onboarding_test.go`
- Modify: `apps/web/src/api/auth.ts:5-14,53-67`, `apps/web/src/api/index.ts:2,36-37,99`

**Interfaces:**
- Produces (Go): `PUT /api/v1/users/me/onboarding` (protected) → `{"ok": true}`, stamps `users.onboarded_at = now()`; `/me` `user.onboarded_at` = RFC3339Nano string or `null`.
- Produces (TS): `MeUser.onboarded_at: string | null`; `api.putOnboarding(): Promise<void>`.

- [ ] **Step 1: Write the migration**

Create `apps/api/internal/store/migrations/0079_user_onboarded_at.sql`:

```sql
-- +goose Up
-- When the student finished (or dismissed) the new-user guided tour. NULL = never
-- onboarded → the welcome modal fires. Follows the users.email_verified_at precedent
-- (a nullable timestamptz on the users row).
ALTER TABLE users ADD COLUMN onboarded_at timestamptz;

-- +goose Down
ALTER TABLE users DROP COLUMN onboarded_at;
```

- [ ] **Step 2: Add the sqlc writer query**

Append to `apps/api/internal/store/queries/users.sql`:

```sql
-- name: SetUserOnboardedAt :exec
-- Stamps the moment the student completed/dismissed onboarding. Idempotent enough for
-- our use (re-running just refreshes the timestamp).
UPDATE users SET onboarded_at = now() WHERE id = @user_id;
```

- [ ] **Step 3: Regenerate sqlc and confirm the model picked up the column**

Run: `cd apps/api && make sqlc && go build ./...`
Expected: build succeeds; `internal/store/sqlc/models.go` `User` struct now has
`OnboardedAt pgtype.Timestamptz`; `users.sql.go` has a `SetUserOnboardedAt(ctx, userID uuid.UUID) error`.

- [ ] **Step 4: Write the failing handler test**

Create `apps/api/internal/api/users_onboarding_test.go` (mirror `users_accent_test.go`):

```go
package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/<module>/apps/api/internal/store/sqlc"
	api "github.com/<module>/apps/api/internal/api"
)

func TestPutUserOnboarding(t *testing.T) {
	pool := newAPITestPool(t)
	h := newTestAPI(pool).Handler()
	cookie := signInSeed(t, pool)

	// Unauthenticated → 401.
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("PUT", "/api/v1/users/me/onboarding", nil))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("unauth: want 401, got %d", rr.Code)
	}

	// Before: onboarded_at is NULL.
	q := sqlc.New(pool)
	before, err := q.GetUserByID(t.Context(), api.SeedUserID)
	if err != nil {
		t.Fatal(err)
	}
	if before.OnboardedAt.Valid {
		t.Fatalf("seed user should start un-onboarded")
	}

	// Authenticated → 200 and stamps the column.
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("PUT", "/api/v1/users/me/onboarding", nil), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("stamp: want 200, got %d (%s)", rr.Code, rr.Body.String())
	}

	after, err := q.GetUserByID(t.Context(), api.SeedUserID)
	if err != nil {
		t.Fatal(err)
	}
	if !after.OnboardedAt.Valid {
		t.Fatalf("onboarded_at should be set after PUT")
	}
}
```

Note: replace `github.com/<module>` with the real module path (check `apps/api/go.mod`; the
existing test files' imports show it — copy from `users_accent_test.go`).

- [ ] **Step 5: Run the test to verify it fails**

Run: `cd apps/api && go test ./internal/api/ -run TestPutUserOnboarding -timeout 900s`
Expected: FAIL/compile error — `putUserOnboarding` route not registered / handler missing.

- [ ] **Step 6: Write the handler**

Create `apps/api/internal/api/users_onboarding.go` (mirror `users_background.go`, no body/allowlist):

```go
package api

import (
	"net/http"

	"github.com/<module>/apps/api/internal/httpx"
)

// putUserOnboarding stamps users.onboarded_at = now(), marking that the student has
// finished or dismissed the new-user guided tour. No request body.
func (a *API) putUserOnboarding(w http.ResponseWriter, r *http.Request) {
	u, ok := UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, r, httpx.ErrUnauthorized("未登录"))
		return
	}
	if err := a.d.Queries.SetUserOnboardedAt(r.Context(), u.ID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}
```

(Copy the exact `httpx` import path from `users_background.go`.)

- [ ] **Step 7: Register the route**

In `apps/api/internal/api/api.go`, next to the background route (~line 81), add:

```go
mux.Handle("PUT /api/v1/users/me/onboarding", protected(a.putUserOnboarding))
```

- [ ] **Step 8: Expose `onboarded_at` on `/me`**

In `apps/api/internal/api/dto.go`, add to `meUserDTO` (after `PageBackground`):

```go
	OnboardedAt    *string      `json:"onboarded_at"`
```

In `buildMeUser`, after constructing the DTO fields, set it from the nullable column
(`tsLayout` = RFC3339Nano is already defined in dto.go:25):

```go
	var onboardedAt *string
	if full.OnboardedAt.Valid {
		s := full.OnboardedAt.Time.Format(tsLayout)
		onboardedAt = &s
	}
```

…and include `OnboardedAt: onboardedAt,` in the returned `meUserDTO{...}` literal.

- [ ] **Step 9: Run the test to verify it passes**

Run: `cd apps/api && go build ./... && go test ./internal/api/ -run TestPutUserOnboarding -timeout 900s`
Expected: PASS.

- [ ] **Step 10: Add the TS client**

In `apps/web/src/api/auth.ts`, add to `MeUser` (after `page_background`):

```ts
  onboarded_at: string | null;
```

…and add the writer beside `setBackground`:

```ts
// Stamp that the student finished/dismissed onboarding. No body; pure flag write.
export async function putOnboarding(): Promise<void> {
  await apiFetch<{ ok: boolean }>("/api/v1/users/me/onboarding", { method: "PUT" });
}
```

In `apps/web/src/api/index.ts`: add `putOnboarding` to the import from `./auth` (line 2), to
the `ApiClient` interface (near line 36-37: `putOnboarding(): Promise<void>;`), and to the `api`
object literal (line 99).

- [ ] **Step 11: Typecheck the frontend**

Run: `cd apps/web && npm run typecheck`
Expected: PASS (no missing-field errors; any test constructing a `MeUser` will be updated in later tasks).

- [ ] **Step 12: Commit**

```bash
git add apps/api/internal/store/migrations/0079_user_onboarded_at.sql apps/api/internal/store/queries/users.sql apps/api/internal/store/sqlc/ apps/api/internal/api/users_onboarding.go apps/api/internal/api/users_onboarding_test.go apps/api/internal/api/api.go apps/api/internal/api/dto.go apps/web/src/api/auth.ts apps/web/src/api/index.ts
git commit -m "feat(tour): onboarded_at first-run flag round-trip (api + client)"
```

---

## Task 2: Tour engine core — types, anchor resolver, provider, `useTour`

The declarative config types, a resilient anchor resolver, and the state-machine provider.

**Files:**
- Create: `apps/web/src/tour/types.ts`, `apps/web/src/tour/anchors.ts`, `apps/web/src/tour/TourProvider.tsx`
- Test: `apps/web/test/tour/anchors.test.ts`, `apps/web/test/tour/TourProvider.test.tsx`

**Interfaces:**
- Produces: the types below; `resolveAnchor(selector, opts?) => Promise<HTMLElement|null>`;
  `<TourProvider nav={TourNavContext} onComplete?={() => void}>`; `useTour(): TourController`.

- [ ] **Step 1: Write the types**

Create `apps/web/src/tour/types.ts`:

```ts
export type TourPlacement = "top" | "bottom" | "left" | "right" | "center";
export type NavTabKey = "home" | "projects" | "courses" | "me";
export type CoursesSub = "courses" | "history" | "gallery";

/** Setters the tour uses to drive the app. Assembled in StudentApp (§Task 9).
 *  P1 only needs the courses-side setters; P3 extends this. */
export interface TourNavContext {
  setTab: (t: NavTabKey) => void;
  openCourse: (slug: string) => void;
  setCoursesSub: (s: CoursesSub) => void;
}

export interface TourStep {
  id: string;
  /** Drive app state before the step renders (navigate, open a course, switch sub-tab). */
  onEnter?: (ctx: TourNavContext) => void | Promise<void>;
  /** CSS selector, convention [data-tour="<id>"]. Absent → centered bubble. */
  anchor?: string;
  /** Cut a hole over the anchor (default true when anchor set). */
  spotlight?: boolean;
  /** Bubble placement relative to the anchor; "center" = modal-like, ignores anchor. */
  placement?: TourPlacement;
  title?: string;
  /** 印记's line, rendered as Markdown. */
  text: string;
  /** "next" = advance on the 下一步 button; "action" = advance when the user does the thing. */
  advance: "next" | "action";
  /** For advance:"action": which element + event advances the step. */
  actionEvent?: { selector: string; type: "click" | "input" };
}

export interface TourSegment {
  id: string;
  name: string;
  steps: TourStep[];
}

export type TourJourney = TourSegment[];

export interface TourController {
  running: boolean;
  segment: TourSegment | null;
  step: TourStep | null;
  segmentIndex: number;
  stepIndex: number;
  /** 0-1 progress within the whole active journey. */
  progress: number;
  play: (target: TourSegment | TourJourney) => void;
  next: () => void;
  prev: () => void;
  skipSegment: () => void;
  stop: () => void;
}
```

- [ ] **Step 2: Write the failing anchor-resolver test**

Create `apps/web/test/tour/anchors.test.ts`:

```ts
import { describe, it, expect, vi, afterEach } from "vitest";
import { resolveAnchor } from "@/tour/anchors";

afterEach(() => { document.body.innerHTML = ""; vi.useRealTimers(); });

describe("resolveAnchor", () => {
  it("resolves immediately when the element is already present", async () => {
    const el = document.createElement("div");
    el.setAttribute("data-tour", "x");
    document.body.appendChild(el);
    await expect(resolveAnchor('[data-tour="x"]')).resolves.toBe(el);
  });

  it("resolves null after the timeout when the element never appears", async () => {
    vi.useFakeTimers();
    const p = resolveAnchor('[data-tour="missing"]', { timeoutMs: 200, intervalMs: 50 });
    await vi.advanceTimersByTimeAsync(250);
    await expect(p).resolves.toBeNull();
  });
});
```

- [ ] **Step 3: Run to verify it fails**

Run: `cd apps/web && npx vitest run test/tour/anchors.test.ts`
Expected: FAIL — `@/tour/anchors` not found.

- [ ] **Step 4: Implement the anchor resolver**

Create `apps/web/src/tour/anchors.ts`:

```ts
export interface ResolveOpts { timeoutMs?: number; intervalMs?: number; }

/** Resolve a tour anchor selector, polling briefly since the element may mount after
 *  navigation. Resolves null on timeout — callers degrade gracefully (never throw). */
export function resolveAnchor(selector: string, opts: ResolveOpts = {}): Promise<HTMLElement | null> {
  const { timeoutMs = 2000, intervalMs = 50 } = opts;
  const immediate = document.querySelector<HTMLElement>(selector);
  if (immediate) return Promise.resolve(immediate);
  return new Promise((resolve) => {
    let elapsed = 0;
    const timer = setInterval(() => {
      const el = document.querySelector<HTMLElement>(selector);
      if (el) { clearInterval(timer); resolve(el); return; }
      elapsed += intervalMs;
      if (elapsed >= timeoutMs) { clearInterval(timer); resolve(null); }
    }, intervalMs);
  });
}
```

- [ ] **Step 5: Run to verify it passes**

Run: `cd apps/web && npx vitest run test/tour/anchors.test.ts`
Expected: PASS.

- [ ] **Step 6: Write the failing provider test**

Create `apps/web/test/tour/TourProvider.test.tsx`:

```tsx
import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { TourProvider, useTour } from "@/tour/TourProvider";
import type { TourNavContext, TourSegment } from "@/tour/types";

const nav: TourNavContext = { setTab: vi.fn(), openCourse: vi.fn(), setCoursesSub: vi.fn() };

const seg = (id: string, n: number): TourSegment => ({
  id, name: id,
  steps: Array.from({ length: n }, (_, i) => ({ id: `${id}-${i}`, text: `${id} ${i}`, advance: "next" as const })),
});

function Probe() {
  const t = useTour();
  return (
    <div>
      <span data-testid="state">{t.running ? `${t.segment?.id}:${t.stepIndex}` : "idle"}</span>
      <button onClick={() => t.play([seg("a", 2), seg("b", 1)])}>play</button>
      <button onClick={t.next}>next</button>
      <button onClick={t.prev}>prev</button>
      <button onClick={t.skipSegment}>skip</button>
      <button onClick={t.stop}>stop</button>
    </div>
  );
}

describe("TourProvider", () => {
  it("plays, advances across segments, and completes", () => {
    const onComplete = vi.fn();
    render(<TourProvider nav={nav} onComplete={onComplete}><Probe /></TourProvider>);
    expect(screen.getByTestId("state").textContent).toBe("idle");

    fireEvent.click(screen.getByText("play"));
    expect(screen.getByTestId("state").textContent).toBe("a:0");
    fireEvent.click(screen.getByText("next"));  // a:1
    expect(screen.getByTestId("state").textContent).toBe("a:1");
    fireEvent.click(screen.getByText("next"));  // rolls into segment b
    expect(screen.getByTestId("state").textContent).toBe("b:0");
    fireEvent.click(screen.getByText("next"));  // past last step → complete
    expect(screen.getByTestId("state").textContent).toBe("idle");
    expect(onComplete).toHaveBeenCalledTimes(1);
  });

  it("calls onEnter with the nav context when a step becomes active", () => {
    const onEnter = vi.fn();
    const s: TourSegment = { id: "s", name: "s", steps: [{ id: "s0", text: "hi", advance: "next", onEnter }] };
    function P() { const t = useTour(); return <button onClick={() => t.play(s)}>go</button>; }
    render(<TourProvider nav={nav}><P /></TourProvider>);
    fireEvent.click(screen.getByText("go"));
    expect(onEnter).toHaveBeenCalledWith(nav);
  });

  it("stop() ends the tour and fires onComplete once", () => {
    const onComplete = vi.fn();
    render(<TourProvider nav={nav} onComplete={onComplete}><Probe /></TourProvider>);
    fireEvent.click(screen.getByText("play"));
    fireEvent.click(screen.getByText("stop"));
    expect(screen.getByTestId("state").textContent).toBe("idle");
    expect(onComplete).toHaveBeenCalledTimes(1);
  });
});
```

- [ ] **Step 7: Run to verify it fails**

Run: `cd apps/web && npx vitest run test/tour/TourProvider.test.tsx`
Expected: FAIL — `@/tour/TourProvider` not found.

- [ ] **Step 8: Implement the provider**

Create `apps/web/src/tour/TourProvider.tsx`:

```tsx
import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import type { TourController, TourJourney, TourNavContext, TourSegment } from "./types";

const noop = () => {};
const TourContext = createContext<TourController>({
  running: false, segment: null, step: null, segmentIndex: 0, stepIndex: 0, progress: 0,
  play: noop, next: noop, prev: noop, skipSegment: noop, stop: noop,
});

function asJourney(target: TourSegment | TourJourney): TourJourney {
  return Array.isArray(target) ? target : [target];
}

export function TourProvider({ nav, onComplete, children }: {
  nav: TourNavContext;
  onComplete?: () => void;
  children: ReactNode;
}) {
  const [journey, setJourney] = useState<TourJourney | null>(null);
  const [segIdx, setSegIdx] = useState(0);
  const [stepIdx, setStepIdx] = useState(0);
  // Keep the latest nav/onComplete without re-subscribing effects.
  const navRef = useRef(nav); navRef.current = nav;
  const doneRef = useRef(onComplete); doneRef.current = onComplete;

  const running = journey != null;
  const segment = running ? journey![segIdx] ?? null : null;
  const step = segment ? segment.steps[stepIdx] ?? null : null;

  const finish = useCallback(() => {
    setJourney(null); setSegIdx(0); setStepIdx(0);
    doneRef.current?.();
  }, []);

  const play = useCallback((target: TourSegment | TourJourney) => {
    setJourney(asJourney(target)); setSegIdx(0); setStepIdx(0);
  }, []);

  const next = useCallback(() => {
    setJourney((j) => {
      if (!j) return j;
      setSegIdx((si) => {
        setStepIdx((sti) => {
          const seg = j[si];
          if (!seg) return sti;
          if (sti + 1 < seg.steps.length) return sti + 1;   // next step
          if (si + 1 < j.length) { queueMicrotask(() => { setSegIdx(si + 1); setStepIdx(0); }); return sti; }
          queueMicrotask(finish);                             // past the end
          return sti;
        });
        return si;
      });
      return j;
    });
  }, [finish]);
```

The nested-setter form above is fragile; implement `next`/`prev`/`skipSegment` with a single
reducer-style computation instead:

```tsx
  const advanceTo = useCallback((nextSeg: number, nextStep: number) => {
    setSegIdx(nextSeg); setStepIdx(nextStep);
  }, []);

  const nextStable = useCallback(() => {
    if (!journey) return;
    const seg = journey[segIdx];
    if (!seg) return;
    if (stepIdx + 1 < seg.steps.length) { advanceTo(segIdx, stepIdx + 1); return; }
    if (segIdx + 1 < journey.length) { advanceTo(segIdx + 1, 0); return; }
    finish();
  }, [journey, segIdx, stepIdx, advanceTo, finish]);

  const prevStable = useCallback(() => {
    if (!journey) return;
    if (stepIdx > 0) { advanceTo(segIdx, stepIdx - 1); return; }
    if (segIdx > 0) { const ps = journey[segIdx - 1]; advanceTo(segIdx - 1, Math.max(0, ps.steps.length - 1)); }
  }, [journey, segIdx, stepIdx, advanceTo]);

  const skipSegment = useCallback(() => {
    if (!journey) return;
    if (segIdx + 1 < journey.length) advanceTo(segIdx + 1, 0);
    else finish();
  }, [journey, segIdx, advanceTo, finish]);

  // onEnter fires whenever the active step changes while running.
  useEffect(() => {
    if (!running || !step) return;
    void step.onEnter?.(navRef.current);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [running, segIdx, stepIdx]);

  const progress = useMemo(() => {
    if (!journey) return 0;
    const total = journey.reduce((n, s) => n + s.steps.length, 0);
    const done = journey.slice(0, segIdx).reduce((n, s) => n + s.steps.length, 0) + stepIdx;
    return total ? done / total : 0;
  }, [journey, segIdx, stepIdx]);

  const value: TourController = {
    running, segment, step, segmentIndex: segIdx, stepIndex: stepIdx, progress,
    play, next: nextStable, prev: prevStable, skipSegment, stop: finish,
  };
  return <TourContext.Provider value={value}>{children}</TourContext.Provider>;
}

export const useTour = (): TourController => useContext(TourContext);
```

**Delete the first fragile `next` draft — keep only `nextStable`/`prevStable`/`skipSegment`.**
(The draft is shown only to explain why the reducer-style form is used.)

- [ ] **Step 9: Run to verify it passes**

Run: `cd apps/web && npx vitest run test/tour/TourProvider.test.tsx test/tour/anchors.test.ts`
Expected: PASS (all cases).

- [ ] **Step 10: Commit**

```bash
git add apps/web/src/tour/types.ts apps/web/src/tour/anchors.ts apps/web/src/tour/TourProvider.tsx apps/web/test/tour/anchors.test.ts apps/web/test/tour/TourProvider.test.tsx
git commit -m "feat(tour): engine core — types, anchor resolver, provider state machine"
```

---

## Task 3: TourRunner — overlay, spotlight, 印记 popover, keyboard + action advance

Renders when the tour is running: dims the page, cuts a spotlight over the anchor, shows the
印记 bubble with controls, advances on 下一步 or (for `advance:"action"`) a real user event.

**Files:**
- Create: `apps/web/src/tour/TourRunner.tsx`
- Test: `apps/web/test/tour/TourRunner.test.tsx`

**Interfaces:**
- Consumes: `useTour()` (Task 2), `resolveAnchor` (Task 2), `Pebble` (`@/ui`), `Markdown` (`@/cards/Markdown`).
- Produces: `<TourRunner />` — rendered once inside `TourProvider` (added in Task 9).

- [ ] **Step 1: Write the failing test**

Create `apps/web/test/tour/TourRunner.test.tsx`:

```tsx
import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { TourProvider, useTour } from "@/tour/TourProvider";
import { TourRunner } from "@/tour/TourRunner";
import type { TourNavContext, TourSegment } from "@/tour/types";

const nav: TourNavContext = { setTab: vi.fn(), openCourse: vi.fn(), setCoursesSub: vi.fn() };

function Harness({ seg }: { seg: TourSegment }) {
  const t = useTour();
  return <button onClick={() => t.play(seg)}>play</button>;
}

function renderTour(seg: TourSegment) {
  return render(
    <TourProvider nav={nav}>
      <Harness seg={seg} />
      <TourRunner />
    </TourProvider>,
  );
}

describe("TourRunner", () => {
  it("renders the 印记 bubble text and advances on 下一步", () => {
    const seg: TourSegment = { id: "s", name: "s", steps: [
      { id: "s0", text: "第一步说明", advance: "next", placement: "center" },
      { id: "s1", text: "第二步说明", advance: "next", placement: "center" },
    ]};
    renderTour(seg);
    fireEvent.click(screen.getByText("play"));
    expect(screen.getByText("第一步说明")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "下一步" }));
    expect(screen.getByText("第二步说明")).toBeInTheDocument();
  });

  it("ends when 结束 is clicked", () => {
    const seg: TourSegment = { id: "s", name: "s", steps: [{ id: "s0", text: "内容", advance: "next", placement: "center" }] };
    renderTour(seg);
    fireEvent.click(screen.getByText("play"));
    fireEvent.click(screen.getByRole("button", { name: "结束" }));
    expect(screen.queryByText("内容")).not.toBeInTheDocument();
  });

  it("advance:action shows a hint and advances when the target is clicked", () => {
    const seg: TourSegment = { id: "s", name: "s", steps: [
      { id: "s0", text: "点它", advance: "action", actionEvent: { selector: "#target", type: "click" }, placement: "center" },
      { id: "s1", text: "完成", advance: "next", placement: "center" },
    ]};
    renderTour(seg);
    // an out-of-tour element to click
    const target = document.createElement("button"); target.id = "target"; document.body.appendChild(target);
    fireEvent.click(screen.getByText("play"));
    expect(screen.getByText("点它")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "下一步" })).not.toBeInTheDocument(); // action steps have no 下一步
    fireEvent.click(target);
    expect(screen.getByText("完成")).toBeInTheDocument();
    document.body.removeChild(target);
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd apps/web && npx vitest run test/tour/TourRunner.test.tsx`
Expected: FAIL — `@/tour/TourRunner` not found.

- [ ] **Step 3: Implement the runner**

Create `apps/web/src/tour/TourRunner.tsx`:

```tsx
import { useEffect, useState } from "react";
import { createPortal } from "react-dom";
import { Pebble } from "@/ui";
import { Markdown } from "@/cards/Markdown";
import { useTour } from "./TourProvider";
import { resolveAnchor } from "./anchors";

const PAD = 8; // spotlight padding around the anchor

export function TourRunner() {
  const t = useTour();
  const [rect, setRect] = useState<DOMRect | null>(null);

  // Resolve + spotlight the anchor whenever the step changes.
  useEffect(() => {
    if (!t.running || !t.step) { setRect(null); return; }
    if (!t.step.anchor || t.step.placement === "center") { setRect(null); return; }
    let cancelled = false;
    void resolveAnchor(t.step.anchor).then((el) => {
      if (cancelled) return;
      if (!el) { setRect(null); return; }
      el.scrollIntoView({ block: "center", behavior: "smooth" });
      setRect(el.getBoundingClientRect());
    });
    return () => { cancelled = true; };
  }, [t.running, t.step]);

  // advance:"action" — advance when the user does the thing on the target element.
  useEffect(() => {
    if (!t.running || !t.step || t.step.advance !== "action" || !t.step.actionEvent) return;
    const { selector, type } = t.step.actionEvent;
    let cleanup = () => {};
    void resolveAnchor(selector).then((el) => {
      if (!el) return;
      const handler = () => t.next();
      el.addEventListener(type, handler, { once: true });
      cleanup = () => el.removeEventListener(type, handler);
    });
    return () => cleanup();
  }, [t.running, t.step, t]);

  // Keyboard: Esc ends; Enter/→ advances a "next" step.
  useEffect(() => {
    if (!t.running) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") { e.preventDefault(); t.stop(); }
      else if ((e.key === "Enter" || e.key === "ArrowRight") && t.step?.advance === "next") { e.preventDefault(); t.next(); }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [t]);

  if (!t.running || !t.step) return null;

  const isAction = t.step.advance === "action";
  const centered = !rect;

  return createPortal(
    <div className="fixed inset-0 z-[100]" role="dialog" aria-modal="true" aria-label="新手引导">
      {/* Dim overlay. With a rect, a box-shadow cutout creates the spotlight. */}
      <div
        className="absolute inset-0"
        style={
          rect
            ? {
                top: rect.top - PAD, left: rect.left - PAD,
                width: rect.width + PAD * 2, height: rect.height + PAD * 2,
                position: "fixed", borderRadius: 12,
                boxShadow: "0 0 0 9999px rgba(15,23,42,0.55)",
                transition: "all 160ms ease",
              }
            : { background: "rgba(15,23,42,0.55)" }
        }
      />
      {/* 印记 popover. Centered when no anchor; otherwise near the anchor. */}
      <div
        className="fixed max-w-[360px] rounded-mk-md bg-mk-surface p-4 shadow-mk-lg ring-1 ring-mk-border"
        style={
          centered
            ? { top: "50%", left: "50%", transform: "translate(-50%,-50%)" }
            : positionNear(rect!, t.step.placement)
        }
      >
        <div className="mb-2 flex items-center gap-2">
          <Pebble size={22} />
          <span className="text-mk-small font-semibold text-mk-accent-700">印记</span>
        </div>
        {t.step.title && <div className="mb-1 text-mk-body font-semibold text-mk-ink">{t.step.title}</div>}
        <div className="text-mk-body text-mk-ink"><Markdown text={t.step.text} /></div>

        <div className="mt-3 flex items-center justify-between">
          <button type="button" className="text-mk-small text-mk-muted hover:text-mk-ink" onClick={t.stop}>结束</button>
          <div className="flex items-center gap-2">
            <button type="button" className="text-mk-small text-mk-muted hover:text-mk-ink" onClick={t.skipSegment}>跳过本节</button>
            {t.stepIndex > 0 || t.segmentIndex > 0 ? (
              <button type="button" className="rounded-mk-full px-3 py-1 text-mk-small text-mk-ink ring-1 ring-mk-border hover:bg-mk-paper" onClick={t.prev}>上一步</button>
            ) : null}
            {isAction ? (
              <span className="rounded-mk-full bg-mk-accent-50 px-3 py-1 text-mk-small font-semibold text-mk-accent-700">试试看 →</span>
            ) : (
              <button type="button" className="rounded-mk-full bg-mk-accent px-3 py-1 text-mk-small font-semibold text-white hover:opacity-90" onClick={t.next}>下一步</button>
            )}
          </div>
        </div>
      </div>
    </div>,
    document.body,
  );
}

function positionNear(rect: DOMRect, placement: TourPlacementLocal = "bottom"): React.CSSProperties {
  const gap = 12;
  switch (placement) {
    case "top":    return { top: rect.top - gap, left: rect.left, transform: "translateY(-100%)" };
    case "left":   return { top: rect.top, left: rect.left - gap, transform: "translateX(-100%)" };
    case "right":  return { top: rect.top, left: rect.right + gap };
    case "bottom":
    default:       return { top: rect.bottom + gap, left: rect.left };
  }
}
type TourPlacementLocal = "top" | "bottom" | "left" | "right" | "center";
```

Notes for the implementer:
- The `bg-mk-accent` solid utility is safe; do NOT use `bg-mk-accent/NN` (mk alpha trap). The
  `rgba(15,23,42,…)` scrim is a real color, fine.
- In jsdom `getBoundingClientRect()` returns zeros, so tests use `placement:"center"` steps
  (the centered branch). Real positioning is verified in the browser (Step 5).

- [ ] **Step 4: Run to verify it passes**

Run: `cd apps/web && npx vitest run test/tour/TourRunner.test.tsx`
Expected: PASS.

- [ ] **Step 5: Typecheck**

Run: `cd apps/web && npm run typecheck`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add apps/web/src/tour/TourRunner.tsx apps/web/test/tour/TourRunner.test.tsx
git commit -m "feat(tour): TourRunner overlay + spotlight + 印记 popover"
```

---

## Task 4: CourseReport example mode + frozen fixture

Lets the tour show a well-crafted example report without a real attempt or a fetch.

**Files:**
- Create: `apps/web/src/tour/fixtures/exampleCourseReport.ts`
- Modify: `apps/web/src/shell/courses/CourseReport.tsx:200,210-239` (+ section `data-tour`, Task 5 adds those)
- Test: `apps/web/test/tour/exampleCourseReport.test.tsx`

**Interfaces:**
- Consumes: `CourseReport` type (`packages/contracts` → re-exported; imported in `CourseReport.tsx` as `CourseReportT`).
- Produces: `exampleCourseReport: CourseReportT`; `CourseReport` accepts `exampleReport?: CourseReportT` (skips all fetches when present).

- [ ] **Step 1: Author the frozen fixture (content-quality bar applies)**

Create `apps/web/src/tour/fixtures/exampleCourseReport.ts`. Use a real, substantial course
(not toy text). Shape from `packages/contracts/src/course.ts:163-171`:

```ts
import type { CourseReport } from "@mind-imprint/contracts";

/** A frozen, illustrative course report shown during onboarding — NOT a real attempt.
 *  Content is substantive on purpose (see spec §5.2 quality bar). */
export const exampleCourseReport: CourseReport = {
  title: "追问“可持续”：一堂关于概念澄清的思辨课",
  goal: "学会在下判断前，先把关键概念拆开、界定清楚，避免用模糊的大词代替真正的论证。",
  teaching_thread:
    "这门课带你走过一次完整的概念澄清：先识别一句话里被当作理所当然的大词，" +
    "再追问它到底指什么、由谁来衡量、在什么范围内成立，最后用一个反例检验你的界定是否站得住。",
  completedStepTitles: [
    "识别被含糊使用的关键概念",
    "为概念给出可操作的界定",
    "区分事实判断与价值判断",
    "用反例检验你的界定",
    "把澄清后的概念放回原来的问题",
  ],
  cardIds: ["concept-clarify", "steelman"],
  secondsSpent: 1140,
  quiz: { total: 5, correct: 4 },
};
```

Note: `cardIds` must be real card ids present in `packages/contracts/cards/` — verify two ids
that exist (grep the cards dir) and use those; the report renders `ToolCardDetail` per id and
looks them up in the cards catalog. If unsure, pick two ids you confirm exist.

- [ ] **Step 2: Write the failing test**

Create `apps/web/test/tour/exampleCourseReport.test.tsx`:

```tsx
import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";

// The report calls api.getCourseReport / getCardsCatalog / listCourses on mount; example mode
// must NOT. We assert the fixture renders and no fetch was attempted.
const getCourseReport = vi.fn();
vi.mock("@/api", () => ({
  api: {
    getCourseReport: (...a: unknown[]) => { getCourseReport(...a); return new Promise(() => {}); },
    getCardsCatalog: () => Promise.resolve([]),
    listCourses: () => Promise.resolve([]),
    getCourseAnswerReport: () => new Promise(() => {}),
  },
}));

import { CourseReport } from "@/shell/courses/CourseReport";
import { exampleCourseReport } from "@/tour/fixtures/exampleCourseReport";

describe("CourseReport example mode", () => {
  it("renders the fixture without fetching a real report", () => {
    render(
      <CourseReport
        courseId="__example__"
        exampleReport={exampleCourseReport}
        onBackToCourses={() => {}}
        onGoPortal={() => {}}
      />,
    );
    expect(screen.getByText(exampleCourseReport.title)).toBeInTheDocument();
    expect(getCourseReport).not.toHaveBeenCalled();
  });
});
```

- [ ] **Step 3: Run to verify it fails**

Run: `cd apps/web && npx vitest run test/tour/exampleCourseReport.test.tsx`
Expected: FAIL — `exampleReport` prop not accepted / fetch still called.

- [ ] **Step 4: Add the `exampleReport` prop + short-circuit**

In `apps/web/src/shell/courses/CourseReport.tsx`, extend the props destructure (line ~200):

```tsx
export function CourseReport({ courseId, attemptId, exampleReport, onBackToCourses, onGoPortal, onRestart }: {
  courseId: string; attemptId?: string; exampleReport?: CourseReportT;
  onBackToCourses: () => void; onGoPortal: () => void; onRestart?: () => void;
})
```

At the very top of the mount `useEffect` (line ~210), short-circuit before any `api.*` call:

```tsx
  useEffect(() => {
    if (exampleReport) { setReport(exampleReport); setError(false); return; }
    // ...existing fetch body unchanged...
  }, [courseId, attemptId, exampleReport]);
```

Also gate the answer drawer in example mode: at the `AnswerDrawer` render site (~line 378),
only allow opening it when `!exampleReport` (the 查看我的答案 button can be hidden or inert in
example mode — simplest: `onClick={() => { if (!exampleReport) setAnswersOpen(true); }}` and
hide the drawer trigger when `exampleReport` is set). Keep the change minimal.

- [ ] **Step 5: Run to verify it passes**

Run: `cd apps/web && npx vitest run test/tour/exampleCourseReport.test.tsx`
Expected: PASS.

- [ ] **Step 6: Typecheck + commit**

Run: `cd apps/web && npm run typecheck`

```bash
git add apps/web/src/tour/fixtures/exampleCourseReport.ts apps/web/src/shell/courses/CourseReport.tsx apps/web/test/tour/exampleCourseReport.test.tsx
git commit -m "feat(tour): CourseReport example mode + frozen example report fixture"
```

---

## Task 5: Courses navigation plumbing + `data-tour` anchors

Two things the segments (Task 8) depend on: (a) the tour can drive the 课程 sub-switcher and
open a course; (b) every spotlighted courses element carries a `data-tour` id.

**Files:**
- Modify: `apps/web/src/shell/CoursesTab.tsx` (pendingSub one-shot + wrapper `data-tour`)
- Modify: `apps/web/src/shell/courses/CoursesView.tsx:155,167` (category + grid anchors)
- Modify: `apps/web/src/shell/courses/CourseReport.tsx` (section anchors)
- Modify: `apps/web/src/shell/courses/LearningHistory.tsx`, `apps/web/src/shell/growth/ToolkitCards.tsx`, `apps/web/src/shell/growth/CardDetailModal.tsx`
- Modify: `apps/web/src/shell/courses/CoursesContainer.tsx` (player anchor reuse note)
- Test: `apps/web/test/shell/CoursesTab.test.tsx` (pendingSub)

**Interfaces:**
- Produces: `CoursesTab` accepts `pendingSub?: "courses"|"history"|"gallery"` + `onPendingSubConsumed?: () => void` (mirrors `pendingCourseId`); anchors listed in File Structure's vocabulary now exist in the DOM.

- [ ] **Step 1: Write the failing pendingSub test**

Create `apps/web/test/shell/CoursesTab.test.tsx` (stub the heavy children so it mounts):

```tsx
import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";

vi.mock("@/shell/courses/CoursesContainer", () => ({ CoursesContainer: () => <div data-testid="courses-container" />, }));
vi.mock("@/shell/courses/LearningHistory", () => ({ LearningHistory: () => <div data-testid="learning-history" />, }));
vi.mock("@/shell/growth/ToolkitCards", () => ({ ToolkitCards: () => <div data-testid="toolkit-cards" />, }));

import { CoursesTab } from "@/shell/CoursesTab";

describe("CoursesTab pendingSub", () => {
  it("opens on the requested sub-tab when pendingSub is given", () => {
    render(
      <CoursesTab
        pendingCourseId={null}
        onPendingCourseConsumed={() => {}}
        pendingSub="gallery"
        onPendingSubConsumed={() => {}}
        onGoPortal={() => {}}
        onImmersiveChange={() => {}}
      />,
    );
    expect(screen.getByTestId("toolkit-cards")).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd apps/web && npx vitest run test/shell/CoursesTab.test.tsx`
Expected: FAIL — `pendingSub` prop not accepted (renders default 课程).

- [ ] **Step 3: Add pendingSub to CoursesTab**

In `apps/web/src/shell/CoursesTab.tsx`, extend `CoursesTabProps` (lines 22-31):

```tsx
  /** One-shot: open on this sub-tab (guided tour). */
  pendingSub?: Sub | null;
  onPendingSubConsumed?: () => void;
```

Seed `sub` from it and consume once (mirror the `pendingCourseId` handling at lines 44-53):

```tsx
  const [sub, setSub] = useState<Sub>(pendingSub ?? "courses");
  useEffect(() => {
    if (pendingSub) onPendingSubConsumed?.();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
```

- [ ] **Step 4: Add the `data-tour` wrapper on the sub-switcher**

In `CoursesTab.tsx`, wrap the `<Segmented>` (line 75) container div with the anchor id (the
existing centering div is a fine host):

```tsx
        <div className="flex shrink-0 items-center justify-center px-4 pb-1.5 pt-4" data-tour="courses-subswitcher">
```

- [ ] **Step 5: Add courses-list anchors**

In `apps/web/src/shell/courses/CoursesView.tsx`:
- category chips wrapper (line 155): add `data-tour="courses-categories"` to the `<div style={{ display: "flex", flexWrap: "wrap", ... }}>`.
- course grid (line 167): add `data-tour="courses-grid"` to the grid `<div>`.

- [ ] **Step 6: Add report + history + gallery + card anchors**

- `CourseReport.tsx`: root container of the ready render (~line 263) → `data-tour="course-report"`; the two-Stat wrapper (~323) → `data-tour="course-report-stats"`; the 小测表现 card (~330) → `data-tour="course-report-quiz"`; the 学到的工具卡 `SectionCard` (~349) → `data-tour="course-report-cards"`.
- `LearningHistory.tsx`: the list/empty-state container (the `<Card>` at ~198 and the grouped list at ~212) → wrap with `data-tour="courses-history"`.
- `ToolkitCards.tsx`: the filters row (status seg ~300 + tags ~304) → `data-tour="tujian-filters"`; the grid (~329) → `data-tour="tujian-grid"`.
- `CardDetailModal.tsx`: the tab row (~281) → `data-tour="card-detail-tabs"`.

For shared primitives that don't pass arbitrary props (`Segmented`, `FilterChip`), anchor the
**surrounding element** (as above) rather than the primitive — do NOT modify the primitives.

- [ ] **Step 7: Player anchor**

Confirm the player root already exposes `data-testid="course-region"`
(`RuntimeCoursePlayer.tsx:286`). Segments will target `[data-testid="course-region"]`,
`.course-nav__next`, and `[data-testid="ask-bubble"]` — no new attribute needed. Add a comment
in `CoursesContainer.tsx` near the player branch noting these are tour anchors so they aren't
removed.

- [ ] **Step 8: Run tests + typecheck**

Run: `cd apps/web && npx vitest run test/shell/CoursesTab.test.tsx && npm run typecheck`
Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add apps/web/src/shell/CoursesTab.tsx apps/web/src/shell/courses/CoursesView.tsx apps/web/src/shell/courses/CourseReport.tsx apps/web/src/shell/courses/LearningHistory.tsx apps/web/src/shell/growth/ToolkitCards.tsx apps/web/src/shell/growth/CardDetailModal.tsx apps/web/src/shell/courses/CoursesContainer.tsx apps/web/test/shell/CoursesTab.test.tsx
git commit -m "feat(tour): courses nav plumbing (pendingSub) + data-tour anchors"
```

---

## Task 6: Welcome modal

The first-run greeting with the 课程 / 项目 / 稍后 branch.

**Files:**
- Create: `apps/web/src/tour/WelcomeModal.tsx`
- Test: `apps/web/test/tour/WelcomeModal.test.tsx`

**Interfaces:**
- Consumes: `Modal` (`@/ui`), `Pebble` (`@/ui`).
- Produces: `<WelcomeModal open displayName onPick onDismiss />` where
  `onPick: (start: "courses" | "projects") => void` and `onDismiss: () => void`.

- [ ] **Step 1: Write the failing test**

Create `apps/web/test/tour/WelcomeModal.test.tsx`:

```tsx
import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { WelcomeModal } from "@/tour/WelcomeModal";

describe("WelcomeModal", () => {
  it("greets by name and routes the three choices", () => {
    const onPick = vi.fn(); const onDismiss = vi.fn();
    render(<WelcomeModal open displayName="Phoebe" onPick={onPick} onDismiss={onDismiss} />);
    expect(screen.getByText(/Phoebe/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "课程" }));
    expect(onPick).toHaveBeenCalledWith("courses");
    fireEvent.click(screen.getByRole("button", { name: "项目" }));
    expect(onPick).toHaveBeenCalledWith("projects");
    fireEvent.click(screen.getByRole("button", { name: "稍后再说" }));
    expect(onDismiss).toHaveBeenCalled();
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd apps/web && npx vitest run test/tour/WelcomeModal.test.tsx`
Expected: FAIL — module not found.

- [ ] **Step 3: Implement the modal**

Create `apps/web/src/tour/WelcomeModal.tsx`:

```tsx
import { Modal, Pebble } from "@/ui";

export function WelcomeModal({ open, displayName, onPick, onDismiss }: {
  open: boolean;
  displayName: string;
  onPick: (start: "courses" | "projects") => void;
  onDismiss: () => void;
}) {
  const name = displayName?.trim() || "同学";
  return (
    <Modal open={open} onClose={onDismiss} title={null}>
      <div className="flex flex-col items-center text-center">
        <Pebble size={48} />
        <div className="mt-3 text-mk-h3 font-semibold text-mk-ink">{name}你好呀 👋</div>
        <p className="mt-2 text-mk-body text-mk-muted">
          欢迎来到思维印记 AI 思辨力成长平台。我是印记，你的 AI 小伙伴。要不让我先带你逛逛这里吧！想先看看什么呢？
        </p>
        <div className="mt-5 flex w-full flex-col gap-2">
          <button type="button" onClick={() => onPick("courses")}
            className="rounded-mk-md bg-mk-accent px-4 py-2.5 text-mk-body font-semibold text-white hover:opacity-90">课程</button>
          <button type="button" onClick={() => onPick("projects")}
            className="rounded-mk-md bg-mk-accent-50 px-4 py-2.5 text-mk-body font-semibold text-mk-accent-700 hover:bg-mk-accent-100">项目</button>
          <button type="button" onClick={onDismiss}
            className="mt-1 text-mk-small text-mk-muted hover:text-mk-ink">稍后再说</button>
        </div>
        <p className="mt-3 text-mk-small text-mk-muted">以后想再逛，可以点左边导航底部的“重新开始引导”。</p>
      </div>
    </Modal>
  );
}
```

Note the `title={null}` — the built-in `Modal` renders an X close button; clicking it or the
scrim calls `onClose` = `onDismiss`, which the parent treats as "稍后" (stamps `onboarded_at`).

- [ ] **Step 4: Run to verify it passes**

Run: `cd apps/web && npx vitest run test/tour/WelcomeModal.test.tsx`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/tour/WelcomeModal.tsx apps/web/test/tour/WelcomeModal.test.tsx
git commit -m "feat(tour): welcome modal with courses/projects/later branch"
```

---

## Task 7: Nav footer (重新开始引导 / 反馈 / 退出登录)

Adds the bottom-of-rail controls and the props to drive them.

**Files:**
- Modify: `apps/web/src/shell/Nav.tsx:42-50,99-140`
- Modify: `apps/web/test/shell/Nav.test.tsx`

**Interfaces:**
- Produces: `Nav` accepts `onRestartTour?: () => void`, `onFeedback?: () => void`, `onLogout?: () => void`. Footer buttons carry `data-tour="nav-restart-tour"`, `data-tour="nav-feedback"`.

- [ ] **Step 1: Extend the Nav test**

In `apps/web/test/shell/Nav.test.tsx`, add:

```tsx
  it("renders the footer controls and fires their callbacks", () => {
    const onRestartTour = vi.fn(); const onFeedback = vi.fn(); const onLogout = vi.fn();
    render(<Nav tab="home" onTab={() => {}} user={{ display_name: "Phoebe" }}
      onRestartTour={onRestartTour} onFeedback={onFeedback} onLogout={onLogout} />);
    fireEvent.click(screen.getByText("重新开始引导"));
    expect(onRestartTour).toHaveBeenCalled();
    fireEvent.click(screen.getByText("反馈"));
    expect(onFeedback).toHaveBeenCalled();
    fireEvent.click(screen.getByText("退出登录"));
    expect(onLogout).toHaveBeenCalled();
  });
```

The existing `Nav` test calls pass no new props; keep them working by making all three optional.

- [ ] **Step 2: Run to verify it fails**

Run: `cd apps/web && npx vitest run test/shell/Nav.test.tsx`
Expected: FAIL — footer text not found.

- [ ] **Step 3: Add the footer**

In `apps/web/src/shell/Nav.tsx`, extend the props (lines 42-50):

```tsx
export function Nav({ tab, onTab, user, onRestartTour, onFeedback, onLogout }: {
  tab: NavTab; onTab: (t: NavTab) => void; user?: Pick<MeUser, "display_name"> | null;
  onRestartTour?: () => void; onFeedback?: () => void; onLogout?: () => void;
}) {
```

After the `items.map(...)` block (after line 136, still inside `<nav>`), add a footer pinned to
the bottom. Reuse `labelCls` (line 93-95) so labels reveal on hover, matching the rail:

```tsx
        <div className="mt-auto flex flex-col gap-1 pt-2">
          <NavFooterButton dataTour="nav-restart-tour" icon={<Compass className="h-[18px] w-[18px] text-white/80" />} label="重新开始引导" onClick={onRestartTour} labelCls={labelCls} />
          <NavFooterButton dataTour="nav-feedback" icon={<MessageSquare className="h-[18px] w-[18px] text-white/80" />} label="反馈" onClick={onFeedback} labelCls={labelCls} />
          <NavFooterButton icon={<LogOut className="h-[18px] w-[18px] text-white/80" />} label="退出登录" onClick={onLogout} labelCls={labelCls} />
        </div>
```

Add the small helper at the bottom of the file and import the icons at the top
(`import { Home, FolderKanban, GraduationCap, Compass, MessageSquare, LogOut } from "lucide-react";`):

```tsx
function NavFooterButton({ icon, label, onClick, labelCls, dataTour }: {
  icon: ReactNode; label: string; onClick?: () => void; labelCls: string; dataTour?: string;
}) {
  return (
    <button type="button" data-tour={dataTour} onClick={onClick}
      className="flex items-center gap-3 rounded-mk-md px-1.5 py-2 transition-colors duration-[120ms] ease-mk hover:bg-white/10 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/60">
      <span className="flex h-7 w-7 shrink-0 items-center justify-center">{icon}</span>
      <span className={cx(labelCls, "text-white/80")}>{label}</span>
    </button>
  );
}
```

(`cx` and `ReactNode` are already in scope/imported in Nav.tsx.)

- [ ] **Step 4: Run to verify it passes**

Run: `cd apps/web && npx vitest run test/shell/Nav.test.tsx`
Expected: PASS (both the new footer test and the existing four).

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/shell/Nav.tsx apps/web/test/shell/Nav.test.tsx
git commit -m "feat(tour): nav footer — restart tour / feedback / logout"
```

---

## Task 8: Courses segment config + journey composition

The declarative Courses group, referencing Task 5's anchors and Task 4's example report.

**Files:**
- Create: `apps/web/src/tour/segments/courses.ts`, `apps/web/src/tour/journey.ts`
- Test: `apps/web/test/tour/courses-segments.test.ts`

**Interfaces:**
- Consumes: `TourSegment`/`TourNavContext` (Task 2); anchor ids (Task 5).
- Produces: `coursesSegments: TourSegment[]`; `coursesJourney: TourJourney`; a P1 `fullJourney` (= courses only for now).

- [ ] **Step 1: Write the failing config test**

Create `apps/web/test/tour/courses-segments.test.ts`:

```ts
import { describe, it, expect, vi } from "vitest";
import { coursesSegments, coursesJourney } from "@/tour/segments/courses";
import type { TourNavContext } from "@/tour/types";

describe("courses segments", () => {
  it("are well-formed: unique step ids, non-empty text, valid advance", () => {
    const ids = new Set<string>();
    for (const seg of coursesSegments) {
      expect(seg.steps.length).toBeGreaterThan(0);
      for (const s of seg.steps) {
        expect(s.text.length).toBeGreaterThan(0);
        expect(["next", "action"]).toContain(s.advance);
        if (s.advance === "action") expect(s.actionEvent).toBeTruthy();
        expect(ids.has(s.id)).toBe(false);
        ids.add(s.id);
      }
    }
  });

  it("the first segment's first step navigates to the courses tab (standalone-safe)", () => {
    const nav: TourNavContext = { setTab: vi.fn(), openCourse: vi.fn(), setCoursesSub: vi.fn() };
    coursesJourney[0].steps[0].onEnter?.(nav);
    expect(nav.setTab).toHaveBeenCalledWith("courses");
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd apps/web && npx vitest run test/tour/courses-segments.test.ts`
Expected: FAIL — module not found.

- [ ] **Step 3: Author the segments**

Create `apps/web/src/tour/segments/courses.ts`. Every segment's first step is standalone-safe
(navigates into position). Text is 印记's voice (short, one idea per step).

```ts
import type { TourSegment } from "../types";

const EXAMPLE_SLUG = "__example__"; // example-report view is opened by StudentApp (Task 9)

export const coursesSegments: TourSegment[] = [
  {
    id: "courses-intro",
    name: "为什么有这些课程",
    steps: [
      {
        id: "courses-intro-0",
        onEnter: (nav) => { nav.setTab("courses"); nav.setCoursesSub("courses"); },
        placement: "center",
        title: "先聊聊课程",
        text: "印记认为，**思辨力**是 AI 时代最重要的能力，而它可以通过系统化的学习和操练一点点长出来。所以我们准备了一系列小课，带你练。",
        advance: "next",
      },
    ],
  },
  {
    id: "courses-categories",
    name: "课程分类",
    steps: [
      {
        id: "courses-categories-0",
        onEnter: (nav) => { nav.setTab("courses"); nav.setCoursesSub("courses"); },
        anchor: '[data-tour="courses-categories"]',
        placement: "bottom",
        text: "课程按主题分成了几类，你可以用这些标签快速筛选，找到当下最想练的那一类。",
        advance: "next",
      },
    ],
  },
  {
    id: "courses-enter",
    name: "进入一门课",
    steps: [
      {
        id: "courses-enter-0",
        anchor: '[data-tour="courses-grid"]',
        placement: "top",
        text: "点开任意一门课的卡片，就能看到它的介绍并开始学习。挑一门你感兴趣的试试吧。",
        advance: "next",
      },
    ],
  },
  {
    id: "courses-player",
    name: "课程里怎么互动",
    steps: [
      {
        id: "courses-player-0",
        anchor: '[data-testid="course-region"]',
        placement: "center",
        text: "课程是一屏一屏推进的：印记会先讲解，再请你回答小问题。想清楚了再往下走——**不用赶**。",
        advance: "next",
      },
      {
        id: "courses-player-1",
        anchor: '[data-testid="ask-bubble"]',
        placement: "left",
        text: "学的过程中有疑问，随时在这里问我，我就在你身边。",
        advance: "next",
      },
    ],
  },
  {
    id: "courses-report-example",
    name: "看懂学习报告",
    steps: [
      {
        id: "courses-report-example-0",
        onEnter: (nav) => nav.openCourse(EXAMPLE_SLUG),
        anchor: '[data-tour="course-report"]',
        placement: "center",
        title: "这是一份学习报告的样子",
        text: "每学完一门课，你都会拿到这样一份报告。下面我带你看看它由哪几块组成。",
        advance: "next",
      },
      {
        id: "courses-report-example-1",
        anchor: '[data-tour="course-report-stats"]',
        placement: "bottom",
        text: "这里是你的**用时**和**完成的阶段数**，一眼看到你走了多远。",
        advance: "next",
      },
      {
        id: "courses-report-example-2",
        anchor: '[data-tour="course-report-quiz"]',
        placement: "bottom",
        text: "小测表现记录了你答对了几题，还能逐题回看自己的作答。",
        advance: "next",
      },
      {
        id: "courses-report-example-3",
        anchor: '[data-tour="course-report-cards"]',
        placement: "top",
        text: "这门课练到的**思维工具卡**会收进这里——它们会在你的图鉴里慢慢集齐。",
        advance: "next",
      },
    ],
  },
  {
    id: "courses-history",
    name: "学习记录",
    steps: [
      {
        id: "courses-history-0",
        onEnter: (nav) => { nav.setTab("courses"); nav.setCoursesSub("history"); },
        anchor: '[data-tour="courses-history"]',
        placement: "center",
        text: "你学过、正在学的课程都会记录在“学习记录”里，随时能回来继续，或重看报告。刚开始这里是空的，学起来就有了。",
        advance: "next",
      },
    ],
  },
  {
    id: "courses-tujian",
    name: "图鉴与工具卡",
    steps: [
      {
        id: "courses-tujian-0",
        onEnter: (nav) => { nav.setTab("courses"); nav.setCoursesSub("gallery"); },
        anchor: '[data-tour="tujian-grid"]',
        placement: "center",
        text: "这是你的**思维工具卡图鉴**。每张卡是一种思考方法，练过就会被点亮、攒起星星。",
        advance: "next",
      },
      {
        id: "courses-tujian-1",
        anchor: '[data-tour="tujian-grid"]',
        placement: "center",
        text: "点开任意一张卡，看看它讲什么。",
        advance: "action",
        actionEvent: { selector: '[data-tour="tujian-grid"] button', type: "click" },
      },
      {
        id: "courses-tujian-2",
        anchor: '[data-tour="card-detail-tabs"]',
        placement: "bottom",
        text: "卡片里有“介绍”和“我的练习历史”。练习历史会记录你在项目里用过它几次，以及能在哪些课程里学到它。",
        advance: "next",
      },
    ],
  },
];

export const coursesJourney = coursesSegments;
```

- [ ] **Step 4: Compose the journey**

Create `apps/web/src/tour/journey.ts`:

```ts
import type { TourJourney } from "./types";
import { coursesSegments } from "./segments/courses";

export const coursesJourney: TourJourney = coursesSegments;
// P1: the full journey is the courses group only. P3 concatenates the projects group,
// and the welcome modal's courses/projects choice reorders the two groups.
export const fullJourney: TourJourney = coursesSegments;
```

- [ ] **Step 5: Run to verify it passes**

Run: `cd apps/web && npx vitest run test/tour/courses-segments.test.ts`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add apps/web/src/tour/segments/courses.ts apps/web/src/tour/journey.ts apps/web/test/tour/courses-segments.test.ts
git commit -m "feat(tour): courses segment config + journey composition"
```

---

## Task 9: StudentApp integration + first-run trigger

Wires everything: mount `TourProvider` + `TourRunner`, assemble `TourNavContext`, drive the
example-report view, show the welcome modal on first login, stamp `onboarded_at`, thread the
nav-footer props, and support `pendingCoursesSub`.

**Files:**
- Modify: `apps/web/src/shell/StudentApp.tsx`
- Test: `apps/web/test/shell/StudentApp.tour.test.tsx`

**Interfaces:**
- Consumes: `TourProvider`/`TourRunner`/`useTour` (Tasks 2-3), `WelcomeModal` (Task 6), `coursesJourney`/`fullJourney` (Task 8), `api.putOnboarding` (Task 1), `CoursesTab.pendingSub` (Task 5), `Nav` footer props (Task 7), `exampleCourseReport` (Task 4).

- [ ] **Step 1: Write the failing integration test**

Create `apps/web/test/shell/StudentApp.tour.test.tsx` (mirror `StudentApp.test.tsx`'s session
+ mocks). Assert the welcome modal shows when `onboarded_at` is null and that dismissing it
calls `putOnboarding`:

```tsx
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";

const putOnboarding = vi.fn(async () => {});
vi.mock("@/api", async (orig) => {
  const actual = await (orig as any)();
  return { ...actual, api: { ...actual.api, putOnboarding, setAccent: vi.fn(async () => {}), setBackground: vi.fn(async () => {}) } };
});
vi.mock("@/workspace/WorkspaceContainer", () => ({ WorkspaceContainer: () => <div /> }));
vi.mock("@/shell/courses/CoursesContainer", () => ({ CoursesContainer: () => <div /> }));

import { StudentApp } from "@/shell/StudentApp";
import { createSession } from "@/shell/session";

function makeSession(onboardedAt: string | null) {
  const store: Record<string, string> = {};
  const s = createSession({ storage: { getItem: (k) => store[k] ?? null, setItem: (k, v) => { store[k] = v; } } as any });
  s.setUser({
    id: "u1", email: "p@x.cn", display_name: "Phoebe", role: "student",
    avatar_color: "vermilion", page_background: "paper", onboarded_at: onboardedAt,
    school: { id: "s1", name: "X" }, classes: [{ id: "c1", name: "1", role_in_class: "student" }],
  });
  return s;
}

describe("StudentApp onboarding", () => {
  beforeEach(() => putOnboarding.mockClear());

  it("shows the welcome modal for a never-onboarded user", () => {
    render(<StudentApp session={makeSession(null)} onLogout={() => {}} />);
    expect(screen.getByText(/欢迎来到思维印记/)).toBeInTheDocument();
  });

  it("does NOT show the welcome modal once onboarded", () => {
    render(<StudentApp session={makeSession("2026-08-01T00:00:00Z")} onLogout={() => {}} />);
    expect(screen.queryByText(/欢迎来到思维印记/)).not.toBeInTheDocument();
  });

  it("stamps onboarding when the user picks 稍后再说", () => {
    render(<StudentApp session={makeSession(null)} onLogout={() => {}} />);
    fireEvent.click(screen.getByRole("button", { name: "稍后再说" }));
    expect(putOnboarding).toHaveBeenCalled();
    expect(screen.queryByText(/欢迎来到思维印记/)).not.toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd apps/web && npx vitest run test/shell/StudentApp.tour.test.tsx`
Expected: FAIL — no welcome modal wired.

- [ ] **Step 3: Wire TourProvider, nav context, welcome modal, example-report view**

In `apps/web/src/shell/StudentApp.tsx`:

Add imports:

```tsx
import { TourProvider } from "@/tour/TourProvider";
import { TourRunner } from "@/tour/TourRunner";
import { WelcomeModal } from "@/tour/WelcomeModal";
import { fullJourney } from "@/tour/journey";
import type { TourNavContext } from "@/tour/types";
```

Add state near the other `useState`s:

```tsx
  const [welcomeOpen, setWelcomeOpen] = useState(user?.onboarded_at == null);
  const [pendingCoursesSub, setPendingCoursesSub] = useState<null | "courses" | "history" | "gallery">(null);
```

Add the example-report deep-link. The tour opens `openCourse("__example__")`; make `openCourse`
route the example slug to the courses tab in a mode that renders `CourseReport exampleReport=…`.
Simplest for P1: add a `showExampleReport` state and render it as an overlay above the body
when set. Add:

```tsx
  const [showExampleReport, setShowExampleReport] = useState(false);
  function openCourse(slug: string) {
    if (slug === "__example__") { setTab("courses"); setShowExampleReport(true); return; }
    setPendingCourseId(slug); setTab("courses");
  }
```

Build the nav context (all setters are in scope in this component):

```tsx
  const tourNav: TourNavContext = {
    setTab,
    openCourse,
    setCoursesSub: (s) => { setTab("courses"); setPendingCoursesSub(s); },
  };
```

Stamp helper:

```tsx
  function completeOnboarding() { void api.putOnboarding(); }
```

Wrap the existing render tree in `TourProvider` (just inside `AccentProvider`/`BackgroundProvider`,
around the `<div className="flex h-full w-full ...">`), add `<TourRunner />` and the
`<WelcomeModal>` + example-report overlay as siblings of the body:

```tsx
      <TourProvider nav={tourNav} onComplete={completeOnboarding}>
        <div className="flex h-full w-full overflow-hidden bg-mk-paper">
          {showNav && (
            <Nav
              tab={tab} onTab={setTab} user={user}
              onRestartTour={() => { /* set by inner component; see Step 4 */ }}
              onFeedback={() => { /* Task 10 */ }}
              onLogout={onLogout}
            />
          )}
          <div className="relative flex-1 overflow-hidden">{body}</div>
        </div>
        <TourRunner />
        <WelcomeModal
          open={welcomeOpen}
          displayName={user?.display_name ?? ""}
          onPick={(start) => { setWelcomeOpen(false); /* play — see Step 4 */ }}
          onDismiss={() => { setWelcomeOpen(false); completeOnboarding(); }}
        />
        {showExampleReport && (
          <div className="absolute inset-0 z-40 overflow-auto bg-mk-paper">
            {/* import CourseReport + exampleCourseReport */}
            <CourseReport courseId="__example__" exampleReport={exampleCourseReport}
              onBackToCourses={() => setShowExampleReport(false)} onGoPortal={() => setShowExampleReport(false)} />
          </div>
        )}
      </TourProvider>
```

- [ ] **Step 4: Connect welcome → play and the nav re-trigger via a small inner bridge**

Because `useTour()` must be called *inside* `TourProvider`, extract the body + welcome +
runner into a small inner component that consumes `useTour()`:

```tsx
function StudentAppInner(props: {
  /* pass tab, setTab, user, body, showNav, onLogout, welcomeOpen, setWelcomeOpen,
     completeOnboarding, showExampleReport, setShowExampleReport, pendingCoursesSub,
     setPendingCoursesSub, ... */
}) {
  const tour = useTour();
  // Nav footer: onRestartTour={() => tour.play(fullJourney)}
  // Welcome onPick: (start) => { props.setWelcomeOpen(false); tour.play(fullJourney); }
  //   (P1 fullJourney = courses only, so start order is a no-op until P3; keep the param.)
  // ...render Nav (with footer props), body, TourRunner, WelcomeModal, example overlay...
}
```

Then `StudentApp` renders `<TourProvider nav={tourNav} onComplete={completeOnboarding}><StudentAppInner .../></TourProvider>`.
Wire `CoursesTab` to consume `pendingCoursesSub`:

```tsx
        <CoursesTab
          pendingCourseId={pendingCourseId}
          onPendingCourseConsumed={() => setPendingCourseId(null)}
          pendingSub={pendingCoursesSub}
          onPendingSubConsumed={() => setPendingCoursesSub(null)}
          studentId={user?.id}
          onGoPortal={() => setTab("projects")}
          onImmersiveChange={setCoursesImmersive}
        />
```

Keep the implementation minimal and typed; the test in Step 1 only asserts the welcome-modal
behavior, so ensure that path is correct.

- [ ] **Step 5: Run to verify it passes**

Run: `cd apps/web && npx vitest run test/shell/StudentApp.tour.test.tsx && npm run typecheck`
Expected: PASS. Also run the existing `StudentApp.test.tsx` to ensure no regression:
`npx vitest run test/shell/StudentApp.test.tsx` (update that test's `MeUser` literal to include `onboarded_at: "..."` so the welcome modal doesn't interfere).

- [ ] **Step 6: Manual browser smoke (format-check readiness)**

Run the stack and confirm: welcome modal appears for a fresh user; picking 课程 starts the
tour; spotlight + 印记 bubble render over real elements; 下一步 advances; the example report
shows; 结束 ends and does not reappear on reload (onboarded_at stamped). Note any positioning
issues for polish.

- [ ] **Step 7: Commit**

```bash
git add apps/web/src/shell/StudentApp.tsx apps/web/test/shell/StudentApp.tour.test.tsx apps/web/test/shell/StudentApp.test.tsx
git commit -m "feat(tour): wire TourProvider + welcome modal + first-run trigger in StudentApp"
```

---

## Task 10: Feedback (table + endpoint + modal) — droppable

Not core to onboarding; do last so it can be cut if needed. The nav 反馈 button opens a small
form that saves to a `feedback` table. Mirrors the `citation` 4-file flow.

**Files:**
- Create: `apps/api/internal/store/migrations/0080_feedback.sql`, `apps/api/internal/store/queries/feedback.sql`, `apps/api/internal/api/feedback.go`, `apps/api/internal/api/feedback_test.go`
- Modify: `apps/api/internal/api/api.go` (route), regen sqlc
- Modify: `apps/web/src/api/*` (client), `apps/web/src/tour/FeedbackModal.tsx` (new), `apps/web/src/shell/StudentApp.tsx` (wire onFeedback)
- Test: `apps/web/test/tour/FeedbackModal.test.tsx`

**Interfaces:**
- Produces: `POST /api/v1/feedback` (protected) body `{ text: string }` → `{ id }`, 400 on empty; `api.submitFeedback(text: string): Promise<void>`.

- [ ] **Step 1: Migration**

Create `apps/api/internal/store/migrations/0080_feedback.sql` (mirror `0068_citation.sql`):

```sql
-- +goose Up
CREATE TABLE feedback (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    text       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX feedback_user_idx ON feedback(user_id);

-- +goose Down
DROP TABLE feedback;
```

- [ ] **Step 2: Query**

Create `apps/api/internal/store/queries/feedback.sql`:

```sql
-- name: CreateFeedback :one
INSERT INTO feedback (user_id, text)
VALUES ($1, $2)
RETURNING *;
```

- [ ] **Step 3: Regen + build**

Run: `cd apps/api && make sqlc && go build ./...`
Expected: PASS; `CreateFeedback(ctx, CreateFeedbackParams{UserID, Text})` generated.

- [ ] **Step 4: Failing handler test**

Create `apps/api/internal/api/feedback_test.go` (mirror `users_accent_test.go`): unauth → 401;
empty text → 400; valid → 201 with an id; row exists for the seed user.

```go
func TestPostFeedback(t *testing.T) {
	pool := newAPITestPool(t)
	h := newTestAPI(pool).Handler()
	cookie := signInSeed(t, pool)

	rr := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]string{"text": "很好用"})
	h.ServeHTTP(rr, httptest.NewRequest("POST", "/api/v1/feedback", bytes.NewReader(body)))
	if rr.Code != http.StatusUnauthorized { t.Fatalf("unauth want 401 got %d", rr.Code) }

	rr = httptest.NewRecorder()
	empty, _ := json.Marshal(map[string]string{"text": "  "})
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", "/api/v1/feedback", bytes.NewReader(empty)), cookie))
	if rr.Code != http.StatusBadRequest { t.Fatalf("empty want 400 got %d", rr.Code) }

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", "/api/v1/feedback", bytes.NewReader(body)), cookie))
	if rr.Code != http.StatusCreated { t.Fatalf("ok want 201 got %d (%s)", rr.Code, rr.Body.String()) }
}
```

- [ ] **Step 5: Run to verify it fails**

Run: `cd apps/api && go test ./internal/api/ -run TestPostFeedback -timeout 900s`
Expected: FAIL (route/handler missing).

- [ ] **Step 6: Handler + route**

Create `apps/api/internal/api/feedback.go` (scope by `UserFromContext`, trim/validate text):

```go
package api

import (
	"net/http"
	"strings"

	"github.com/<module>/apps/api/internal/httpx"
	"github.com/<module>/apps/api/internal/store/sqlc"
)

func (a *API) postFeedback(w http.ResponseWriter, r *http.Request) {
	u, ok := UserFromContext(r.Context())
	if !ok { httpx.WriteError(w, r, httpx.ErrUnauthorized("未登录")); return }
	var req struct{ Text string `json:"text"` }
	if err := decodeJSON(r, &req); err != nil { httpx.WriteError(w, r, err); return }
	text := strings.TrimSpace(req.Text)
	if text == "" { httpx.WriteError(w, r, httpx.ErrBadRequest("empty_feedback", "反馈内容不能为空。", nil)); return }
	row, err := a.d.Queries.CreateFeedback(r.Context(), sqlc.CreateFeedbackParams{UserID: u.ID, Text: text})
	if err != nil { httpx.WriteError(w, r, err); return }
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"id": row.ID.String()})
}
```

Register in `api.go` near the other user routes:

```go
mux.Handle("POST /api/v1/feedback", protected(a.postFeedback))
```

- [ ] **Step 7: Run to verify it passes**

Run: `cd apps/api && go build ./... && go test ./internal/api/ -run TestPostFeedback -timeout 900s`
Expected: PASS.

- [ ] **Step 8: TS client + FeedbackModal + wire**

Add `submitFeedback(text)` to `apps/web/src/api/*` (mirror `putOnboarding`; POST body
`{ text }`). Create `apps/web/src/tour/FeedbackModal.tsx` (reuse `@/ui` `Modal` + a textarea +
submit calling `api.submitFeedback`, then `toast`). Wire `onFeedback` in StudentApp to open it.
Test `apps/web/test/tour/FeedbackModal.test.tsx`: typing + submit calls `api.submitFeedback`
with the text; empty submit is disabled.

- [ ] **Step 9: Run frontend tests + typecheck**

Run: `cd apps/web && npx vitest run test/tour/FeedbackModal.test.tsx && npm run typecheck`
Expected: PASS.

- [ ] **Step 10: Commit**

```bash
git add apps/api/internal/store/migrations/0080_feedback.sql apps/api/internal/store/queries/feedback.sql apps/api/internal/store/sqlc/ apps/api/internal/api/feedback.go apps/api/internal/api/feedback_test.go apps/api/internal/api/api.go apps/web/src/api/ apps/web/src/tour/FeedbackModal.tsx apps/web/src/shell/StudentApp.tsx apps/web/test/tour/FeedbackModal.test.tsx
git commit -m "feat(tour): feedback table + endpoint + modal"
```

---

## Final verification (before finishing the phase)

- [ ] `cd apps/web && npm run typecheck` — clean.
- [ ] `cd apps/web && npm run test` — full frontend suite green (existing + new tour tests).
- [ ] `cd apps/api && go build ./... && go test ./... -timeout 1800s` — Go suite green (Docker running).
- [ ] Manual browser walk of the courses tour end-to-end (Task 9 Step 6) — this is the user's format-check gate.
- [ ] Push to main.

---

## Self-Review (author's pass)

**Spec coverage (P1 scope):** engine (Tasks 2-3) ✓; `onboarded_at` server field + writer
(Task 1) ✓; welcome modal (Task 6) ✓; nav footer (Task 7) ✓; courses segments + anchors
(Tasks 5, 8) ✓; frozen example report (Task 4) ✓; first-run trigger + integration (Task 9) ✓;
feedback (Task 10, droppable) ✓. Demo project, backend demo-mode guard, and the projects
segments are correctly OUT of P1 (they are P2/P3). No-live-LLM holds (example report is a
fixture; no course is completed).

**Placeholder scan:** the only "TBD"-shaped notes are P3 forward-pointers in `journey.ts` and
Task 9's inner-component sketch, which include the actual wiring; the fragile-`next` draft in
Task 2 is explicitly marked to be deleted in favor of `nextStable`. Card ids in the fixture
(Task 4) must be verified against `packages/contracts/cards/` — called out as a step, not left
vague.

**Type consistency:** `TourNavContext` (`setTab`/`openCourse`/`setCoursesSub`) is used
identically in Tasks 2, 8, 9. `CoursesSub` values (`courses`/`history`/`gallery`) match
`CoursesTab`'s `Sub`. `exampleReport?: CourseReportT` matches the `CourseReport` contract type.
`MeUser.onboarded_at: string | null` matches the DTO `*string json:"onboarded_at"`.
