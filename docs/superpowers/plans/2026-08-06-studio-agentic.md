# Part 3a · Agentic Studio Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Steps use `- [ ]` checkboxes.

**Goal:** Rebuild the in-project studio into the "agentic" experience the design spec calls for — remove the left sidebar, give the project one **constant, flippable (⇄ left/right), collapsible ~320px AI panel** across all rooms, unify the duplicated chat/composer, and restyle the four rooms + exploration graph + card sheet to the design system.

**Architecture:** `WorkspaceContainer` stops rendering the `Rail` sidebar. Instead it renders constant studio chrome: a top bar (「← 主页」capsule + project title + a room switcher) and a persistent `AiPanel` (flip/collapse state at the container level, persisted to localStorage). The AiPanel is a **chrome shell with a content slot**; each room provides its coach content into that slot and its work content into the main area — so coach LOGIC per room is preserved (low risk), only its position/chrome is unified. A shared `ChatLog` + `Composer` replace the 4 near-duplicate chat implementations. The graph already has L2 titles + curved edges (verified) — it needs only a token/canvas restyle.

**Tech Stack:** React + Vite + TS + Tailwind + the `apps/web/src/ui/` design-system library. Tests: Vitest/jsdom (`apps/web/test/**`). No backend changes.

## Global Constraints

- **Consume Part 1 `ui/`** primitives + tokens; **no hardcoded hex** in new/rewritten code (except spec-sanctioned literals). Colors via `mk-*`/CSS vars. Copy `cx` from `ui/Card.tsx`.
- **Tailwind rule:** exactly ONE class per CSS property per element (same-property utils emit alphabetically). **Keyframes:** prefix `mk-*` (globals shared).
- **Preserve ALL studio behavior/logic:** coach turn loops, summon-card flow, reading loop, writing/draft autosave, exploration dig, review reflection, all API calls. This is a repositioning+restyle, NOT a logic rewrite. Any test that passed before must pass after.
- **Scope = student studio.** No console. No course-player (that's Part 3b). No backend. **No deploy** (Part 4).
- **The AI panel is CONSTANT:** same side/width/collapse behavior across rooms; only its content changes per room. Flip ⇄ and collapse states persist (localStorage). Spec §17: ~320px, no mobile, no dark mode.
- **Reading room** (`ReadingRoom.tsx`/`.css`, BEM system) migrates to tokens; it's a distinct full-screen coach|reading surface — keep its two-pane split but restyle to tokens and align its coach column to the AiPanel look.
- Web verify per task: `npm --prefix apps/web test` (FULL suite — this touches shared shell), `npm --prefix apps/web run typecheck`, `npm --prefix apps/web run build`.

---

## File Structure

- `apps/web/src/studio/ai/ChatLog.tsx` — **Create:** shared message-log (bubbles + inline CardTurnChip/CoachProposal/CoachLinkOffer).
- `apps/web/src/studio/ai/Composer.tsx` — **Create:** shared 3-state composer.
- `apps/web/src/studio/ai/AiPanel.tsx` — **Create:** flippable/collapsible chrome shell + content slot.
- `apps/web/src/studio/ai/SummonShelf.tsx` — **Create/adapt** from `workspace/blocks/CoachCardPanel.tsx` (restyle the summon deck).
- `apps/web/src/workspace/WorkspaceContainer.tsx` — **Modify:** remove `Rail`, add top bar + room switcher + constant `AiPanel`.
- `apps/web/src/workspace/blocks/PlanBlock.tsx`, `ReviewBlock.tsx`, `WritingBlock.tsx` — **Modify:** split into work-area + coach-slot; restyle to `ui/`.
- `apps/web/src/studio/reading/ReadingRoom.tsx` + `ReadingRoom.css` + `HangingCard.tsx` — **Modify:** BEM→token restyle.
- `apps/web/src/workspace/blocks/MaterialsSidebar.tsx` — **Modify:** restyle (keep behavior).
- `apps/web/src/workspace/blocks/exploration/warrenLayout.ts` + `WarrenMap.tsx` + `QuestionMindmap.tsx` + `ExplorationView.tsx`/`ExplorationSidebar.tsx` — **Modify:** canvas warm-paper + dot grid; theme hex → macaron token array.
- `apps/web/src/studio/StudioCardSheet.tsx`, `studio/Bean.tsx`, `primitives/{annotate,compare,graph,matrix,scale,sort}/*.tsx` — **Modify:** token restyle.
- `apps/web/src/workspace/blocks/{CoachProposal,CoachLinkOffer,CardTurnChip}.tsx` — **Modify:** restyle (used by ChatLog).
- Tests under `apps/web/test/studio/**`, `apps/web/test/workspace/**`.

---

### Task 1: Shared ChatLog + Composer

**Files:**
- Create: `apps/web/src/studio/ai/ChatLog.tsx`, `apps/web/src/studio/ai/Composer.tsx`
- Test: `apps/web/test/studio/ai/ChatLog.test.tsx`, `Composer.test.tsx`

**Interfaces (spec §13):**
- `ChatMessage = { id: string; role: "assistant"|"student"|"system"; text?: string; node?: ReactNode }` (node lets a room inject a CardTurnChip/CoachProposal/CoachLinkOffer inline).
- `ChatLog({ messages, thinking?, className })` — scrollable log; assistant = white bubble (radius 4/13/13/13); student = `bg-mk-accent-50` (radius 13/4/13/13); system = centered `text-mk-caption text-mk-muted`; `thinking` shows the "印记正在打字" 3-dot indicator (reuse `.mk-think-dot`). Auto-scrolls to bottom on new messages. `mk-scroll`.
- `Composer({ value, onChange, onSend, state?, onStop?, placeholder? })` — `state: "empty"|"typing"|"replying"` (default derived: empty when !value). Empty → send disabled (`#E7DDD0`); typing → accent send; replying → ■ stop button (calls `onStop`). Focus → accent ring. Textarea auto-grows (cap ~5 lines). Enter sends (Shift+Enter newline).

- [ ] **Step 1: Failing tests** — ChatLog renders assistant/student/system messages with the right bubble class + renders a `node`; `thinking` shows the dot indicator. Composer: typing enables send + Enter calls `onSend`; empty disables send; `replying` shows stop and calls `onStop`; Shift+Enter does not send.
- [ ] **Step 2: Run → FAIL.**
- [ ] **Step 3: Implement** both, `ui/` tokens only.
- [ ] **Step 4: Run → PASS; typecheck; build.**
- [ ] **Step 5: Commit** — `git commit -m "feat(studio): shared ChatLog + Composer"`

---

### Task 2: AiPanel chrome shell

**Files:**
- Create: `apps/web/src/studio/ai/AiPanel.tsx`
- Test: `apps/web/test/studio/ai/AiPanel.test.tsx`

**Interfaces (spec §17):**
- `AiPanel({ side, onFlip, collapsed, onToggleCollapse, title?, children })` — a persistent side column, width `w-[320px]` when expanded, collapsed = a slim `w-[48px]` strip showing a 印记 `Pebble` + an expand affordance. Header: a `Pebble size={24}` + `title` (default "印记") + a **flip button** (⇄ icon → `onFlip`, swaps side) + a **collapse button** (chevron → `onToggleCollapse`). Body = `children` (the room's coach content). Border on the inner edge (`border-mk-border`), `bg-mk-surface`. The side (left/right) only affects which border edge + control orientation — the PARENT decides DOM order; this component just renders the column and reports flip/collapse intents.
- Persisted state lives in the PARENT (WorkspaceContainer, Task 4). This component is controlled.

- [ ] **Step 1: Failing test** — expanded AiPanel shows the title + children + flip + collapse buttons; clicking flip fires `onFlip`, collapse fires `onToggleCollapse`; when `collapsed`, children are hidden and only the strip + expand control render (clicking it fires `onToggleCollapse`).
- [ ] **Step 2: Run → FAIL.**
- [ ] **Step 3: Implement.** Respect `prefers-reduced-motion` for the collapse transition.
- [ ] **Step 4: Run → PASS; typecheck; build.**
- [ ] **Step 5: Commit** — `git commit -m "feat(studio): AiPanel flippable/collapsible chrome shell"`

---

### Task 3: SummonShelf (restyle CoachCardPanel deck)

**Files:**
- Create: `apps/web/src/studio/ai/SummonShelf.tsx` (or refactor `workspace/blocks/CoachCardPanel.tsx` in place and re-export)
- Modify: `workspace/blocks/CoachProposal.tsx`, `CoachLinkOffer.tsx`, `CardTurnChip.tsx` (restyle to `ui/` tokens)
- Test: update/add.

**Interfaces (spec §13):** the summon-card **chip shelf** (horizontal deck of tool-card chips the student can open; opening is student-confirmed — 铁律②) + the AI-proposed-card chip (`CoachProposal`) + link chip (`CoachLinkOffer`) + completed-turn chip (`CardTurnChip`). Preserve their existing props/behavior; restyle to tokens (macaron-tinted chips, radius-sm). Keep `FORMING_DECK`/`READING_DECK`/`THINKING_DECK` exports.

- [ ] **Steps:** restyle → tests pass → commit `git commit -m "feat(studio): restyle summon shelf + coach chips to tokens"`.

---

### Task 4: Studio shell rewire — remove Rail, constant AiPanel

**Files:**
- Modify: `apps/web/src/workspace/WorkspaceContainer.tsx` (remove inline `Rail`; add top bar + room switcher + constant AiPanel)
- Test: `apps/web/test/workspace/WorkspaceContainer.test.tsx` (or a new studio-shell test)

**Interfaces (spec §17):**
- Studio layout after `openProject`:
  ```
  <div class="flex h-full flex-col">
    <TopBar/>                 // ← 主页 capsule + project title/qual + room switcher (Segmented: 立项/阅读/写作/回顾)
    <div class="flex flex-1 min-h-0">
      {side==='left' ? <AiPanel/> : null}
      <main class="flex-1 min-w-0">{roomWork}</main>
      {side==='right' ? <AiPanel/> : null}
    </div>
  </div>
  ```
- **State at this level (persisted to localStorage):** `aiSide: 'left'|'right'` (default `'right'`, key `mk-studio-ai-side`), `aiCollapsed: boolean` (key `mk-studio-ai-collapsed`). Replaces the old `mk-workspace-fullscreen`.
- **Room switching** stays local state (`room`), now driven by the top-bar Segmented instead of the Rail.
- **「← 主页」capsule** → `onBack`/`backToAll()` (unchanged behavior: returns to Directory).
- **Room content split:** each room component is refactored (Tasks 5–8) to expose `work` (main area) and `coach` (panel slot). In THIS task, introduce the contract: rooms render via a `StudioRoom` shape or WorkspaceContainer renders `<ActiveRoom renderInPanel={(coach)=>...} />`. Simplest concrete contract: each room component accepts a prop `mountCoach: (node: ReactNode) => ReactNode` OR returns both slots. Pick the lowest-churn approach — RECOMMENDED: WorkspaceContainer renders `<AiPanel>{coachSlot}</AiPanel>` and passes a `setCoach`/portal target; but to avoid portal complexity, refactor each room to `Room({ ...props, coachSlot })` where the room renders its work directly and calls a passed `renderCoach` — decide during implementation and document it. For THIS task, wire it for ONE room (plan) as the reference; Tasks 5–8 convert the rest.
- The `Rail` component + `mk-workspace-fullscreen` are removed.

- [ ] **Step 1: Failing test** — opening a project renders the top bar with the 主页 capsule + room switcher (4 rooms) + a constant AiPanel; clicking flip swaps the panel side (assert DOM order or a side attribute); collapse hides the panel body; the 主页 capsule fires back-to-directory; switching rooms via the Segmented changes the room.
- [ ] **Step 2: Run → FAIL.**
- [ ] **Step 3: Implement.** Keep the plan room working through the new shell as the reference integration. Persist side/collapse.
- [ ] **Step 4: Run FULL suite + typecheck + build green.**
- [ ] **Step 5: Commit** — `git commit -m "feat(studio): remove sidebar, add top bar + constant flippable AiPanel"`

---

### Task 5: Plan room → new shell + restyle

**Files:** Modify `apps/web/src/workspace/blocks/PlanBlock.tsx`. Test: update.

Refactor `PlanBlock` to the room contract from Task 4: work area = the 立项/proposal + 计划板 sub-view; coach content (its chat log + proposal dims + summon shelf) → the AiPanel slot via `ChatLog`/`Composer`/`SummonShelf`. Remove its bespoke 380px aside + inline chat/composer (use the shared ones). Restyle to tokens. Preserve the turn loop + `submitFraming`/proposal logic.

- [ ] **Steps:** refactor+restyle → FULL suite + typecheck + build → commit `git commit -m "feat(studio): plan room on shared AiPanel + restyle"`.

---

### Task 6: Review room → new shell + restyle

**Files:** Modify `apps/web/src/workspace/blocks/ReviewBlock.tsx`. Test: update.

Same contract: work = reflection prompts + goal-anchor callout; coach → AiPanel slot (shared ChatLog/Composer/SummonShelf). Preserve `submitReflection`/anchor logic. Restyle.

- [ ] **Steps:** → FULL suite/typecheck/build → commit `git commit -m "feat(studio): review room on shared AiPanel + restyle"`.

---

### Task 7: Writing room → new shell + restyle (+ MaterialsSidebar)

**Files:** Modify `apps/web/src/workspace/blocks/WritingBlock.tsx` (2058 lines — largest), `MarkdownPreview.tsx`, `MaterialsSidebar.tsx`. Test: update.

Work area = the 大纲/片段/正文 tab bar + editor + MarkdownPreview + the (restyled) MaterialsSidebar; coach (its `CoachRail` inline chat/logic) → AiPanel slot via shared components. Preserve draft autosave, snippet/outline logic, examiner voices, 整稿体检, finish-writing. Restyle to tokens (MarkdownPreview 9 hex, MaterialsSidebar). This is the biggest room — be careful, run the FULL suite.

- [ ] **Steps:** → FULL suite/typecheck/build → commit `git commit -m "feat(studio): writing room on shared AiPanel + restyle"`.

---

### Task 8: Reading room (BEM→token) + HangingCard

**Files:** Modify `apps/web/src/studio/reading/ReadingRoom.tsx` + `ReadingRoom.css` + `HangingCard.tsx`, `ReadingOutcomes.tsx`. Test: update.

Migrate the BEM `mk-reading-room__*` styles + `ReadingRoom.css` to design tokens (keep the two-pane coach|reading grid, but colors/spacing/radii via tokens; align the coach column visually to the AiPanel — the reading room is a distinct full-screen surface, so it may keep its own split rather than the WorkspaceContainer AiPanel, but must LOOK consistent). Restyle `HangingCard` (28 hex, the lens card that hangs under a paragraph) + `ReadingOutcomes` + the inline "我的笔记" hex block. Preserve the read-together loop (hang-lens-on-sentence, you-find-evidence), computeOffsets, finalize.

- [ ] **Steps:** → FULL suite/typecheck/build → commit `git commit -m "feat(studio): reading room token migration + HangingCard restyle"`.

---

### Task 9: Exploration graph — canvas + theme tokens

**Files:** Modify `apps/web/src/workspace/blocks/exploration/warrenLayout.ts`, `WarrenMap.tsx`, `QuestionMindmap.tsx`, `ExplorationSidebar.tsx`. Test: update (React Flow needs the jsdom mock — see existing exploration tests).

- Canvas: `<Background>` → warm paper feel (dot grid `color` = `#E7DDD0` per spec §18, gap ~18, the page/canvas bg = `var(--mk-paper)`).
- `warrenLayout.ts` `NODE_THEMES` (26 hex) → derive from the **7 macaron tokens** (ordinal colors per spec §18: L1 question nodes get macaron ordinal colors; accent reserved for selection). Replace the hardcoded hue palette with a macaron-based theme array; `mixToward` can stay (operates on the token hex values imported from `ui/tokens`). Edge colors: confirmed `EDGE_SOLID`/`EDGE_DASHED` → tokens (confirmed edge = 墨 for accepted, `#C9BCAD`-ish dashed for proposed per spec §18; "印记提议" label already exists). L2 node = white card radius-8, title already shown; selection ring = accent.
- `ExplorationSidebar` (already clean `mk-*`) → align to AiPanel look (it's the exploration coach).

- [ ] **Steps:** → FULL suite/typecheck/build → commit `git commit -m "feat(studio): exploration graph — warm canvas + macaron theme tokens"`.

---

### Task 10: Card sheet + Bean + primitives restyle

**Files:** Modify `apps/web/src/studio/StudioCardSheet.tsx`, `studio/Bean.tsx`, `primitives/{annotate,compare,graph,matrix,scale,sort}/*.tsx`. Test: update.

- `StudioCardSheet` (10 hex, inline CSS-in-JS → tokens; width already ≈ the panel's ~320px). `Bean` (shared by AskPanel + ChatSurface — restyle carefully; 4 hex; keep the luminance eye-contrast logic). Primitives (annotate/compare/graph/matrix/scale/sort — ~119 hex total): migrate each widget's colors to tokens (accent for selection/marks, macaron/semantic where meaningful, neutral chrome). Preserve each widget's interaction + serialize logic.

- [ ] **Steps:** → FULL suite/typecheck/build → commit `git commit -m "feat(studio): card sheet + Bean + card primitives token restyle"`.

---

### Task 11: Studio integration + hex sweep

**Files:** Modify any remaining studio files with stray old-palette hex (`workspace/`, `studio/` — NOT `shell/courses/` which is Part 3b). Test: a studio smoke test.

- Grep `workspace/ studio/` for old-palette hex (`#F3F4F8 #2A3B7A #D98263 #EAECF2 #1C2333 #E8A33D #4C9A82 #9AA1B0 #8A92A3 #EDEFF9 #FBFAFE` etc.) and sweep any stragglers in owned files.
- Confirm the constant AiPanel behaves identically across all 4 rooms (flip/collapse persists; content swaps per room). Smoke test: open a project, switch rooms, flip + collapse the panel, assert persistence.
- FULL suite + typecheck + build green.

- [ ] **Steps:** → commit `git commit -m "feat(studio): integration smoke test + studio hex sweep"`.

---

## Self-Review Notes

- **Spec coverage:** §17 no-sidebar + flippable/collapsible AI (T2/T4); §13 chat bubbles/composer/summon chips (T1/T3); §18 graph (T9 — L2 title + curved already done, only canvas+theme); rooms restyle (T5–T8); card sheet/primitives (T10).
- **Deferred (Part 3b / later):** course player; `studio/material/AddSourceForm`/`SourceLog` possible dead code (confirm before deleting — leave for now); full copy sweep (Part 4).
- **Risk:** T4 is the crux (shell rewire + room-content contract). Wire ONE room (plan) as reference in T4, convert the rest in T5–T8. Run the FULL suite every task (shared shell). The coach state model stays per-room (no logic rewrite) — only its chrome/position unifies.
- **Type consistency:** `ChatMessage` (T1) reused by all rooms (T5–T8); `AiPanel` side/collapsed contract (T2) owned by WorkspaceContainer (T4).
