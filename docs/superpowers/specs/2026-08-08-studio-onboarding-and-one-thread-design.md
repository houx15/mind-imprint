# 写作空间 onboarding + 单线程 + subagent 分层 Design

> Refinement of the agentic writing studio (印记 IS the agent, rooms are its tools).
> Status: approved design, ready for implementation plan.
> Date: 2026-08-08. Supersedes the relevant surface behavior in
> `2026-08-07-agentic-studio-orchestrator-redesign.md` (which stands for the
> orchestrator loop itself).

## Goal

Make first contact with a project feel like a real conversation with 印记, keep
the chat a single continuous thread the student can always trust, and make the
five rooms read as *places the student explores* rather than five separate AIs.
Four changes plus a clarified subagent model.

## Context (current behavior)

- **Opening** is a hardcoded frontend string `CHAT_INTRO`
  (`apps/web/src/studio/ai/StudioCoachChat.tsx:23`), plus a second hardcoded
  `INTRO_ZH` in `PlanBlock.tsx:39`. Neither is LLM-generated; both are display
  fallbacks that vanish on the first real turn.
- **Tabs** (the 5-segment `Segmented` switcher) render immediately on entering a
  project (`WorkspaceContainer.tsx:732`). There is no "start" gate; a new project
  lands chat-first but the tabs are already present.
- **History** — `GET /coach/history?surface=studio` returns *every* studio turn
  with no limit (`coach.go:488`, `chat.sql:46`); the client reloads the whole
  thread on every open (`WorkspaceContainer.tsx:518`).
- **Per-tab chat split** — 阅读 (`find_sources`) and 回顾 (`reflection`) are
  separate sub-agent *surfaces* with their own persisted threads
  (`surface` = `find_sources` / `reflection`). Switching tabs can therefore
  surface a different conversation, which reads as "the chat changed."
- **Tab buttons** render at `text-mk-small` = **12px** (`feedback.tsx:145`) with
  no icons and low-contrast grey inactive — they read as hints, not navigation.
  Per-room icons already exist unused in `Icon.tsx` (forming/plan/reading/
  writing/reflection).

## The 印记 + subagent model (authoritative)

**印记 = one continuous main thread** (`surface = "studio"`) spanning **提案 /
管理 / 写作**. Switching among these three tabs never changes the visible chat.
This is the single source of truth for "one chat history."

**Explicit subagents** — visible, own AI presence, framed as entering a focused
space; they legitimately show their own conversation (a takeover of the coach
panel), and returning exits back to 印记:
- **A specific paper inside 阅读.** Entering one paper spawns a paper-scoped
  subagent because a paper consumes a lot of context. The 阅读 *list* level is
  not itself a subagent.
- **回顾 (reflection).** A fresh AI introduces itself; a distinct presence here
  is intended and acceptable.

**Hidden subagents** — no chat surface at all. They render only a **loading
status + a one-line hint**, and their result lands in the room:
- Generating / editing the **project plan** (`generate_plan`).
- Reading / digesting the **reading-list search results** (`find_sources`).

**Hint copy uses the English word `subagent`, never 子代理.** e.g.
`subagent 正在检索来源…`, `subagent 正在整理研究计划…`.

**Compaction** also shows a loading status while it runs, e.g. `正在整理对话…`.

## Change 1 — Opening is a real LLM turn

Delete `CHAT_INTRO`. On first entry to a project whose studio thread is empty,
the frontend fires **one** "opening" coach turn (a sentinel input the
orchestrator recognizes) so 印记 produces a genuine welcome. It is a normal
persisted assistant turn — no special storage.

The orchestrator system prompt gains a crafted **opening** section. The opening
message must:
1. Welcome the student to 写作空间.
2. Echo the project's 题目 (from the spine projection — the orchestrator already
   receives it).
3. Name the whole journey — that completing a writing project moves through the
   rooms (立项 → 阅读 → 写作 → 回顾), in the studio's own words.
4. Lay out the four things they will settle first, as a short list:
   - 目标（research question）
   - 缘由（motivation）
   - 活动与时间（plan）
   - 资源（resources）
5. End by asking 准备好开始了吗？

Tone follows the four design 铁律 — 克制, one-thing-at-a-time in the *dialogue*
that follows; the opening itself is a single framing message, not a barrage of
questions. Real content, echoing the actual 题目, never lorem.

The opening turn keeps the project in the pre-start state (see Change 2): the
returned directive does **not** open a tool.

## Change 2 — Start gate → tabs

Add a `started: boolean` flag to `StudioState` (default `false`).

- **Before 开始** (`started === false`): the workspace shows a **pure full-width
  chat**, and the switcher/tabs are **not rendered at all** (not merely
  unhighlighted). This is a stronger gate than today's `chatOnly`.
- The opening message is accompanied by an **开始** affordance (a primary button
  under the thread, with the `arrow`/`spark` icon). It is not a 12px chip — it is
  a button the student must clearly see.
- Clicking **开始** sends a start turn to the orchestrator, which sets
  `started = true` and opens the first tool (提案/管理). The interactive area +
  the 5 tabs appear, and 印记 begins the 提案 discussion (the substance of the
  old `INTRO_ZH` now spoken by the live agent, not a hardcoded string).
- For **existing** projects (`started === true`, non-empty thread), entry goes
  straight to the interactive area — no opening turn, no gate.

`INTRO_ZH` and its scripted chips in `PlanBlock.tsx` are removed; the 提案
framing is now the agent's job.

## Change 3 — Recap + lazy pages

Replace the load-everything display path with pagination on the main studio
thread.

**Backend** — `GET /coach/history` gains cursor pagination for
`surface=studio`:
- Query params: `limit` (default 20) and `before` (an opaque cursor —
  `created_at,id` of the oldest loaded row).
- Response: `{ messages: [...oldest→newest within the page...], hasMore: bool,
  recap: string | null }`.
- `recap` is populated **only on the first page** (no `before`) and only when
  `hasMore` is true. It is the stored compaction digest prose —
  `conversation_digest.prose` via `GetConversationDigest(project_id)`
  (`chat.sql:85`). No LLM call on a plain history GET — GET must not spend
  tokens, per the project rule. If no digest row exists yet, `recap` is `null`
  and the UI simply shows 载入更早.
- The AI-context path (`LoadActiveCoachHistory`, 12-turn window) is unchanged.

**Frontend** —
- Initial open loads the most recent page (~20) instead of the whole thread.
- An **AI recap card** renders at the top of the thread when `recap` is present
  ("我们之前聊到…"), visually distinct from a normal turn.
- A **载入更早的对话** control at the top prepends the next older page
  (`before` = current oldest cursor) while preserving scroll position; it hides
  when `hasMore` is false.
- Optimistic in-flight turns are never clobbered by a page load (keep the
  existing `prev.length ? prev : …` guard, adapted for prepend).

Pagination is scoped to the main studio thread. Explicit-subagent chats (paper,
reflection) are short-lived / bounded and keep their existing load.

## Change 4 — Tab buttons obey the design system

The 5 tabs are primary navigation, not hints.

- Font size **≥ 14px** (`text-mk-body`), not `text-mk-small`.
- Each tab shows its existing **icon** (`Icon.tsx` forming/plan/reading/writing/
  reflection) beside the label.
- Inactive tabs use a legible ink color (not `text-mk-muted`); active keeps the
  white "thumb."
- Implemented either by passing icon+label nodes as the `Segmented` option
  `label` (the type is already `ReactNode`) and bumping the size, or a
  purpose-built room switcher — implementer's choice, but the `Segmented`
  primitive's default 12px must not be silently reused for these tabs.

## Loading states (hidden subagents + compaction)

Every hidden subagent turn and compaction shows a **loading status + hint** in
the coach panel where the agent's reply would appear:
- `generate_plan` → `subagent 正在整理研究计划…`
- `find_sources` → `subagent 正在检索来源…`
- compaction → `正在整理对话…`

These are transient (replaced by the result / next turn), single-line, and may
use the `spark` icon. The hint line is a genuine hint, so 12px is allowed here —
this is the one place 12px is correct.

## Data model & contract changes

- `StudioState` (`packages/contracts/src/orchestrator.ts` +
  `apps/api/internal/agent/studiostate.go`) gains `started: boolean`
  (default `false`). `DefaultStudioState()` and the migration default JSON add
  it; existing rows without the key resolve to `false` on unmarshal, so a
  **backfill is needed** for already-started projects (any project with a
  non-empty studio thread → `started = true`) to avoid re-gating live projects.
  A goose migration performs the backfill.
- `orchestrator.ts` history/response types gain the paginated
  `CoachHistoryPage` shape (`messages`, `hasMore`, `recap`).
- New orchestrator tool intent for start is **not** required — 开始 is an
  ordinary turn; the server sets `started = true` when it opens the first tool
  from the start turn. (If cleaner, the handler may set `started` on the first
  `open_tool` emitted after the opening; implementer's choice, documented.)

## Files (anticipated)

- `packages/contracts/src/orchestrator.ts` — `started`, `CoachHistoryPage`.
- `apps/api/internal/agent/studiostate.go` — `started` field + default.
- `apps/api/internal/agent/orchestrator.go` (+ prompt file) — opening section,
  sentinel handling.
- `apps/api/internal/api/coach.go` — opening turn, `started` transition,
  paginated history handler, subagent/compaction loading semantics unchanged
  server-side (loading is a client concern).
- `apps/api/internal/store/queries/chat.sql` — paginated display query
  (`limit` + `before` cursor).
- `apps/api/internal/store/migrations/00NN_studio_started.sql` — column-less
  (flag lives in the existing `studio_state` jsonb); migration only backfills
  `started=true` for projects with an existing studio thread.
- `apps/web/src/studio/ai/StudioCoachChat.tsx` — remove `CHAT_INTRO`, recap
  card, 载入更早, 开始 button, loading hints.
- `apps/web/src/workspace/WorkspaceContainer.tsx` — start gate (hide switcher
  when `!started`), fire opening turn, paginated load, single-thread coach
  panel across 提案/管理/写作.
- `apps/web/src/workspace/blocks/PlanBlock.tsx` — remove `INTRO_ZH` + scripted
  chips.
- `apps/web/src/ui/feedback.tsx` and/or `Icon.tsx` — tab buttons with icons +
  ≥14px (either a new switcher or an icon+size variant).
- `apps/web/src/workspace/api/workspace.ts` — paginated `getCoachHistory`,
  opening/start calls.

## Testing

- **Go:** history pagination (limit/`before`/`hasMore`/recap-only-on-first-page,
  recap null when no digest); `started` default false + backfill migration sets
  it true for projects with a studio thread and false for empty ones; opening
  turn keeps a tool closed; start turn sets `started=true` and opens a tool.
- **Web (vitest + tsc):** switcher hidden when `!started`; 开始 button present
  after opening and reveals tabs; recap card renders when `recap` set; 载入更早
  prepends and hides at `hasMore=false`; the coach panel shows the same thread
  across 提案/管理/写作; tab buttons render icon + ≥14px; loading hints render for
  hidden subagents.
- Pre-existing `TestWeeklyReportForSeededClass` is calendar-flaky — ignore.

## Non-goals / 铁律 held

- No AI 代写正文; 印记 still 陪想查论证, one thing at a time.
- No addictive mechanics.
- Explicit subagents (paper, reflection) keep their own presence by design —
  "one chat history" governs 提案/管理/写作 and the removal of the *hidden*
  find_sources chat, not the deliberate explicit subagents.
- No new fancy document editor; 写作面 stays plain.
