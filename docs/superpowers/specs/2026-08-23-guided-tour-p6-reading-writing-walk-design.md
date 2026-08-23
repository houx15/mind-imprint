# Guided Tour P6 — Reading-room & Writing-room real-scene walk · Design

**Status:** DESIGNED (2026-08-23) · builds on P5 ([[guided-tour-p5-real-scene-2026-08-23]], HEAD `1b0eb2c1`)
**Spec author:** houyx15
**Scope owner doc for demo behaviour:** `docs/2026-08-09-all-statuses.md` (writing-project status flow)

## Why

Post-P5 user feedback (2026-08-23):

1. The projects journey never walks students into the **reading room** — one of the most
   important features. The current tour walks the exploration graph + library table but only
   *narrates* 精读 (2 centered steps) and never shows reading a paper with AI, searching
   literature, or getting AI source suggestions.
2. The **writing room** walk is too shallow: it never shows the **写作卡 / 片段引导**
   (snippet guidance), and never shows **AI 批注** (margin comments) — students don't see
   what AI feedback on their writing looks like.
3. Students are never told about the **finish button**: after finishing/定稿 they can no
   longer change their writing.

## Decision (user-confirmed)

**Keep the demo project read-only. Seed everything a student needs to view or know**, and
walk the tour through the *real* read-only rooms — real-UI, not centered explanations, and
not a per-user writable clone. (A per-user writable demo was explicitly rejected: bigger
build, real token cost, and it re-opens the "users break the demo" surface P5 closed.)

The demo project (`00000000-0000-0000-0000-000000000200`, `is_demo=true`, status
`finished`) is world-readable; **every non-GET request 403s** via `loadOwnedProject`
(`apps/api/internal/api/projects.go:202-212`). Token-spending endpoints short-circuit to
canned fixtures (`apps/api/internal/api/demo.go`). The design works within that: seed real
data readable over GET, make the two live-interaction moments read-only-safe via the
existing canned-demo pattern, and never trigger a write from the tour.

## What is / isn't reachable (from code investigation)

| Feature | Backing | Seedable read-only? | Approach |
|---|---|---|---|
| AI 批注 on essay | `intervention` table, `type='essay_annotation'`, read via `GET /projects/{id}/proposal-annotations?doc=essay` | **Yes** | Seed rows in a new migration |
| 精读 immersive room | `ReadingRoom` opened by `setReadingSource(MaterialSource)`; no GET returns a `MaterialSource` (only `enter-reading`, a POST that 403s) | Needs small GET | Add read-only `GET …/materials/{mid}/source` |
| Reading-with-AI chat | `ReadingRoom` chat is client state seeded to a greeting; `read-turn` POST 403s on demo; no history fetch | Not DB-seedable | Seed a demo transcript into the room's initial messages (frontend), disable send on demo |
| Literature-search AI suggestions | `explore-suggestions` shows live `DigCandidate[]`; demo `dig` short-circuits to `cannedDig()` = empty (no 403) | Via canned | Make `cannedDig()` return 3 real candidates |
| 写作卡 / 片段引导 | `GuidedWritingCard` real UI, anchor `writing-aicard` exists | Already real | Spotlight in tour |
| Finish button + lock | `完成写作` (`WritingBlock`) + `定稿并开始评估` (`ReviewBlock`), real UI, disabled on demo | Already real | Spotlight + quote real lock copy in bubble |

## Design

### Backend

**B1 — Seed real AI 批注 (migration `0083_seed_demo_essay_annotations.sql`).**
Insert 5–7 `intervention` rows for the demo essay:
`project_id='…0200'`, `type='essay_annotation'`, `card_instance_id=NULL`,
`body=<note text>`, `anchor` jsonb =
`{"docKind":"essay","level":<paper|paragraph|sentence>,"nature":<good|suggest|problem>,"quote":<sentence text>,"locator":"第N段"}`,
`criterion=<level>`, `level=<nature>` (the read path reconstructs from `anchor`; columns are
projections per `agentstore.go:706-738`). `quote` values must match sentences in the seeded
essay `edit_buffer` (`…02f0`, `doc_kind='essay'`, seeded in `0082`). Include a mix of
natures: at least one `good` (e.g. praising the concession段), one `suggest`, one `problem`
(e.g. flags the China-carbon counter-example). Read verification: `GET
/projects/…0200/proposal-annotations?doc=essay` returns them (GET is allowed for demo).

**B2 — Real literature-search suggestions (`demo.go` `cannedDig`).**
Change `cannedDig()` (`apps/api/internal/api/demo.go:57-59`) from `{candidates: []}` to 3
real `DigCandidate` sources relevant to the demo research question (中国是否让地球更可持续),
so clicking a find action in the real graph populates the real 采纳/丢弃 panel. No 403 (the
`digExploration` handler short-circuits on `IsDemo` before `loadOwnedProject`,
`exploration.go:633`). Match the `DigCandidate` shape exactly. Adopt/丢弃 still POST → the
tour never clicks them, only spotlights.

**B3 — Read-only material-source GET (`GET /projects/{id}/materials/{mid}/source`).**
New handler returning the `MaterialSource` projection for a material, **GET-only, no write,
no `appendAutoLog`** (unlike `enter-reading`). Reuse the projection used by `enter-reading`
(`studio.ProjectMaterials` → `MaterialDTO`, `workspace_library.go:675-764`) minus the write.
Routed through the demo-allowed GET path (world-readable via `loadOwnedProjectRow`). Returns
the full required `MaterialSource` (14 fields, `packages/contracts/src/studioState.ts:111-161`).
Frontend api client: `getMaterialSource(projectId, materialId): Promise<MaterialSource>`.

### Frontend

**F1 — ReadingRoom demo replay props.**
`ReadingRoom` accepts optional `initialMessages?: ChatMessage[]` and `demoMode?: boolean`.
`readingLoop` seeds `messages` from `initialMessages` when provided (else `[GREETING]`).
When `demoMode`, the send box is disabled with a read-only hint (mirror the demo-disable
pattern used elsewhere) so no `read-turn` 403 can fire. A demo reading transcript
(short, real-content exchange about the Nature Sustainability paper, using a 思维卡 lens)
lives in a tour/demo fixture, not in the core loop.

**F2 — Wire `openDemoReadingRoom()` for real.**
Replace the stub (`shell/StudentApp.tsx:165-172`, `tour/segments/projects.ts:176-186`).
`openDemoReadingRoom()` now: switch to the reading room, `getMaterialSource(DEMO,
'…0271')`, then `setReadingSource(...)` with the demo transcript + `demoMode`. The immersive
`ReadingRoom` renders read-only (mount only GETs `open-card`; no auto-write on mount). Keep a
graceful fallback to `setReadingView("list")` if the GET fails, so the tour never dead-ends.

**F3 — New `[data-tour]` anchors.**
- `writing-finish` on the 完成写作 button (`WritingBlock.tsx:397`).
- `writing-annotations` on the AI批注 tab / `ProposalAnnotationGroup` (`ReferencePanel.tsx`).
- `review-finalize` on the 定稿并开始评估 button (`ReviewBlock.tsx:283`).
- `explore-find` on the find-actions row if `explore-keyword` alone isn't enough to spotlight
  the find controls (`ExplorationSidebar.tsx:163-178`).
(Reuse existing: `rr-article`/`rr-chat`/`rr-deck`/`rr-finish`/`rr-notes`, `writing-aicard`,
`explore-keyword`, `explore-suggestions`, `warren-question`.)

**F4 — Expand `tour/segments/projects.ts`.**
- **reading-warren (search + suggestions):** after walking the graph, an **action step**
  selecting a question node (click `warren-question`) so the NodePanel opens, then spotlight
  `explore-keyword` + find controls ("用关键词或‘找相似’让印记帮你补充来源"), then an
  **action step** clicking 找相似 → `cannedDig` real candidates → spotlight
  `explore-suggestions` ("印记给出候选来源，你来决定采纳或丢弃"). Fallback to narrating over
  the real controls if the live dig proves fragile in smoke.
- **reading-room (精读, real):** replace the 2 centered narration steps. `onEnter:
  openDemoReadingRoom()`; spotlight `rr-article` (真实论文正文), `rr-chat` (边读边问印记 —
  showing the seeded transcript), `rr-deck` (思维卡 lens deck), `rr-notes` (我的笔记),
  `rr-finish` (读完标记这篇).
- **writing (deeper):** keep tabs step; add spotlight on `writing-aicard`
  (片段引导/写作卡 — "印记把每个论证部分拆成可填的引导，正文仍由你自己写" — 铁律① safe copy),
  add spotlight on `writing-annotations` (real seeded 批注 — "印记通读后给出的分层批注：绿色
  是亮点、蓝色是建议、红色是要处理的问题；点一条会跳到正文对应句"), add spotlight on
  `writing-finish` (完成写作 — quote lock copy: 锁定初稿、解锁回顾，之后仍可重新打开).
- **reflection (finish/lock warning):** add spotlight on `review-finalize` (定稿并开始评估 —
  quote the real point-of-no-return copy: "定稿后，正文与回顾都会锁定、无法再修改" — this is
  the student's answer to "after finish you can't change your writing").

### 铁律 compliance

Writing-room copy describes AI as *reviewing/annotating/scaffolding*, never writing body
text (铁律①). Read-only throughout (no writes triggered). No dark patterns. The 批注 shown are
real seeded feedback, the suggestions are real candidates the student decides on (采纳/丢弃),
matching "AI 克制，绝不替学生定论".

## Testing

- Backend: migration applies; `proposal-annotations?doc=essay` returns seeded 批注 for demo;
  `cannedDig` returns 3 candidates; `GET …/materials/{mid}/source` returns a valid
  `MaterialSource` for the demo material and is reachable by a non-owner over GET; still 403 on
  any non-GET. Run `internal/api` + `internal/agent` (card/agent goldens) suites.
- Frontend: segment tests for the new reading/writing/reflection steps (anchors exist, action
  events target real selectors); ReadingRoom renders with `initialMessages`+`demoMode` (send
  disabled); `openDemoReadingRoom` fallback path. Full web suite green.
- Prod-style browser smoke of the whole projects journey (jsdom can't hit-test action clicks):
  demo → reading graph search/suggestions action clicks advance → immersive 精读 opens with
  real paper + transcript → writing 片段引导 + 批注 + finish spotlights → reflection 定稿
  warning. Confirm 0 console errors (no stray 403).

## Out of scope

- Per-user writable demo / live LLM in the tour.
- A canned `read-turn` SSE endpoint (replaced by the seeded transcript + disabled send).
- Persisting reading chat history (ephemeral by design).
