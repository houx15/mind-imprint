# P4 · 「继续印记」takeover-return + narration polish Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Close the last agentic-studio gap — after a student manually takes over (switcher / self-editing), give them a visible **「继续印记」** affordance that returns them to 印记's current status step; and polish 印记's narration so it always says what it just configured and asks exactly one next-step question.

**Architecture:** Manual takeover already sets `tookOver` (P2a) without mutating `studio_state`; `applyStudioState(state)` re-opens the status room and resets `tookOver`. P4 surfaces a 「继续印记」control while `tookOver` is true that calls `applyStudioState(studioState)` (re-fetching `getStudioState` first for freshness) — returning to 印记's step. The narration piece is a focused orchestrator-prompt refinement (no new tools).

**Tech Stack:** React+TS+Tailwind (`apps/web`) + a Go prompt tweak (`apps/api`). Branch `studio-p2-focused-but-free` (on top of P3).

## Global Constraints
- **Spec:** `docs/superpowers/specs/2026-08-07-agentic-studio-orchestrator-redesign.md` §6 (手动接管 doesn't change status; 「继续印记」returns to status-step) + §8 (叙述语气: 叙述已配置好的工作台 + 引导下一步, 一次一问).
- **铁律②:** 印记 guides; the student can always take over; 「继续印记」is opt-in, never forced/auto.
- **DESIGN PRINCIPLES (enforced):** any NEW/edited UI ≥14px body, 12px only for hints; never `bg-mk-<token>/<opacity>` / `border-mk-<token>/<NN>` (transparent); one Tailwind class per competing property; centered illustrated empty states where applicable.
- Web: pnpm; tests `apps/web/test/**`; tsc 0 + vitest. Go: `CGO_ENABLED=0`, tests FOREGROUND (docker), pre-existing fails not ours. Never `git add -A`.

---

## File Structure
- `apps/web/src/workspace/WorkspaceContainer.tsx` — the 「继续印记」control in the switcher row + its handler (T1).
- `apps/api/internal/agent/orchestrator.go` — narration guidance in `orchestratorSystemPrompt` (T2).

---

## Task 1: 「继续印记」returns to 印记's status step

**Files:** `apps/web/src/workspace/WorkspaceContainer.tsx`; Test `apps/web/test/workspace/WorkspaceContainer.test.tsx`.

**Interfaces — Consumes:** existing `tookOver` state, `studioState`, `applyStudioState`, `getStudioState`, `handleManualRoom`. **Produces:** a visible 「继续印记」control shown ONLY while `tookOver === true`, which returns the view to the status step.

- [ ] **Step 1: Write the failing test.** In `WorkspaceContainer.test.tsx`: open a project that resolves chat-first (or any status), manually take over via the switcher to a different room (assert `tookOver` effect — the manually chosen room shows), then assert a 「继续印记」button is present; click it and assert the view returns to the status room (the one `roomForResume(studioState)` names) and the 「继续印记」button disappears (tookOver cleared). Use the existing `fakeStudioState`/`getStudioState`/`handleManualRoom` test seams.

- [ ] **Step 2: Implement.** In the switcher row (grep the `<Segmented .../>` + `PlanSpine` block), render a 「继续印记」control ONLY when `tookOver` is true (and a project is open). Its handler:

```tsx
const continueYinji = useCallback(async () => {
  const pid = activeProjectIdRef.current;
  if (!pid) return;
  // Re-fetch the current status for freshness, then re-assert 印记's view
  // (applyStudioState resets tookOver + opens the status room). Fall back to
  // the already-loaded studioState if the refetch fails.
  try {
    const fresh = await getStudioState(pid);
    if (activeProjectIdRef.current === pid) applyStudioState(fresh);
  } catch {
    if (studioState) applyStudioState(studioState);
  }
}, [applyStudioState, studioState]);
```

  Style the control per the design system: a calm chip/button, **≥14px** label (e.g. `继续印记 →`), solid tokens (e.g. `border-mk-accent`/`bg-mk-accent-50`), not shouty. Place it at the trailing end of the switcher row so it reads as "return to the guided flow."

- [ ] **Step 3:** Full web suite green + tsc 0. **Step 4: Commit** `feat(studio): 「继续印记」returns the student to 印记's status step (P4)`.

---

## Task 2: Narration polish — 印记 says what it configured + one next-step question

**Files:** `apps/api/internal/agent/orchestrator.go`; Test `apps/api/internal/agent/orchestrator_test.go` (light, no docker).

**Interfaces — Produces:** a sharpened `orchestratorSystemPrompt` narration directive (no new tools, no arg changes).

- [ ] **Step 1:** In `orchestratorSystemPrompt`, refine the narration guidance so 印记, whenever it configures the workspace (open_tool / curate_reference / generate_plan), **narrates what it set up AND asks exactly one next-step question** — spec §8's voice. Concrete example line (keep it concise, same Chinese voice), e.g. append to the posture:

```
叙述规则：每当你配置了工作台（开了房间 / 摆了参考 / 生成了计划），narrate 里先用一句话说清「我给你配了什么」，再问下一步唯一的一个问题——像「写作面板给你开好了，左边把你读过的材料列出来了。先跟我说说你打算怎么开头？」。一次只问一个，不连问，不替学生定论。
```

  Keep the existing "一次只问一个" / 克制 rules intact; this makes the narration-of-configuration explicit.

- [ ] **Step 2:** A light agent-package test asserting the prompt contains the narration-of-configuration directive (a substring check on `orchestratorSystemPrompt`), so a future edit can't silently drop it. `CGO_ENABLED=0 go test ./internal/agent/` foreground.

- [ ] **Step 3: Commit** `feat(agent): narration — 印记 states what it configured + one next-step question (P4)`.

---

## Self-Review Checklist
- **Spec §6:** 「继续印记」visible only on takeover, returns to status step, clears tookOver ✓. Manual takeover still never mutates studio_state ✓.
- **Spec §8:** narration states the configured workspace + one question ✓.
- **铁律②:** opt-in return, never forced ✓.
- **Design:** the new control ≥14px, solid tokens ✓.
- **Green:** web tsc 0 + vitest; Go agent test foreground.
