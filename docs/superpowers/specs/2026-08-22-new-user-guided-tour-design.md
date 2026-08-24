# New-User Guided Tour ("引导旅程") — Design Spec

**Date:** 2026-08-22
**Status:** Design approved; ready for implementation planning.
**Owner:** houyx15

---

## 1. Overview & Goal

**Goal:** When a new student first enters 思维印记, 印记 (the AI companion) offers to walk
them through the product. The tour is a composable, config-driven guided journey that
spotlights real UI, auto-navigates the app, and teaches the two core surfaces — **课程**
(courses) and **项目** (projects) — plus the settings/accent finale.

**Key properties (approved):**
- **Custom engine**, not a third-party tour library — the tour is a signature branded
  experience (印记 talking to the student), tightly coupled to our own React-state
  navigation, with a specific composable architecture. Those are exactly where libraries
  fight us; the only thing a library saves is popover positioning, which is bounded.
- **Hybrid interaction:** the tour drives navigation itself and mostly spotlights + explains
  (advance on 下一步); only a few signature moments are real "you try it" interactions.
- **Composable:** `Journey → Segment (环节) → Step`. Any single segment can run standalone
  (contextual help / re-trigger), or segments concatenate into the full journey.
- **Non-blocking:** every step is skippable; a global 跳过 / 结束 is always available. This is
  a UX choice, not a 铁律 constraint — a tour does not force student *thinking*, so 铁律②
  does not apply.
- **No live LLM during onboarding:** every AI-flavored thing the tour shows — demo project
  AI chat, AI suggestions, AI organizing, AI annotations, evaluation report, example course
  report — is **frozen/mock fixture data**. Onboarding is instant, deterministic, free, and
  cannot break on a model timeout.

**Non-goals (this design):**
- Not a general in-app help system (though the standalone-segment architecture leaves the
  door open to contextual help later).
- The 反馈 (feedback) control in the nav footer is scaffolded minimally, not a full feature.
- No analytics/telemetry on tour completion beyond the single first-run flag.

---

## 2. Engine Architecture

A small in-house engine: one overlay Runner component + a React context provider + a
declarative config format. No new runtime dependency.

### 2.1 Config model

```ts
// packages/contracts (or apps/web/src/tour/types.ts) — single source of truth
interface TourStep {
  id: string;
  // Drive app state BEFORE the step renders: switch tab/sub-tab, open the demo
  // project, enter a course, scroll a section into view. Receives a typed context
  // of navigation setters (see 2.3). Idempotent.
  onEnter?: (ctx: TourNavContext) => void | Promise<void>;
  // CSS selector, by convention `[data-tour="<id>"]`. If the element is absent,
  // the step degrades gracefully (see 2.4).
  anchor?: string;
  spotlight?: boolean;          // cut a hole over the anchor and dim the rest (default true if anchor set)
  placement?: "top" | "bottom" | "left" | "right" | "center"; // bubble placement; "center" = modal-style, no anchor
  bubble: {
    text: string;               // 印记's line (Markdown allowed, rendered via the existing md renderer)
    title?: string;
  };
  advance: "next" | "action";   // "next" = 下一步 button; "action" = wait for a real user event
  // For advance:"action" — how we know the user did the thing.
  actionEvent?: { selector: string; type: "click" | "input" | "custom"; customEventName?: string };
}

interface TourSegment {
  id: string;                   // e.g. "courses-player"
  name: string;                 // display name, e.g. "课程内的互动"
  steps: TourStep[];
}

type TourJourney = TourSegment[]; // ordered; the full run concatenates groups
```

### 2.2 Runner + Provider

- `TourProvider` (wraps the app inside `StudentApp`) holds tour state: `activeJourney`,
  `segmentIndex`, `stepIndex`, `running`. Exposes an imperative API via context:
  `play(target: TourSegment | TourJourney)`, `stop()`, `next()`, `prev()`, `skipSegment()`.
- `TourRunner` (rendered by the provider when `running`) paints:
  - a full-viewport dimming overlay with a **spotlight cutout** positioned over the resolved
    anchor (a `box-shadow: 0 0 0 9999px rgba(...)` cutout, or an SVG mask);
  - the **印记 popover** — small avatar (reuse `Pebble` / accent chip), the bubble text, and
    controls: 上一步 / 下一步 (or the action hint) / 跳过本节 / 结束.
  - keyboard: Esc = stop, →/Enter = next (when advance:"next"). Respect
    `prefers-reduced-motion`.
- Styling uses the existing `mk-*` design tokens and accent so the tour matches the app and
  the current student's chosen accent color natively.

### 2.3 Navigation context (driving the app)

The app is pure React state (no router), so the tour drives it through a small typed
context assembled in `StudentApp`:

```ts
interface TourNavContext {
  setTab: (t: NavTab) => void;                      // StudentApp top-level tab
  openDemoProject: () => void;                       // pending-project deep-link → studio
  openCourse: (slug: string) => void;                // pending-course deep-link
  setProjectsSub?: (s: "projects" | "reports") => void;   // NEW prop, mirrors pending pattern
  setCoursesSub?: (s: "courses" | "history" | "gallery") => void; // NEW prop
  setStudioRoom?: (r: "forming" | "plan" | "reading" | "writing" | "reflection") => void; // NEW
  // ...small setters as needed, each added by mirroring the existing one-shot "pending" pattern
}
```

**Codebase facts this relies on** (from exploration):
- Top-level tabs: `StudentApp` `const [tab,setTab]=useState<NavTab>` (`apps/web/src/shell/StudentApp.tsx:47,152`);
  `Nav` is controlled via `onTab` (`apps/web/src/shell/Nav.tsx:42-50,123`).
- Deep-link pattern already exists: `openProjectFromHome`/`openReportFromHome`/`openCreateFromHome`/`openCourse`
  set a `pending*` state + flip the tab; the child tab consumes it at mount
  (`StudentApp.tsx:53-94`, `ProjectsTab.tsx:48-58`, `CoursesTab.tsx:44-53`).
- Sub-tabs (`setSub`) are currently **private** to `ProjectsTab`/`CoursesTab`. Driving an
  arbitrary sub-tab (e.g. jump to 图鉴 / 学习记录, or to a studio room) requires adding a new
  `initialSub` / `pendingSub` prop per tab, mirroring the existing `pendingReportId` mechanism
  (`ProjectsTab.tsx:48-49`). The studio room switch (`RoomSwitcher`, `apps/web/src/workspace/RoomSwitcher.tsx:19`,
  `BLOCK_META` `apps/web/src/workspace/Icon.tsx:104-110`) similarly needs an external setter surfaced.
- Immersive mode hides the nav rail while a studio/player is open
  (`StudentApp.tsx:67-69`, `CoursesContainer.tsx:109-111`). The tour must account for the rail
  being hidden in those segments (anchor to in-studio/in-player elements, not the rail).

### 2.4 Anchor resolution & resilience

- Convention: anchors are `[data-tour="<id>"]`. There are **zero** `data-tour` attributes
  today; they are added as part of each segment's task (see §6).
- Resolution polls briefly for the element (it may mount after `onEnter` navigation). On
  timeout, the step degrades: if `spotlight` fails, fall back to a centered bubble; log a dev
  warning. A missing anchor never hard-stops the tour.
- The course **player** and evaluation **report** already expose stable hooks we reuse
  instead of adding new ones:
  - Player renderer: `.course-nav__next`, `.course-single-choice__submit`,
    `data-single-choice-prompt`, `.course-shell`/`data-course-shell`,
    `data-testid="ask-bubble"`, `data-testid="course-region"`
    (`packages/course-renderer/src/course/CourseNav.tsx:27-42`,
    `blocks/assessment/SingleChoiceRenderer.tsx:64-95`,
    `apps/web/src/shell/courses/RuntimeCoursePlayer.tsx:286`, `AskPanel.tsx:196`).
  - Report: root `data-testid="evaluation-report"`, sections `#s1`…`#s9`
    (`apps/web/src/shell/report/EvaluationReport/index.tsx:37,41-75`).

---

## 3. Trigger & Persistence

- **First-run flag = server field on the user**, mirroring the accent/background preference
  pattern (`apps/web/src/api/auth.ts:54-67`, wired `api/index.ts`). Add e.g. `onboarded_at`
  (nullable timestamp) to the user row + `MeUser` (`apps/web/src/api/auth.ts:5-14`) + a
  `PUT /api/v1/users/me/onboarding` writer. Seed the welcome modal's "should show" from
  `user?.onboarded_at == null`, the same way `initialAccent={coerceAccent(user?.avatar_color)}`
  is seeded (`StudentApp.tsx:139-140`).
- **Fires once, across devices.** Completing OR dismissing the welcome flow stamps
  `onboarded_at`.
- **Manual re-trigger** anytime from the nav-footer 重新开始引导 entry (§4). This replays the
  full journey (or, later, a chosen segment).

---

## 4. Shell Additions

### 4.1 Welcome modal
On first login, a centered modal (uses the Runner's `placement:"center"` or a dedicated
component):

> "xx你好呀，欢迎来到思维印记 AI 思辨力成长平台。我是印记，你的 AI 小伙伴。要不让我先带你逛逛这里吧！想先看看什么呢？"

Options: **课程** / **项目** / **稍后再说**.
- 课程 → play the full journey starting with the courses group, then projects.
- 项目 → play starting with the projects group, then courses.
- 稍后再说 → dismiss + stamp `onboarded_at`; a brief note points them at the nav-footer entry
  ("以后可以在这里再叫我：<footer icon>").

The courses-first / projects-first choice is simply **which order the two groups concatenate**
in the journey.

### 4.2 Nav footer
Add a footer block to the accent rail (`apps/web/src/shell/Nav.tsx`, after the `items.map`
at ~`:136`, pinned with `mt-auto`; reuse the collapse/expand `labelCls` at `:93-95`). It hosts:
- **重新开始引导** — calls `tour.play(fullJourney)`.
- **反馈** — net-new; a small form that **saves to a `feedback` table** (user id, text,
  timestamp) via a new `POST /api/v1/feedback`. Minimal UI; not a core deliverable.
- **退出登录** — reuse the existing `onLogout` (originates `AppShell.tsx:80`, currently only in
  `SettingsView.tsx:210-218`).

`Nav` currently receives only `tab,onTab,user` (`Nav.tsx:42-50`); thread new props
(`onRestartTour`, `onFeedback`, `onLogout`) from `StudentApp`. Add stable `data-tour` hooks to
the footer entry so the welcome "稍后" step can point at it.

---

## 5. Demo Data & Mock-AI Strategy

**Approved:** one **shared, read-only** demo project, hand-authored to maximize showcase, with
**all AI content mocked** — no live LLM anywhere in the demo.

### 5.1 Demo mode (safety for a shared read-only project)
The demo project carries a flag (e.g. `project.is_demo = true`). Enforced on both ends so a
user poking the demo *outside* the tour can never burn tokens or mutate shared state:
- **Backend:** every token-consuming / mutating endpoint checks the flag and either returns a
  **canned fixture** (studio coach, `courseAsk`, exploration dig/organize, proposal
  annotations, evaluation-gen) or **rejects/no-ops writes** — a demo project is never
  mutated; return a no-op success so the frozen UI never surfaces an error.
  Endpoints to guard, from exploration:
  - evaluation generation `apps/api/internal/api/evaluation_generate.go:60` (return the frozen
    report instead of the 4 LLM calls);
  - studio coach threads / `chat_message` writes;
  - exploration dig/organize (`ExplorationView.runDig`/organize);
  - proposal annotation review;
  - course ask (`RuntimeCoursePlayer` `AskPanel`, `api.courseAsk`).
- **Frontend:** the demo renders **pre-seeded result states**; during the tour the AI moments
  are **show-and-tell** (spotlight the seeded result; 印记 explains "this is where AI
  suggested these"), not live button clicks. Where a live "you try it" is desirable (e.g. the
  reading-room chat), the action is wired to a **frozen canned turn**, never the model.

### 5.2 Seeded content (hand-authored)

**Content-quality bar (hard requirement).** The demo is a showcase; short or toy text
undersells the product. It must read like a *real, substantial* finished project:
- a genuine research question (the China-sustainability question is real and canonical);
- a **full-length** proposal and a **full-length** essay/main paper — proper paragraphs, not
  stubs or placeholders;
- a real literature list with real-looking sources and a substantive warren map (main
  question + several subquestions + edges + a 未归类 item);
- substantive reflections (real answers to all 5 prompts), and a proper, populated evaluation
  report.
Authoring this content well is a first-class task, not filler.

The existing demo seed (`apps/api/internal/store/migrations/0018_seed_demo_project.sql`) is a
**partial in-progress** snapshot (project id `…0101`, status `active`, S4 current; only
`graph_node`, 2 empty `material`, `intervention`, 2 `card_instances`). It must be replaced /
extended into a **finished** project (status `done`) with real content across all five rooms.
Tables to populate (full inventory from exploration):
- **立题/管理:** `graph_node`, `plan_item`, `activity_log_entry`, milestone `event`s.
- **阅读:** `reference` (+ `collection`, `citation`, `source_log_entry`), `exploration_lead`
  + `graph_edge`/`question_edge` (warren map with main question + subquestions + a 未归类
  item), `material` rows with **real `blocks`** (so the reading room has content), a seeded
  reading-room `chat_thread`/`chat_message` (one AI turn), reading notes / 证据笔记.
- **写作:** `project_proposal` (finished), `outline_node`, `snippet`, `draft_snapshot`,
  `writing_finish`, plus proposal annotations (AI 批注, seeded).
- **回顾:** `project_reflection` (5 answers, `done=true`), `project_mirror_prose`.
- **评估报告:** a hand-authored, frozen `evaluation_report` JSON payload matching the
  `EvaluationReport` contract (fixture reference: `apps/web/src/shell/report/EvaluationReport/__fixtures__/mock.ts`).
- **AI threads:** `chat_thread`/`chat_message` (surface="studio") + `messages` for the
  seeded coach dialogue shown in 立题.

### 5.3 Courses demo data — NO DB seed, honest surfaces (decided)
The courses journey does **not** seed fake data into the new user's account (that would put a
course they never took into their real 学习记录 and dishonestly fill 图鉴 stars). Instead:
- **Enter a course:** open a **real published 2.0 course** and teach interaction on the live
  player as show-and-tell — opening scene, narration reveal, one interactive block, the gated
  `.course-nav__next`, the AI ask bar. **Never complete the attempt** for the student; back out
  when the teaching is done. (Renderer hooks already exist; no seed needed.)
- **Example course report:** show a **well-crafted frozen fixture report in a clearly-labeled
  "示例" (example) view** the tour opens — not a faked attempt. There is no example report for a
  brand-new user today (`CourseReport.tsx:245-258`); the exact rendering path (fixture prop /
  example mode on `CourseReport.tsx`, vs. a frozen-report demo course endpoint) is a P1-plan
  detail. Content quality bar (§5.2) applies to this fixture too.
- **学习记录 / 图鉴 practice history:** teach the surface **honestly** — spotlight the
  (initially empty) attempt log / the "还没遇到" card states and explain "your finished courses
  and their reports / your practiced cards and stars will appear here." Structure is taught
  without faking the user's record.

Net effect: **the courses slice (P1) needs no database seed — only one report fixture** — so it
reaches the format-check gate fast.

---

## 6. Anchors

Add `data-tour="<id>"` attributes to every spotlighted element that lacks a stable hook.
Inventory (surfaces → representative anchors), from exploration:
- **Nav:** footer entry; (rail icons already have role/aria but add `data-tour` for precision).
- **Courses:** Segmented sub-options (`CoursesTab.tsx:75` — no per-option testid today), a
  representative `CourseCard`, the first `FilterChip` (categories), a `HistoryRow`
  (`LearningHistory.tsx:66`), a `CardTile` (`ToolkitCards.tsx:85`), the card-detail modal's
  介绍/练习历史 tabs + `RelatedCourses` block (`CardDetailModal.tsx`), the report's
  查看我的答案 button + key report sections (`CourseReport.tsx`).
- **Projects — five rooms** (all need anchors added; `WorkspaceContainer.tsx` room switch
  `:1291-1414`, `RoomSwitcher` via role+name is reusable):
  - 立题: 提问卡 button (`PlanBlock.tsx:354-362`), the AI rail (single portaled `AiPanel`,
    `WorkspaceContainer.tsx:1285-1291`), proposal tracker (`PlanBlock.tsx:275`).
  - 管理: 看板/甘特图/活动日志 toggle (`PlanBlock.tsx:621-624`), a Gantt bar, 导出 button
    (`PlanBlock.tsx:629`).
  - 阅读: warren map main-question node + 未归类 sentinel (`WarrenMap.tsx:51,156-166`),
    keyword box + suggestions tray (`ExplorationSidebar.tsx:180,493`), organize action,
    `PaperDetail` 进入阅读室 (`PaperDetail.tsx:69`); reading room chat/select-text/cards/notes/finish
    (`ReadingRoom.tsx` refs); library table (`ReadingBlock.tsx:699`).
  - 写作: proposal/main-paper doc switch, 大纲/片段/正文 tabs (`WritingBlock.tsx:187-194`),
    a snippet AI card, `ReferencePanel` reading-notes + 批注 (`WorkspaceContainer.tsx:1325`).
  - 回顾: reflection prompts (`ReviewBlock.tsx:62,79-87`).
- **Evaluation report:** reuse `#s1`…`#s9` — no new anchors needed.
- **Settings:** reuse `data-testid="accent-swatch"` (`SettingsView.tsx:116`); scroll the 主题色
  Card into view; navigate via `setTab("me")`.

---

## 7. Segment Inventory

Derived from the approved vision. Each segment is standalone-safe (its first step navigates
into position). `A` = show-and-tell (advance:"next"); `T` = real "you try it" (advance:"action").

### Welcome
- `welcome` — center modal, branch to courses-first or projects-first (§4.1).

### Courses group
- `courses-intro` — why these courses exist (思辨力 as the key AI-era ability; systematic
  practice). `A`
- `courses-categories` — spotlight the category filter chips; explain the taxonomy. `A`
- `courses-enter` — guide into a course (open detail → player). `T` (click a course) or `A`.
- `courses-player` — teach the in-course loop: opening scene, narration reveal, answering an
  interactive block, 下一步 gating, the AI ask bar. Uses renderer hooks. Mostly `A`, one `T`
  (answer one question).
- `courses-report-example` — show the example (frozen) course report; walk its parts (用时/
  阶段, 小测表现 + 逐题, 学到的工具卡). `A`
- `courses-reports-page` — switch to 学习记录; explain the attempt log. `A`
- `tujian-cards` — switch to 图鉴; explain tool cards + the grid. `A`
- `tujian-detail` — open one card; walk 介绍 (口诀/steps/例子). `T` (open a card).
- `tujian-history-related` — show 我的练习历史 tab + "在这些课程里学它" link. `A`

### Projects group
- `projects-intro` — the idea of AI-guided writing projects; **AI has a borderline** and there
  is **evaluation**. `A`
- `projects-open-demo` — open the shared demo project into the studio. `A`
- `project-rooms-bar` — spotlight the 立题-管理-阅读-写作-回顾 bar; the five parts overview.
  (Demo lands on 立题.) `A`
- `forming` — 立题: components to discuss; spotlight the AI rail (asks questions, 提问卡,
  hints to fill the 立题 form). `A`
- `plan-manage` — 管理: 甘特图, edit-via-AI-or-manual + export, 看板, 活动日志 (auto + manual),
  export. `A`
- `reading-warren` — 阅读/research: warren map main question + subquestions; 未归类; click a
  node to dive; AI keyword suggestions (seeded tray, adopt→search→view = show-and-tell); AI
  organize (seeded). Note: sometimes paste text because AI can't fetch full texts. `A`
- `reading-room` — enter reading room: chat (mocked one turn), select-text-to-chat,
  card/lens guidance, reading notes, finish reading. Mostly `A`, optional `T` (send the
  canned chat turn).
- `reading-library` — back to 阅读; the 文献库 (library) page. `A`
- `writing` — 写作: proposal/main-paper switch; 大纲/片段/正文; a snippet AI-guiding card
  (seeded); reading-notes left bar; AI 批注 (seeded). `A`
- `reflection` — 回顾: the reflection prompts. `A`
- `evaluation-report` — walk the frozen evaluation report part by part, spotlighting each
  section `#s1`…`#s9`. `A`

### Finale
- `settings-accent` — `setTab("me")`; spotlight the 主题色 swatches; "you can change your
  favorite color here." `A`

---

## 8. Phasing (matches approved checkpoints)

Each phase ends at a point the user can review before the next starts.

- **P1 — Courses slice (format-check gate).** Tour engine (Provider + Runner + config types +
  anchor resolution + nav context), welcome modal, nav footer, the `onboarded_at` server
  field + writer, the **Courses group** segments, and **one frozen example-report fixture**
  (no DB seed — §5.3). Deliverable: a real, shippable courses onboarding.
  **← user reviews the format here.**
- **P2 — Demo project + mock data (data-check gate).** Hand-authored finished demo project
  migration (all five rooms) + `is_demo` flag + backend demo-mode guard + frozen
  evaluation-report JSON + seeded AI content. **← user reviews the data here.**
- **P3 — Projects slice.** The five-room segments + all `data-tour` anchors across the rooms +
  the evaluation-report walkthrough. Depends on P2.
- **P4 — Full journey + finale + polish.** Courses/projects-first branching wired end-to-end,
  the settings/accent finale, feedback-button scaffold, re-trigger polish.

---

## 9. File Structure (anticipated)

- `apps/web/src/tour/` — engine: `TourProvider.tsx`, `TourRunner.tsx`, `types.ts`,
  `useTour.ts`, `anchors.ts`, and `segments/` (one file per segment group: `courses.ts`,
  `projects.ts`, `welcome.ts`, `settings.ts`) + `journey.ts` (composition).
- `apps/web/src/shell/Nav.tsx` — footer block (re-trigger / feedback / logout).
- `apps/web/src/shell/StudentApp.tsx` — assemble `TourNavContext`, mount `TourProvider`,
  welcome-modal trigger from `onboarded_at`, thread nav-footer props.
- `apps/web/src/shell/ProjectsTab.tsx` / `CoursesTab.tsx` / `WorkspaceContainer.tsx` —
  external sub-tab / room setters (mirror the pending pattern).
- `apps/api/internal/store/migrations/` — new migration: finished demo project + `is_demo`
  flag column + finished-course-attempt seed.
- `apps/api/internal/api/` — demo-mode guards on token-consuming/mutating endpoints; the
  `onboarding` writer.
- `apps/web/src/api/auth.ts` — `MeUser.onboarded_at`, `putOnboarding`.
- `data-tour` attributes added across the surfaces in §6.

---

## 10. Resolved Decisions (were open questions)

1. **Demo course in the player:** ✅ play a **real** published course live as show-and-tell,
   **no completion**; the example report is a **frozen "示例" fixture**, not a faked attempt
   (§5.3). The courses slice needs no DB seed.
2. **Demo-mode write semantics:** ✅ demo projects **reject/no-op every write** (no-op success
   so the frozen UI never errors) (§5.1).
3. **Feedback control:** ✅ a small in-app form that **saves to a `feedback` table** via
   `POST /api/v1/feedback` (§4.2).
4. **Tour config types location:** ✅ **frontend-local** in `apps/web/src/tour` — the backend
   never reads tour config, so no `packages/contracts` entry is needed.
