# Guided Tour — P3 (Projects Tour) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Steps use checkbox (`- [ ]`) syntax.

**Goal:** The projects half of the guided tour — segments that open the shared demo project (`…0200`) and walk a new user through all five studio rooms + the evaluation report, plus the `data-tour` anchors those segments spotlight, plus the nav plumbing so the tour can open the demo and switch rooms.

**Architecture:** Reuse the P1 tour engine (`TourProvider`/`TourRunner`/`useTour`, `TourNavContext`, `[data-tour]` anchors). Extend `TourNavContext` with `openDemoProject()` + `setStudioRoom(room)`. Add a `pendingRoom` prop to `WorkspaceContainer` (mirroring the existing `initialProjectId` one-shot) so the tour can drive room switches. Author `segments/projects.ts` and add it to `fullJourney` (courses group then projects group). Anchors are added across the studio room components. The demo is world-readable (P2), so opening `…0200` works for any new user; opening a project enters immersive mode (nav rail hidden) — the TourRunner overlay persists, so the tour keeps working.

**Tech Stack:** React + Vite + TS; the tour engine from P1; the demo project from P2.

**Spec:** `docs/superpowers/specs/2026-08-22-new-user-guided-tour-design.md` (§7 Segments — Projects group).

## Global Constraints

- Reuse the P1 engine; NO new dependency; tour config stays frontend-local in `apps/web/src/tour`.
- Anchors are `[data-tour="<id>"]`; missing/conditional anchors degrade gracefully (centered bubble) — the demo is FINISHED, so in-progress-only affordances (提问卡 when objective empty, warren 未归类 when count>0, exploration suggestions tray, writing doc-switch when >1 doc) may be absent; those steps use `placement:"center"` or tolerate absence.
- The tour opens the demo project by its hardcoded id `00000000-0000-0000-0000-000000000200` (it is not in any user's project list; it is world-readable).
- Opening a project enters immersive mode (nav rail hidden via `showNav`); the tour must not rely on the nav rail while in the studio.
- mk alpha trap: solid `bg-mk-*` / alpha on real colors only.
- Frontend tests: `npx vitest run <path>`; `npm run typecheck`; full `npm run test` before finishing.
- Git: stage specific files (never `git add -A`); pre-existing untracked `docs/*.md` are not ours; commit per task with trailer `Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>`; controller pushes.
- Boundary hook blocks `/dev/null` redirects (use `2>&1`) + `cd` in compound commands.

---

## File Structure

**Modified (engine/nav):**
- `apps/web/src/tour/types.ts` — extend `TourNavContext` with `openDemoProject: () => void` and `setStudioRoom: (room: StudioRoom) => void` (StudioRoom = "forming"|"plan"|"reading"|"writing"|"reflection"). Add `DEMO_PROJECT_ID` const.
- `apps/web/src/shell/StudentApp.tsx` — implement `openDemoProject` (openProjectFromHome(DEMO_PROJECT_ID)) + `setStudioRoom` (via a new `pendingRoom` state passed to ProjectsTab), in the `tourNav`.
- `apps/web/src/shell/ProjectsTab.tsx` — thread `pendingRoom`/`onPendingRoomConsumed` to WorkspaceContainer.
- `apps/web/src/workspace/WorkspaceContainer.tsx` — accept `pendingRoom` prop; effect calls `handleManualRoom(pendingRoom)` on change (mirrors the `initialProjectId` effect at ~:1052); add `data-tour` anchors (room bar, coach rail).

**Modified (anchors — add `data-tour` only):**
- `apps/web/src/workspace/RoomSwitcher.tsx`, `workspace/blocks/PlanBlock.tsx`, `workspace/blocks/ReadingBlock.tsx`, `workspace/blocks/exploration/WarrenMap.tsx`, `workspace/blocks/exploration/ExplorationSidebar.tsx`, `workspace/blocks/exploration/PaperDetail.tsx`, `studio/reading/ReadingRoom.tsx`, `workspace/blocks/WritingBlock.tsx`, `workspace/blocks/ReferencePanel.tsx`, `workspace/blocks/ReviewBlock.tsx`.

**New (segments):**
- `apps/web/src/tour/segments/projects.ts` — the projects group.
- `apps/web/src/tour/journey.ts` (modify) — `fullJourney = [...coursesSegments, ...projectsSegments]`.
- Tests under `apps/web/test/tour/**` and `apps/web/test/shell/**`.

**Anchor id vocabulary** (added in Task 2, referenced by Task 3):
`room-bar`, `room-tab-forming`/`-plan`/`-reading`/`-writing`/`-reflection`, `coach-rail`,
`forming-proposal`, `forming-question-card` (conditional), `manage-viewtoggle`, `manage-export`, `manage-gantt`,
`reading-viewtoggle`, `warren-question`, `warren-unfiled` (conditional), `explore-keyword`, `explore-suggestions` (conditional), `paper-enter-reading` (conditional), `library-table`,
`rr-chat`, `rr-article`, `rr-deck`, `rr-notes`, `rr-finish`,
`writing-docswitch` (conditional), `writing-tabs`, `writing-aicard`, `writing-refpanel`,
`reflection-prompts`. Eval report reuses `#s1`–`#s9` + `[data-testid="evaluation-report"]`.

---

## Task 1: Nav plumbing — open demo project + drive studio rooms

**Files:**
- Modify: `apps/web/src/tour/types.ts` (TourNavContext + `DEMO_PROJECT_ID` + `StudioRoom`)
- Modify: `apps/web/src/workspace/WorkspaceContainer.tsx` (`pendingRoom` prop + effect; room bar `data-tour="room-bar"`; coach rail `data-tour="coach-rail"`)
- Modify: `apps/web/src/shell/ProjectsTab.tsx` (thread pendingRoom)
- Modify: `apps/web/src/shell/StudentApp.tsx` (tourNav.openDemoProject + setStudioRoom via pendingRoom state)
- Test: `apps/web/test/workspace/pendingRoom.test.tsx`

**Interfaces:**
- Produces: `TourNavContext` gains `openDemoProject()` and `setStudioRoom(room)`; `WorkspaceContainer` accepts `pendingRoom?: BlockKey|null` + `onPendingRoomConsumed?`.

- [ ] **Step 1: Extend the types**

In `apps/web/src/tour/types.ts`: add `export type StudioRoom = "forming" | "plan" | "reading" | "writing" | "reflection";`, `export const DEMO_PROJECT_ID = "00000000-0000-0000-0000-000000000200";`, and to `TourNavContext` add:
```ts
  openDemoProject: () => void;
  setStudioRoom: (room: StudioRoom) => void;
```

- [ ] **Step 2: Add `pendingRoom` to WorkspaceContainer (failing test first)**

Create `apps/web/test/workspace/pendingRoom.test.tsx` mirroring the existing `WorkspaceContainer.test.tsx` mock harness: mount with `initialProjectId` set (so a project opens) and `pendingRoom="reading"`; assert the reading room mounts (spy on `handleManualRoom` effect via a rendered marker, or assert the reading room's content/testid appears). If full mount is impractical, assert at the smallest layer that consumes `pendingRoom`. Look at `WorkspaceContainer.test.tsx` for the harness.

- [ ] **Step 3: Run to verify it fails**

Run: `cd apps/web && npx vitest run test/workspace/pendingRoom.test.tsx`
Expected: FAIL — `pendingRoom` prop not accepted.

- [ ] **Step 4: Implement `pendingRoom`**

In `WorkspaceContainer.tsx`: add `pendingRoom?: BlockKey | null` + `onPendingRoomConsumed?: () => void` to the props (near the `initialProjectId` props ~:110-115). Add an effect (mirror the `initialProjectId` effect ~:1052-1058) that, when `pendingRoom` changes and is set, calls `handleManualRoom(pendingRoom)` (defined ~:401, which sets room + `tookOver=true` so the room mounts even in chat-first) then `onPendingRoomConsumed?.()`. Add `data-tour="room-bar"` to the room-bar wrapper (~:1229) and `data-tour="coach-rail"` to the AI panel slot (~:1143 or the AiPanel wrapper ~:1136).

- [ ] **Step 5: Thread through ProjectsTab + StudentApp**

In `ProjectsTab.tsx`: accept `pendingRoom`/`onPendingRoomConsumed` and pass to `WorkspaceContainer` (beside `initialProjectId`, ~:87). In `StudentApp.tsx`: add `const [pendingRoom, setPendingRoom] = useState<StudioRoom|null>(null)`, pass to ProjectsTab; in `tourNav` add:
```ts
    openDemoProject: () => openProjectFromHome(DEMO_PROJECT_ID),
    setStudioRoom: (room) => setPendingRoom(room),
```
(Clear `pendingRoom` via `onPendingRoomConsumed`.) `openProjectFromHome` already sets `pendingProjectId` + `setTab("projects")`.

- [ ] **Step 6: Run tests + typecheck**

Run: `cd apps/web && npx vitest run test/workspace/pendingRoom.test.tsx && npm run typecheck`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add apps/web/src/tour/types.ts apps/web/src/workspace/WorkspaceContainer.tsx apps/web/src/shell/ProjectsTab.tsx apps/web/src/shell/StudentApp.tsx apps/web/test/workspace/pendingRoom.test.tsx
git commit -m "feat(tour): nav plumbing — open demo project + drive studio rooms" -m "Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 2: `data-tour` anchors across the studio rooms

Add `data-tour` attributes at the exact locations below (anchor surrounding wrappers, never shared primitives). Mechanical.

**Files + exact anchor points** (from the anchor map):
- `RoomSwitcher.tsx`: each tab button (~:25) → `data-tour={`room-tab-${b.key}`}`.
- `PlanBlock.tsx` (forming): proposal card wrapper (~:308) → `forming-proposal`; 提问卡 button (~:357, conditional) → `forming-question-card`. (working): toolbar row (~:619) → `manage-viewtoggle`; 导出 button (~:629) → `manage-export`; GanttView root (~:746) → `manage-gantt`.
- `ReadingBlock.tsx`: view toggle wrapper (~:460) → `reading-viewtoggle`; RefTable call/root (~:485 / :817) → `library-table`.
- `WarrenMap.tsx`: `WarrenNodeView` root (~:91) → `warren-question`; `UnfiledNodeView` root (~:162, conditional) → `warren-unfiled`.
- `ExplorationSidebar.tsx`: keyword box wrapper (~:178) → `explore-keyword`; ResultsPanel aside (~:498, conditional) → `explore-suggestions`.
- `PaperDetail.tsx`: primary-action wrapper (~:128) → `paper-enter-reading`.
- `ReadingRoom.tsx`: chat log (~:494) → `rr-chat`; article (~:696) → `rr-article`; 透镜库 button (~:636) → `rr-deck`; notes wrapper (~:768) → `rr-notes`; 完成这篇 button (~:690) → `rr-finish`.
- `WritingBlock.tsx`: doc-switch wrapper (~:360, conditional) → `writing-docswitch`; tab bar (~:404) → `writing-tabs`; ProposalGuidePane/SnippetsPane (~:444) → `writing-aicard`.
- `ReferencePanel.tsx`: root (~:184) → `writing-refpanel`.
- `ReviewBlock.tsx`: prompts wrapper (~:204) → `reflection-prompts`.

- [ ] **Step 1: Add the anchors**

Add each `data-tour` attribute at the listed location. For per-tab room ids use the template literal. Do not modify shared primitives — anchor the surrounding element.

- [ ] **Step 2: Sanity test**

Add/extend a lightweight test (e.g. `apps/web/test/tour/projectAnchors.test.tsx`) that renders one or two of these components (using existing test harnesses) and asserts a couple of `data-tour` attributes are present (e.g. `reflection-prompts` on ReviewBlock, `writing-tabs` on WritingBlock). Keep minimal — the real proof is the segment run.

- [ ] **Step 3: Run tests + typecheck**

Run: `cd apps/web && npx vitest run test/tour/projectAnchors.test.tsx && npm run typecheck`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add apps/web/src/workspace/RoomSwitcher.tsx apps/web/src/workspace/blocks/PlanBlock.tsx apps/web/src/workspace/blocks/ReadingBlock.tsx apps/web/src/workspace/blocks/exploration/WarrenMap.tsx apps/web/src/workspace/blocks/exploration/ExplorationSidebar.tsx apps/web/src/workspace/blocks/exploration/PaperDetail.tsx apps/web/src/studio/reading/ReadingRoom.tsx apps/web/src/workspace/blocks/WritingBlock.tsx apps/web/src/workspace/blocks/ReferencePanel.tsx apps/web/src/workspace/blocks/ReviewBlock.tsx apps/web/test/tour/projectAnchors.test.tsx
git commit -m "feat(tour): data-tour anchors across the five studio rooms" -m "Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 3: Projects segment config + journey composition

**Files:**
- Create: `apps/web/src/tour/segments/projects.ts`
- Modify: `apps/web/src/tour/journey.ts` (`fullJourney` = courses + projects)
- Test: `apps/web/test/tour/projects-segments.test.ts`

**Interfaces:**
- Consumes: `TourSegment`/`TourNavContext` (`openDemoProject`/`setStudioRoom`), the anchors (Task 2), eval `#s1`–`#s9`.
- Produces: `projectsSegments: TourSegment[]`; `fullJourney` includes them after the courses group.

- [ ] **Step 1: Failing config test**

Create `apps/web/test/tour/projects-segments.test.ts` (mirror `courses-segments.test.ts`): assert unique step ids, non-empty text, valid advance, action steps have actionEvent; assert the first segment's first step calls `openDemoProject` (or `setTab("projects")`); assert `fullJourney` contains both courses and projects segments.

- [ ] **Step 2: Run to verify it fails**

Run: `cd apps/web && npx vitest run test/tour/projects-segments.test.ts`
Expected: FAIL — module not found.

- [ ] **Step 3: Author the segments**

Create `apps/web/src/tour/segments/projects.ts`. Each segment's first step navigates into position (standalone-safe). 印记 voice; one idea per step; show-and-tell (advance:"next") except where a real "you try it" is natural. Segments (from spec §7):
- `projects-intro` — center, `onEnter: nav.setTab("projects")`; AI-guided writing projects; **AI has a borderline** + there's **evaluation**.
- `projects-open-demo` — `onEnter: nav.openDemoProject()`; center; "我先带你打开一个示例项目。" (opens `…0200`).
- `project-rooms-bar` — anchor `[data-tour="room-bar"]`; the 立题-管理-阅读-写作-回顾 five parts.
- `forming` — `onEnter: nav.setStudioRoom("forming")`; anchor `forming-proposal`; then `coach-rail` (AI asks questions / 提问卡 / hints to fill 立题). (提问卡 anchor is conditional — if absent, that step is center.)
- `plan-manage` — `onEnter: setStudioRoom("plan")`; `manage-viewtoggle` (甘特图/看板/日志), `manage-gantt`, `manage-export`.
- `reading-warren` — `onEnter: setStudioRoom("reading")`; `reading-viewtoggle`, `warren-question` (main Q + subquestions), `warren-unfiled` (center if absent), `explore-keyword` (AI keyword suggestions — center note "有时需要粘贴正文，因为 AI 拿不到全文"), `paper-enter-reading`.
- `reading-room` — center (the reading room opens via a source; for the tour, describe it centrally): chat, select-text-to-chat, card/lens, notes, finish. (Anchors `rr-*` only resolve when actually inside the reading room; keep these steps center unless the tour opens a source.)
- `reading-library` — `onEnter: setStudioRoom("reading")`; `library-table` (the 文献库).
- `writing` — `onEnter: setStudioRoom("writing")`; `writing-tabs` (大纲/片段/正文), `writing-aicard` (snippet AI card), `writing-refpanel` (reading notes + 批注).
- `reflection` — `onEnter: setStudioRoom("reflection")`; `reflection-prompts`.
- `evaluation-report` — `onEnter: setStudioRoom("reflection")` then guide to the report; anchor `[data-testid="evaluation-report"]` for the intro, then walk sections `#s1`…`#s9` (or a representative subset — s1 基本信息, s2 综述, s5/s6 depth/autonomy, s9 risks). NOTE: the report renders on the 评估报告 surface, not inside the studio — see the P3-plan open question below; simplest is to spotlight the report when the finished demo shows it, else describe centrally.

Keep copy substantive and 印记-voiced.

- [ ] **Step 4: Compose the journey**

In `apps/web/src/tour/journey.ts`: `import { projectsSegments } from "./segments/projects";` and set `export const fullJourney: TourJourney = [...coursesSegments, ...projectsSegments];` (keep `coursesJourney` as-is).

- [ ] **Step 5: Run to verify it passes**

Run: `cd apps/web && npx vitest run test/tour/projects-segments.test.ts && npm run typecheck`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add apps/web/src/tour/segments/projects.ts apps/web/src/tour/journey.ts apps/web/test/tour/projects-segments.test.ts
git commit -m "feat(tour): projects segment config + full journey composition" -m "Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 4: Integration + how the eval-report step reaches the report

Wire the projects tour end-to-end and resolve how the tour reaches the evaluation report for the demo.

**Files:**
- Modify: `apps/web/src/shell/StudentApp.tsx` (ensure `openDemoProject` + `setStudioRoom` drive the studio; ensure the eval-report step can show the demo's report)
- Test: `apps/web/test/shell/StudentApp.projectsTour.test.tsx`

- [ ] **Step 1: Decide + implement the eval-report reach**

The demo `…0200` is a finished project; its evaluation report is available at the 评估报告 surface (`ReportsView`, opened via `pendingReportId`/`reportFocus` in ProjectsTab). Implement the `evaluation-report` segment's reach: add to `tourNav` an `openDemoReport()` that sets `pendingReportId = DEMO_PROJECT_ID` + `setTab("projects")` (ProjectsTab lands on the 评估报告 sub focused on that project — mirror the existing `openReportFromHome`). The `evaluation-report` segment's first step calls `nav.openDemoReport()`, then anchors `[data-testid="evaluation-report"]` + `#s1`…. Add `openDemoReport` to `TourNavContext` (types.ts) and wire it. (If `ReportsView` requires the report be in the user's own list, confirm it fetches by id for the world-readable demo; if not, fall back to describing the report centrally and note it.)

- [ ] **Step 2: Failing integration test**

Create `apps/web/test/shell/StudentApp.projectsTour.test.tsx` (mirror `StudentApp.tour.test.tsx`): with a user, invoke the tour (e.g. simulate the welcome "项目" pick or a direct `play`), and assert that playing the projects group drives `setTab("projects")` and opens the demo project (the WorkspaceContainer mock receives `initialProjectId = DEMO_PROJECT_ID`) and that a room switch occurs (pendingRoom propagates). Mock heavy children as the existing test does.

- [ ] **Step 3: Run to verify it fails, then implement, then pass**

Run: `cd apps/web && npx vitest run test/shell/StudentApp.projectsTour.test.tsx`
Implement the wiring until PASS. Also run the existing `StudentApp.tour.test.tsx` to ensure no regression.

- [ ] **Step 4: Full suite + typecheck**

Run: `cd apps/web && npm run test && npm run typecheck`
Expected: green.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/tour/types.ts apps/web/src/shell/StudentApp.tsx apps/web/test/shell/StudentApp.projectsTour.test.tsx
git commit -m "feat(tour): wire projects tour + eval-report reach in StudentApp" -m "Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Self-Review (author's pass)

- **Spec coverage (§7 projects group):** intro (borderline + evaluation) ✓; open demo ✓; rooms bar ✓; forming/plan/reading(warren+room+library)/writing/reflection ✓; evaluation-report walk ✓. Nav plumbing (open demo + room switch) is the enabling Task 1.
- **Graceful degradation:** conditional anchors (提问卡/未归类/suggestions/doc-switch) and the reading-room `rr-*` anchors (only present when a source is open) are called out; those steps use center placement so they never dead-spotlight. This mirrors P1's accepted approach.
- **Open questions (settle during execution):** (a) whether `ReportsView` renders the demo's report for a non-owner (world-readable) or needs a fallback — Task 4 Step 1 verifies; (b) the reading-room walkthrough is described centrally rather than opening a live source (opening a source mid-tour is complex); acceptable for P3, can deepen in polish. (c) The welcome "项目"-first ordering is P4 (P3's `fullJourney` is courses-then-projects).
- **No live LLM:** the tour only navigates + spotlights the demo (read-only, canned); no model calls.
