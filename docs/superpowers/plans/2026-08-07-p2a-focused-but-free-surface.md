# P2a · 「聚焦但自由」交互区（前端表层）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Re-shape the studio's interactive area so it shows only the current step's artifact + optional direct manipulation — morphing width, a 5-segment manual switcher (提案/管理/阅读/写作/回顾), and deletion of the Tier-1 redundant chrome — all frontend-only and independently deployable.

**Architecture:** The interactive area's width now follows 印记's stored `widthTier` (chat full-width → half → wide + slim chat rail). Today's conflated `立项` room splits into two switcher segments — 提案 (PlanBlock forming phase) and 管理 (PlanBlock board phase) — driven by the `room` state, not PlanBlock's internal self-decision. The reading room stops being a separate coach surface and joins the one constant 印记 rail. Every button/panel/copy that duplicates 印记's job (stage-transition nudges it cues in chat, "chat about X" doorways, room-navigation copy, the 中/EN switcher) is deleted. Anything that requires a NEW 印记 capability (`生成计划`, exploration question-creation) is explicitly OUT of scope — it lands in P2b.

**Tech Stack:** React 18 + Vite + TypeScript + Tailwind (`apps/web`); Vitest (glob `test/**`); design system `docs/superpowers/specs/2026-08-06-design-system-design.md`.

## Global Constraints

- **Spec of record:** `docs/superpowers/specs/2026-08-07-agentic-studio-orchestrator-redesign.md` §7.5 (the P2 kill-list + rulings + 5-segment switcher + morphing). The principle: interactive area = current step's artifact + optional direct manipulation; 印记 cues every transition from chat; the "free" affordances (edit note, adjust/export plan, select-to-ask, request review) stay first-class visible.
- **Frontend-only, no backend/contract changes.** The `openTool` enum stays `chat/plan/reading/writing/reflection` (the `forming` value is P2b). 提案 vs 管理 is a frontend `room` distinction; resume maps 印记's `plan` directive to forming-or-board using `studioState.stage` (`proposal_forming`/`topic_discussion` → forming; else board).
- **Do NOT delete Tier-2 chrome:** `生成项目计划` button + regen modal (PlanBlock), and the reading-room question boxes (`记下问题`/`从笔记新建问题`/seed). They stay until P2b. Deleting them here breaks plan-generation / question-creation.
- **Design-system gotchas:** one Tailwind class per competing CSS property; **never** `bg-mk-<hex-token>/<opacity>` or `border-mk-<hextoken>/<NN>` (renders transparent). Animations use `ease-mk` + design-system durations; honor `prefers-reduced-motion`. Icons are the local `Icon`/`UiIcon`, sizes as siblings use.
- **Tests:** vitest `include: ["test/**"]` — a test placed under `src/` silently doesn't run. Web tests live in `apps/web/test/**`, mirroring the source path.
- **Never `git add -A`** — stage explicit paths (the repo has many untracked scratch files).
- Full green bar per task: `pnpm exec tsc --noEmit` (exit 0) + `pnpm exec vitest run` (all pass).

---

## File Structure

- `apps/web/src/workspace/blocks/mockData.ts` — `BlockKey` union gains `"forming"` (Task 1).
- `apps/web/src/workspace/Icon.tsx` — `BLOCK_META` becomes 5 entries + a `forming` icon case (Task 1).
- `apps/web/src/workspace/studioResume.ts` — `openToolToRoom` + a new `roomForResume(state)` helper mapping `plan`→forming/board by stage (Task 1).
- `apps/web/src/workspace/blocks/PlanBlock.tsx` — phase becomes a controlled `phase` prop; internal `phase`/`setPhase`/`hasBoard`/`onBackToBoard` removed; Tier-1 chrome deleted (Tasks 1, 4).
- `apps/web/src/workspace/WorkspaceContainer.tsx` — 5-segment switcher mount, forming/board dispatch, `widthTier` morphing layout, reading joins the constant rail (Tasks 1, 2, 3).
- `apps/web/src/studio/reading/ReadingRoom.tsx` + `apps/web/src/workspace/blocks/ReadingBlock.tsx` — FloatingCoach removed; reading uses the constant 印记 rail (Task 3).
- `apps/web/src/workspace/blocks/WritingBlock.tsx`, `ReviewBlock.tsx`, `WritingReferencePanel.tsx` — Tier-1 nav chrome + copy deleted (Task 5).
- `apps/web/src/workspace/blocks/NextStepGuide.tsx` (deleted), `PlanSpine.tsx` (tap-to-jump removed) (Task 6).

---

## Task 1: 5-segment switcher + 提案/管理 split (PlanBlock phase becomes controlled)

**Files:**
- Modify: `apps/web/src/workspace/blocks/mockData.ts:7`
- Modify: `apps/web/src/workspace/Icon.tsx:39-45,97-102`
- Modify: `apps/web/src/workspace/studioResume.ts`
- Modify: `apps/web/src/workspace/blocks/PlanBlock.tsx:78-250` (phase → prop)
- Modify: `apps/web/src/workspace/WorkspaceContainer.tsx` (mount + resume + switcher)
- Test: `apps/web/test/workspace/WorkspaceContainer.test.tsx`, `apps/web/test/workspace/studioResume.test.ts` (new)

**Interfaces:**
- Produces: `BlockKey = "forming" | "plan" | "reading" | "writing" | "reflection"` (`plan` = 管理/board; `forming` = 提案). `roomForResume(state: StudioState): BlockKey`. `PlanBlock` now takes `phase: "forming" | "working"` and `onPlanGenerated: () => void` (flip room to 管理 after generate); it no longer self-decides phase.
- Consumes: existing `StudioState` (`stage`, `openTool`), `handleManualRoom`, `applyStudioState`.

- [ ] **Step 1: Widen the `BlockKey` union.** In `mockData.ts:7`:

```ts
export type BlockKey = "forming" | "plan" | "reading" | "writing" | "reflection";
```

- [ ] **Step 2: 5 switcher segments + a forming icon.** In `Icon.tsx`, add a `case "forming":` to the `Icon` switch (a simple document/pencil-on-page glyph, matching the existing 1.7-stroke style), e.g.:

```tsx
case "forming":
  return (
    <svg {...common}>
      <path d="M6 3h9l4 4v14H6z" />
      <path d="M14 3v5h5M9 13h6M9 17h4" />
    </svg>
  );
```

Then replace `BLOCK_META` (`Icon.tsx:97-102`) with five entries — note `立项` splits into `提案`+`管理`:

```ts
export const BLOCK_META: { key: BlockKey; label: string; sub: string }[] = [
  { key: "forming", label: "提案", sub: "Proposal" },
  { key: "plan", label: "管理", sub: "Plan" },
  { key: "reading", label: "阅读", sub: "Read" },
  { key: "writing", label: "写作", sub: "Write" },
  { key: "reflection", label: "回顾", sub: "Review" },
];
```

- [ ] **Step 3: Write the failing test for `roomForResume`.** Create `apps/web/test/workspace/studioResume.test.ts`:

```ts
import { describe, it, expect } from "vitest";
import { roomForResume } from "@/workspace/studioResume";

const base = { openTool: "plan", widthTier: "half", reference: [], updatedAtTurn: 0 } as const;

describe("roomForResume", () => {
  it("proposal_forming resumes into 提案 (forming)", () => {
    expect(roomForResume({ ...base, stage: "proposal_forming" } as any)).toBe("forming");
  });
  it("topic_discussion resumes into 提案 (forming)", () => {
    expect(roomForResume({ ...base, stage: "topic_discussion" } as any)).toBe("forming");
  });
  it("plan_generation and later resume into 管理 (plan board)", () => {
    expect(roomForResume({ ...base, stage: "plan_generation" } as any)).toBe("plan");
    expect(roomForResume({ ...base, stage: "body_writing", openTool: "writing" } as any)).toBe("writing");
  });
  it("reading/writing/reflection openTools win regardless of stage", () => {
    expect(roomForResume({ ...base, stage: "proposal_forming", openTool: "reading" } as any)).toBe("reading");
  });
});
```

Run: `pnpm exec vitest run test/workspace/studioResume.test.ts` — Expected: FAIL (`roomForResume` not exported).

- [ ] **Step 4: Implement `roomForResume`.** In `studioResume.ts`, keep `openToolToRoom` as-is and add:

```ts
import type { StudioState } from "@mind-imprint/contracts";

/**
 * Resume mapping (P2a): 印记 only emits `plan`/`chat` for the proposal side
 * (the explicit `forming` openTool is P2b). So when the directive names the plan
 * tool, choose 提案(forming) vs 管理(board) from the STAGE — a real signal, not a
 * heuristic. Any non-plan room openTool wins directly.
 */
export function roomForResume(state: StudioState): BlockKey {
  if (state.openTool !== "plan" && state.openTool !== "chat") {
    return openToolToRoom(state.openTool);
  }
  return state.stage === "topic_discussion" || state.stage === "proposal_forming"
    ? "forming"
    : "plan";
}
```

Run the test — Expected: PASS.

- [ ] **Step 5: Make PlanBlock's phase controlled.** In `PlanBlock.tsx`:
  - Add `phase: "forming" | "working"` and `onPlanGenerated: () => void` to the props type (`:78-97`). Remove `onOpenRoom` if now unused inside (it's replaced by `onPlanGenerated` for the generate→board flip; keep any other onOpenRoom uses — grep first).
  - Delete the internal phase state (`:101` `const [phase, setPhase]`) and `hasBoard` (`:105`); read `phase` from props instead.
  - In `doGenerate` success (`:172-173`), replace `setHasBoard(true); setPhase("working");` with `props.onPlanGenerated();`.
  - In the `phase === "forming"` branch (`:230`), drop the `onBackToBoard={hasBoard ? ... : undefined}` prop (the switcher is now the way back to 管理). Remove `onBackToBoard` from `FormingPhase`'s props + its back-button render (`FormingPhase` `:400-402`).

- [ ] **Step 6: Mount forming/board by room + wire the 5-segment switcher.** In `WorkspaceContainer.tsx`:
  - The load effect + `applyStudioState` must land on the split room. Change `applyStudioState` (`:258-264`) to use `roomForResume`:

```ts
const applyStudioState = useCallback((state: StudioState) => {
  setStudioState(state);
  setTookOver(false);
  if (state.openTool !== "chat") setRoom(roomForResume(state));
}, []);
```

  (import `roomForResume` alongside `openToolToRoom`; the interim `setRoom("plan")` default in the load effect at `:419` stays valid.)
  - The room render block (`:713-725`): mount PlanBlock for BOTH forming and board:

```tsx
{(room === "forming" || room === "plan") && (
  <PlanBlock
    key={projectId}
    projectId={projectId}
    title={workspace.title}
    qualification={workspace.qualification}
    proposal={workspace.proposal}
    createdAt={workspace.createdAt}
    phase={room === "forming" ? "forming" : "working"}
    onPlanGenerated={() => handleManualRoom("plan")}
    refreshWorkspace={refreshWorkspace}
    recap={summary}
  />
)}
```

  - The `Segmented` at `:653-658` already maps `BLOCK_META` → now renders all five automatically. No change needed beyond BLOCK_META. Verify `handleManualRoom("forming")` works (it sets `room="forming"` + `tookOver`).

- [ ] **Step 7: Update `WorkspaceContainer.test.tsx` for the split.** The existing chat-first test seeds `openTool:"plan"` with a `plan_generation` stage default (`fakeStudioState`), so it still lands on the board (`room="plan"`). Add a case asserting a `proposal_forming` project resumes into forming (the PlanBlock mock renders `data-testid="plan-block"` for both; assert it mounts and that `getStudioState` drove `stage: "proposal_forming"`). Keep the mock's single `plan-block` testid (forming and board share the component). Run the full file green.

- [ ] **Step 8: Typecheck + full suite.** `pnpm exec tsc --noEmit` (0) and `pnpm exec vitest run` (all pass). Fix any block whose test mocked PlanBlock's old props.

- [ ] **Step 9: Commit.**

```bash
git add apps/web/src/workspace/blocks/mockData.ts apps/web/src/workspace/Icon.tsx apps/web/src/workspace/studioResume.ts apps/web/src/workspace/blocks/PlanBlock.tsx apps/web/src/workspace/WorkspaceContainer.tsx apps/web/test/workspace/studioResume.test.ts apps/web/test/workspace/WorkspaceContainer.test.tsx
git commit -m "feat(studio): 5-segment switcher — split 立项 into 提案 + 管理 (P2a)"
```

---

## Task 2: Morphing width — interactive area follows `widthTier`

**Files:**
- Modify: `apps/web/src/workspace/WorkspaceContainer.tsx` (the `<main>` / AiPanel layout)
- Test: `apps/web/test/workspace/WorkspaceContainer.test.tsx`

**Interfaces:**
- Consumes: `studioState.widthTier` (`"chat" | "half" | "wide"`), `tookOver`, `room`.
- Produces: a layout where — `chat` → the 印记 chat fills the full content width and there is NO interactive area; `half` → chat as a column beside a ~half-width interactive area; `wide` → interactive area fills, chat collapses to a slim rail. A manual takeover (`tookOver`) forces at least `wide` (a room is showing).

**Design note (read `spec §3`):** widthTier is 印记's stored intent. Today's P1 layout renders a *calm landing* in `<main>` plus the chat in a fixed side panel — that is NOT the spec's "chat 全宽，无交互区". This task makes the chat itself the full-width surface in the `chat` tier, and the interactive area appear (half/wide) only when 印记 has opened a tool. Reuse the existing `AiPanel` + `aiSlotEl` portal; drive the panel's flex-basis from an effective tier.

- [ ] **Step 1: Derive the effective tier.** In `WorkspaceContainer.tsx`, above the return, compute:

```ts
// Morphing width (spec §3): 印记's widthTier drives the split between the 印记
// chat and the interactive area. A manual takeover always shows a room, so it
// implies at least `wide`. Null status (still loading) = chat.
const widthTier: WidthTier = tookOver
  ? "wide"
  : (studioState?.widthTier ?? "chat");
const chatOnly = widthTier === "chat" && !tookOver;
```

(Import `WidthTier` from `@mind-imprint/contracts`.) Note `showChatFirst` (`:612`) already means "no room mounted"; align it so `chatOnly` ⇒ chat-first render, and `half`/`wide` ⇒ a room.

- [ ] **Step 2: Width classes per tier.** Replace the fixed AiPanel + `flex-1` main with a tier-driven split. The 印记 chat column basis and the interactive visibility come from `widthTier`:
  - `chat`: interactive area hidden (`<main>` not rendered / `w-0`); chat container spans the full content width, comfortably max-width-centered (e.g. `mx-auto max-w-3xl`).
  - `half`: chat column `basis-[42%]` (or `flex-1` capped) + interactive `basis-[58%]`.
  - `wide`: chat collapses to the slim rail (reuse `aiCollapsed`-style narrow width, e.g. `w-[64px]` rail or the existing collapsed AiPanel) + interactive `flex-1`.

  Add `transition-[flex-basis,width] duration-[240ms] ease-mk` to the morphing containers, guarded by `motion-reduce:transition-none`. Keep the `aiSide` left/right flip working (the chat column can sit on either side).

- [ ] **Step 3: Chat-only fills width.** When `chatOnly`, render `<StudioCoachChat recap={summary} />` in a full-width centered container INSTEAD of the current `ChatFirstLanding`-in-main + side-panel split. (The calm `ChatFirstLanding` placeholder is no longer needed as a main-filler; if you keep it, it must not sit *beside* the chat — the chat is the surface.) Ensure exactly one `StudioCoachChat` mounts (no double portal).

- [ ] **Step 4: Tests.** In `WorkspaceContainer.test.tsx`:
  - `chat` tier (seed `fakeStudioState("chat")` with `widthTier:"chat"`): assert the composer (`findByPlaceholderText(/和印记说说你的项目/)` or the coach placeholder) is present and NO room testid is shown.
  - `half`/`wide` tier (seed `widthTier:"half"`/`"wide"` + a room openTool): assert the room testid shows AND the chat composer is still present (rail).
  - A manual `handleManualRoom` in chat status still shows the room (tookOver ⇒ wide).
  Run the file green. (These assert presence/mounting, not pixel widths — layout classes aren't queryable in jsdom.)

- [ ] **Step 5: Typecheck + full suite green. Commit.**

```bash
git add apps/web/src/workspace/WorkspaceContainer.tsx apps/web/test/workspace/WorkspaceContainer.test.tsx
git commit -m "feat(studio): morphing width — chat full → half → wide slim rail (P2a)"
```

---

## Task 3: Reading room joins the constant 印记 rail (FloatingCoach removed)

**Files:**
- Modify: `apps/web/src/workspace/WorkspaceContainer.tsx:621` (`showAiPanel`) + the reading swap
- Modify: `apps/web/src/workspace/blocks/ReadingBlock.tsx:505-512,1552-1576` (FloatingCoach mount + body)
- Modify: `apps/web/src/studio/reading/ReadingRoom.tsx` (if it owns a separate coach column)
- Test: `apps/web/test/workspace/WorkspaceContainer.test.tsx`

**Interfaces:**
- Consumes: the container-owned `StudioCoachChat` + `aiSlotEl` portal, `useStudioChat()`.
- Produces: the reading room renders its WORK in `<main>` and portals nothing of its own coach — 印记 is the one constant rail, same as plan/writing/reflection.

- [ ] **Step 1: Delete FloatingCoach.** Remove the `问印记 · 找资料` floating widget in `ReadingBlock.tsx` — its mount (`:505-512`) and body (`:1552-1576`), plus any now-orphaned state/handlers it owned (grep for the widget's open/close state and remove). Remove the `印记 · 找资料` header string.
- [ ] **Step 2: Let the constant panel show for reading.** In `WorkspaceContainer.tsx:621`, change `showAiPanel = showChatFirst ? true : room !== "reading"` to `showAiPanel = true` for the room path (the reading exception no longer applies — reading now portals into the constant rail like every other room). Verify the full-screen reading *swap* (`readingSource != null` → `<ReadingRoom>` at `:582`) still renders its own surface; if `ReadingRoom` had its OWN coach column, replace it with the shared `StudioCoachChat` via the same portal contract, or route it through the constant rail. (Read `ReadingRoom.tsx` to confirm; if it already delegates to `useStudioChat`, only the ReadingBlock FloatingCoach removal is needed.)
- [ ] **Step 3: The `问印记·找资料` / `和印记聊聊` copy inside reading/exploration** — remove any remaining "open the coach" copy that pointed at the now-deleted FloatingCoach.
- [ ] **Step 4: Tests.** Update/extend the reading-related tests: opening the reading room shows the room work AND the constant 印记 chat (no separate FloatingCoach). Assert the FloatingCoach testid/label is gone. Run green.
- [ ] **Step 5: Typecheck + full suite green. Commit.**

```bash
git add apps/web/src/workspace/WorkspaceContainer.tsx apps/web/src/workspace/blocks/ReadingBlock.tsx apps/web/src/studio/reading/ReadingRoom.tsx apps/web/test/workspace/WorkspaceContainer.test.tsx
git commit -m "feat(studio): reading room joins the one constant 印记 rail; drop FloatingCoach (P2a)"
```

---

## Task 4: Tier-1 chrome-strip — PlanBlock (提案/管理)

**Files:**
- Modify: `apps/web/src/workspace/blocks/PlanBlock.tsx`
- Test: any PlanBlock test asserting a killed affordance

**Do NOT touch** `生成项目计划` / the regen modal (`:322-341,433`) — Tier-2, stays for P2b. Delete only:

- [ ] **Step 1: `聊聊计划`** toolbar button (`:797-799`, `onClick={onReopen}`) — remove the button and, if `onReopen`/`onReemit` is now unused, its prop + plumbing.
- [ ] **Step 2: `查看我的题目` / `收起` panel** (`:766-775`) — the toggle + its revealed objective/reason body, and its open/close state.
- [ ] **Step 3: `进入 →` doorway + nav copy** — the per-card `进入 →` button (`:881`) and the instruction copy "…点任务卡查看或修改，点「进入 →」去对应房间。" (`:782`). Plan cards stay (view/edit the artifact); only the room-jump doorway + copy go.
- [ ] **Step 4: `写开题报告（可选）`** (`:454-460`) — 印记 cues 写提案 in chat.
- [ ] **Step 5: 中/EN switcher** — the `Segmented` at `:473-482`, the `lang` state (`:118`), `onToggleLang` plumbing (`:242-247`), and the `lang === "en" ? ... : ...` reply-language branches in `onSend`/`onGuideMe`/`onReview` (`:208,216-218,226`). Replies are Chinese; drop the `(reply in English)` suffixes. (The essay's target-language project setting in `CreateProjectDrawer` is unrelated — leave it.)
- [ ] **Step 6: `onBackToBoard`** — already removed in Task 1; confirm no dangling reference.
- [ ] **Step 7: Update tests** that asserted any killed control; ensure `让印记看看我的开题` (`onReview`, `:224`) STAYS. Full suite green.
- [ ] **Step 8: Commit.**

```bash
git add apps/web/src/workspace/blocks/PlanBlock.tsx apps/web/test
git commit -m "strip(studio): Tier-1 redundant chrome in PlanBlock — 聊聊计划/查看题目/进入→/写开题/中EN (P2a)"
```

---

## Task 5: Tier-1 chrome-strip — Writing / Review / WritingReferencePanel

**Files:**
- Modify: `apps/web/src/workspace/blocks/WritingBlock.tsx`, `ReviewBlock.tsx`, `WritingReferencePanel.tsx`
- Test: their block tests

Delete (per sweep + spec §7.5 rulings):

- [ ] **Step 1: WritingBlock header nav** — `看开题 →` (`:213`), `已归档 · 看回顾 →` (`:215`), `去回顾 →` (`:220`). Room-jumps 印记 cues. Keep `重新打开写作` (`:219`, artifact-state).
- [ ] **Step 2: `完成写作` — keep the lock, drop the routing.** Keep the `完成写作` milestone button + its lock action (`:223`, confirm modal `:279-306`), but change its post-confirm "去回顾 →" routing (`完成写作，去回顾 →` at `:284`+) so it only locks the draft — 印记 cues 回顾. (Remove the room-navigation call from the confirm handler; keep the finish/lock API call.)
- [ ] **Step 3: `和印记聊聊哪里还站不住` copy** (`:813`) — the chat is always present.
- [ ] **Step 4: ReviewBlock lock hint + `去写作房间 →`** (`:198,200-206`) — 印记 gates/cues this. Keep the finalize buttons `我写完了我的反思` (`:265`), `定稿并开始评估` (`:284`), and `回到全部项目 →` (`:252`, top-level nav).
- [ ] **Step 5: WritingReferencePanel empty-state copy** (`:82`) — "提案要点还没成形。回到「立项」和印记聊聊…" → a neutral empty state (no "回到「立项」" nav, no "和印记聊聊").
- [ ] **Step 6: Update tests. Full suite green. Commit.**

```bash
git add apps/web/src/workspace/blocks/WritingBlock.tsx apps/web/src/workspace/blocks/ReviewBlock.tsx apps/web/src/workspace/blocks/WritingReferencePanel.tsx apps/web/test
git commit -m "strip(studio): Tier-1 nav chrome in Writing/Review/WritingRef; keep lock+finalize (P2a)"
```

---

## Task 6: Remove NextStepGuide + PlanSpine tap-to-jump

**Files:**
- Delete: `apps/web/src/workspace/blocks/NextStepGuide.tsx`
- Modify: `apps/web/src/workspace/WorkspaceContainer.tsx:662-668` (NextStepGuide mount), `:661` (PlanSpine)
- Modify: `apps/web/src/workspace/blocks/PlanSpine.tsx` (drop the onOpenPlan jump)
- Test: any test referencing NextStepGuide / PlanSpine tap

- [ ] **Step 1: Delete `NextStepGuide.tsx`** and its mount in `WorkspaceContainer.tsx` (`:663-668`) — the deterministic "下一步…去写作/去回顾 →" nudge is exactly the transition 印记 cues in chat. Remove the import.
- [ ] **Step 2: PlanSpine stays as a read-only indicator, loses the tap-to-jump.** In `PlanSpine.tsx`, remove the `onOpenPlan` click handler / make the pill non-interactive (keep the "你在这一步" display). Update its mount (`WorkspaceContainer.tsx:661`) to drop the `onOpenPlan={() => handleManualRoom("plan")}` prop.
- [ ] **Step 3: Delete any test that rendered NextStepGuide or asserted the PlanSpine jump.** Full suite green.
- [ ] **Step 4: Commit.**

```bash
git add apps/web/src/workspace/blocks/NextStepGuide.tsx apps/web/src/workspace/blocks/PlanSpine.tsx apps/web/src/workspace/WorkspaceContainer.tsx apps/web/test
git commit -m "strip(studio): remove NextStepGuide; PlanSpine read-only, no tap-jump (P2a)"
```

---

## Self-Review Checklist (run after all tasks)

- **Spec coverage:** morphing width (T2) ✓ · 5-segment switcher + 提案/管理 split (T1) ✓ · Tier-1 kill-list — nav copy/进入→/聊聊计划/FloatingCoach/中EN/NextStepGuide/完成写作-route/PlanSpine-tap (T3–T6) ✓ · reading joins constant rail (T3) ✓. Tier-2 (生成计划, question boxes) correctly deferred to P2b.
- **Kept-affordance audit:** `让印记看看我的开题`, plan adjust/export, writing select-to-ask + 请印记体检, `完成写作` lock, review finalize — all still present.
- **No backend/contract change** touched. `openTool` enum unchanged.
- **Design gotchas:** no `bg-mk-<hex>/<opacity>`; transitions carry `motion-reduce` guards; one class per property.
- **Green:** `pnpm exec tsc --noEmit` (0) + `pnpm exec vitest run` (all pass) on the final commit.
- **Live-verify (before deploy):** open Phoebe — proposal-stage project lands in 提案 (forming) chat-full; opening a room morphs to wide with a slim 印记 rail; the switcher shows five segments; the killed buttons are gone; 生成计划 still works (unchanged). Then deploy (frontend-only) + P2b.
