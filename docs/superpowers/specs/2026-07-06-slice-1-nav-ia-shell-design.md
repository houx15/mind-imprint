# Slice 1 · Nav / IA Shell — Design Spec

> 2026-07-06 · Part of `docs/2026-07-06-refactor-roadmap.md` (Slice 1 of 5).
> Binding design: `思维印记 工作区.dc.html` (Claude Design project `0772a92d-1e24-43b6-9237-5f93fb01f920`) — left rail, `COURSES: LIST`, `TASKS: DIRECTORY` sections.
> The `.dc.html` is binding for all UI; where prose spec and design disagree, the design wins.

## Goal

Reposition the student app's information architecture from a flat 3-tab shell (`任务 / 记录 / 设置`) to the 4-pillar shell the refactor calls for (`课程 / 批判思维 / 我的评估 / 设置`), and reskin the working-portal home to "项目" (project) vocabulary. Render the real Courses card grid from a mock fixture so the tab is exercisable. Frontend-only.

## Global constraints

- **Frontend only.** No changes to `apps/api` (Go), `packages/contracts`, the store schema, or the API surface. No new network calls.
- **UI copy is Chinese**, verbatim from the binding design. Code identifiers, comments, and this spec are English.
- **Palette / tokens** (already used across `apps/web`): indigo `#2A3B7A`, active-pill `#EDEFF9`, ink `#1C2333`, muted `#6B7384` / `#8A92A3` / `#9AA1B0`, hairline `#EAECF2`, bg `#F3F4F8`, accent terracotta `#D98263`, green `#4C9A82`, amber `#E8A33D`. Radii 12–18px. Fonts already loaded globally.
- **No addictive mechanics, no forced modals** (design law). Nothing in this slice adds streaks/badges/notifications.
- Existing behavior preserved: opening a project still routes to the current `WorkspaceContainer` unchanged (its redesign is Slices 2–3).

## Scope

1. **`LeftRail.tsx` — rewrite to 4 tabs.**
   - Tabs, in order, each with the design's exact icon path:
     - `courses` → label **课程** — open-book: `M4 5.5A2.5 2.5 0 016.5 3H20v15H6.5A2.5 2.5 0 004 20.5z` + `M20 18v3H6.5A2.5 2.5 0 014 18.5` + `M9 7.5h7M9 11h5`
     - `tasks` → label **批判思维** — star: `M12 3l2.4 5 5.6.7-4 3.9 1 5.4L12 15.4 6.9 18l1-5.4-4-3.9L9.6 8z`
     - `records` → label **我的评估** — bar-chart: `M12 20V10M6 20v-5M18 20V6` + `M3 20h18`
     - `settings` → label **设置** — gear (unchanged from current).
   - Keep the existing logo SVG, the avatar dot (→ `settings`), and the active/inactive token constants already in the file (`ACTIVE_BOX` `#EDEFF9`, `ACTIVE_ICON_STROKE`/`ACTIVE_LABEL` `#2A3B7A`, inactive `#9AA1B0`). Icon `stroke-width` 2, viewBox `0 0 24 24`, size 20.
   - `TabKey` becomes `"courses" | "tasks" | "records" | "settings"`. Internal key `tasks` intentionally retained for the 批判思维 portal to minimize churn; only its **label** changes.

2. **`StudentApp.tsx` — route the new tab.**
   - `Tab` type gains `"courses"`. Default tab stays `"tasks"` (the working portal — the app's primary surface).
   - `tab === "courses"` renders the new `CoursesView`. Existing `tasks/records/settings` branches unchanged except the type widening.

3. **`CoursesView` (new) — real grid from a mock fixture.**
   - New folder `apps/web/src/shell/courses/`.
   - `CoursesView.tsx`: header block per design — eyebrow `课程`, H1 `系统地学会一种思考方式`, subtitle `每一门课都是一段 AI 带着你走的学习旅程——有讲解，也有你亲自上手的挑战。学完，去「批判思维」工作台把它用在你自己的问题上。` — then a 2-col grid (`repeat(2,1fr)`, gap 20) of course cards.
   - Course card matches the `COURSES: LIST` `sc-for` node: figure band (gradient by branch, branch chip top-left, medal icon), body (title 18/800, blurb, meta row `{tasks} · {tools}` + clock `{time}`, optional progress bar + `{pct}`, footer row = tone label (with check when done) + CTA button label+arrow). Card hover lift is the standard `translateY(-3px)` shadow.
   - **Cards are inert on click in this slice** (the course player is Slice 4). Clicking is a no-op (no navigation, no dead route). The CTA button and card share the same no-op handler.
   - `fixtures.ts`: a typed `MockCourse[]` with **one** seed course, clearly commented as a Slice-1 fixture to be replaced by backend course data in Slice 4:
     - branch: `批判性思维` (indigo family — figure gradient `linear-gradient(135deg,#EDEFF9,#F3F0EC)`, chip indigo, medal icon = the star path, iconColor `#2A3B7A`)
     - title: `一条网络信息，该不该信`
     - blurb: `从一句「卫星图显示中国让地球变绿」出发，跟着印记学会横向溯源、辨识来源、拆穿断言——把「随手一信」变成「查过再信」。`
     - meta: `3 个任务 · 4 个工具`
     - time: `约 40 分钟`
     - progress: not started (`showProgress:false`)
     - tone: `未开始` (neutral gray tone style, no check)
     - cta: `开始学习`
   - `MockCourse` type carries the presentation fields the card needs (id, branch, branchTone, title, blurb, meta, time, progressPct|null, tone, toneKind: `not_started|in_progress|done`, ctaLabel). Style derivation (figStyle/chipStyle/medalStyle/toneStyle/ctaStyle/barTrack/barFill) lives in the component or a small `courseCardView.ts` helper, mirroring the existing `taskCardView` pattern.

4. **`DirectoryView.tsx` — "项目" vocabulary reskin (copy-only).**
   The layout already matches the design (status pill, last-active, title, progress bar, cards label via `taskCardView`). Only copy changes:
   - eyebrow `下午好，Phoebe` → `下午好，{userName} · 批判思维工作台`. `DirectoryView` gains a `userName?: string` prop; `StudentApp` supplies it from `session.getUser()` (display name), falling back to `Phoebe` when absent. This is the only prop/signature change in the reskin.
   - H1 `今天你在尝试什么？` → `你想搞懂什么？`
   - subtitle → `每一个项目是你正在思考的一件事——可以随时离开，回来接着想。想搞懂新的东西时，开一个新项目。`
   - input placeholder → `开一个新项目——把你正纠结的问题写下来，带上你自己的东西（链接、草稿、本子上的话）。`
   - start button `开始` → `开新项目`
   - section title `进行中的任务` → `进行中的项目`
   - count `{n} 个任务` → `{n} 个项目`
   - Empty state (no projects yet): show the section header with `0 个项目` and a one-line muted hint `还没有项目，从上面开一个吧。` (the current code renders an empty grid; add this hint). No behavioral change to `handleStart`.

## Out of scope (explicitly)

Workspace redesign, material sidebar, material-anchored cards, real course data / course player / course report, voice, Records reskin, Settings reskin, the 产品设计 portal (the binding design omits it), any backend or contracts change.

## Data flow

- `CoursesView` reads only its local `fixtures.ts` — no store, no API.
- `DirectoryView` is unchanged in data terms (still `store.listTasks()` + `taskCardView`); only strings and one empty-state hint change.
- Tab state stays local to `StudentApp` (`useState`), as today.

## Testing (Vitest + Testing Library, matching existing `apps/web` tests)

- **`LeftRail`**: renders exactly 4 tabs in order with labels `课程/批判思维/我的评估/设置`; the tab matching `tab` prop has `aria-selected=true` and active styling; clicking a tab calls `onTab` with its key; avatar click calls `onTab("settings")`.
- **`StudentApp`**: `tab="courses"` renders `CoursesView` (assert its H1) and none of directory/workspace; switching back to `tasks` renders the directory. (Follow the existing `AppShell.test.tsx` store/session mocking pattern.)
- **`CoursesView`**: renders the fixture course's title, meta, time, tone, and CTA; clicking the card/CTA does not throw and does not navigate (no-op handler called).
- **`DirectoryView`**: renders `你想搞懂什么？`, `进行中的项目`, and `{n} 个项目`; with zero tasks shows the empty hint; with tasks shows one card per task (reuse existing test scaffolding if present).

## Risks / notes

- `LeftRail` currently carries stale "lifted from HTML lines …" comments referencing the old design; replace them with references to the new `.dc.html` left-rail section so the provenance comments don't drift.
- Pulling the Courses grid forward from Slice 4 is deliberate (roadmap updated). The fixture is throwaway and must be labeled as such so Slice 4 doesn't mistake it for real data.
