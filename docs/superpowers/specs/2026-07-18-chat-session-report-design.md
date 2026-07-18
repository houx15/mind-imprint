# A2 — Chat Session Report · Design

> Second sub-project of the cross-surface assessment track (A1 → **A2** → A3 → B → C).
> A1 (course session report, merged `8509940`) is the parallel this follows closely.
> The V4 model migration (`a03682e`) landed between A1 and A2; the flagship tier is now `deepseek-v4-pro`.

**Goal:** Give a chat thread a student-opt-in, in-surface assessment — a per-thread "思维印记" — and in doing so scope chat's event writes so chat evidence stops being unreachable, closing the temporary `surface='chat'` exemption A1 parked in `event_scope_ck`.

**Why now:** After A1, course evidence is reachable but chat's three event writes are still unscoped (`project_id`/`session_id` both NULL, no thread column on `event`), so nothing can read a single thread's evidence. A1 added a deliberately temporary exemption arm to `event_scope_ck` so enforcement wouldn't silently kill chat's best-effort writes a slice before chat had a scope to satisfy it. A2 is that slice.

---

## 1. Context (verified against the code)

- **`chat_thread` is a real table** (`0016_refactor2_foundations.sql:118-125`): `id` server-minted, `user_id NOT NULL`, `title`, `seeded_project_id`, `created_at`. Messages live in `chat_message` (`thread_id NOT NULL`). So a per-thread report has a clean, owned anchor — no opaque/client ids.
- **`event` has no `thread_id` column.** `AppendEvent` (`store/queries/event.sql`) inserts `(project_id, user_id, session_id, surface, type, payload)`. Chat's three writes all set `surface='chat'`, `project_id` NULL, `session_id` NULL:
  - `prompt_sent` — `internal/api/chat.go:195` (direct `AppendEvent`), error → `slog.Warn`.
  - `card_completed` — `internal/api/chat.go:269` (direct `AppendEvent`), error → `slog.Warn`.
  - `card_surfaced` — `internal/agent/chat_step.go:181` via `ChatStore.InsertUserEvent` seam, payload `{"card_id":…}`, error → `slog.Warn`.
- **`card_instances` / `material` already carry `thread_id`** (migration `0022`, scope CHECK `num_nonnulls(task_id, project_id, thread_id) >= 1`). Chat card dispositions are recorded on `card_instances.status` (`completed`/`skipped`), thread-scoped — NOT in the Studio `disposition`/`intervention` tables. `skipChatCard` writes **no event** (status-only).
- **`ChatStore` seam** (`chat_step.go:72-81`, impl `chatstore.go`) exposes the scopeless `InsertUserEvent(ctx, userID, surface, typ, payload)` — the chat analog of the course `InsertUserEvent` A1 replaced with `InsertSessionEvent`. Its impl comment names the debt: *"chat has no session scope until A2 (DEC-A1.2)"*.
- **The exemption arm** (`0024_session_assessment_scope.sql:26-36`): `CHECK (surface = 'chat' OR num_nonnulls(project_id, session_id) >= 1) NOT VALID`, with the in-schema note *"A2 adds thread_id, scopes chat's writes, and MUST delete this arm."*
- **成长报告 binding design** (`docs/design/思维印记_工作区.dc.html:1516`): chat appears only as three aggregate counters — `对话数` / `被追问后返工` / `触发思考工具` — under `对话足迹 · 也是评估的一部分`. **No per-thread chat report surface is drawn.** So the in-surface report A2 builds is a new surface; its treatment follows A1's `CourseReport` precedent. Chat privacy line: `你的对话只属于你` (line 662).

---

## 2. Decisions

### DEC-A2.1 — Thread scope on `event` and `evaluations` (migration 0025)
Add `thread_id uuid REFERENCES chat_thread(id) ON DELETE CASCADE` to both `event` and `evaluations`, additive, mirroring how `0024` added `session_id`. Indexes: `event_thread_created_idx (thread_id, created_at)`, `evaluations_thread_idx (thread_id, created_at DESC)`.

### DEC-A2.2 — Close the exemption arm; keep `NOT VALID`
`event_scope_ck` is dropped and re-added as:
```
CHECK (num_nonnulls(project_id, session_id, thread_id) >= 1) NOT VALID
```
The `surface='chat'` arm is **gone**. `NOT VALID` is retained for the identical reason A1 gave: pre-A2 chat/course rows were written with no scope at all and there is nothing to backfill FROM — `NOT VALID` grandfathers them while enforcing every new insert. Consequence, and the point of the slice: **from 0025 on, a chat event without a `thread_id` is rejected** (`NOT VALID` skips existing-row validation at creation time but enforces all subsequent inserts). The arm's closure is thereby load-bearing, not cosmetic.

`evaluations_scope_ck` widens to `num_nonnulls(task_id, project_id, session_id, thread_id) >= 1` (ordinary widening; every existing row still satisfies it).

**Down** mirrors 0024's careful form and is not optional:
1. `DELETE FROM evaluations WHERE thread_id IS NOT NULL AND task_id IS NULL AND project_id IS NULL AND session_id IS NULL;` — thread-scoped reports have no other scope and would violate the restored CHECK, aborting the whole Down exactly where a chat report exists.
2. Restore `evaluations_scope_ck` to its 0024 form (`num_nonnulls(task_id, project_id, session_id) >= 1`).
3. Restore `event_scope_ck` to its 0024 form **including the `surface='chat'` arm** — dropping `thread_id` reverts chat writes to unscoped, so the exemption must return or the restored constraint would reject new chat inserts.
4. Drop indexes and the two `thread_id` columns.
This gets a seeded Down test (a thread-scoped evaluation present before `goose.DownContext`), the way 0024's Down test was fixed in the A1 whole-branch review.

### DEC-A2.3 — Scope all three event writes; delete the scopeless seam
`AppendEvent` gains a `thread_id` param (as it gained `session_id` in A1). The two direct handler writes (`chat.go:195`, `chat.go:269`) pass the thread id already in scope. The seam write replaces `ChatStore.InsertUserEvent(ctx, userID, surface, typ, payload)` with `InsertThreadEvent(ctx, threadID, typ, payload)`, which resolves `user_id` from `chat_thread` and hard-codes `surface='chat'` — the same move A1 made (`InsertSessionEvent`) to delete the scopeless signature so no caller can silently write an unscoped row again. New read query `ListEventsByThread` (`thread_id = $1 ORDER BY created_at`).

Adding a field to `AppendEventParams` is a keyed struct literal change: existing project/studio/course call sites keep compiling with `thread_id` defaulted to `{Valid:false}` (correct — they carry `project_id`/`session_id`), but the compiler cannot flag a chat site that *should* set `thread_id` and doesn't. The plan must **grep every `AppendEvent`/`AppendEventParams` site** (not trust compilation) and verify each chat write sets `thread_id` — the exact trap A1's Task 2 hit.

### DEC-A2.4 — Reuse A1's assessor and rubric unchanged; do not pre-empt B
`buildAssessmentInputFromThread(events []studio.Event, cards []sqlc.CardInstance) agent.AssessmentInput` parallels `buildAssessmentInputFromSession`: chat evidence = student turns (`prompt_sent`) + card offers/dispositions (`ListCardInstancesByThread`); `gates`/`wordCounts`/`reviewBands`/`graph` empty; `Spont` left empty (not fabricated). Same `agent.Assess` + `rubric.CT()` + `agent.EmbeddedAnchors()`, same **flagship `EvalResolver` (never downgraded** — now `deepseek-v4-pro`). Output dimensions render `NA` where chat produced no writing evidence (expected — chat is coach-alone; `NA` renders as 未涉及, never `L0`, never hidden).

The binding design frames chat's distinct value as "问题有没有变得更锋利" — the 提示词透镜 / 追问深度 territory that **B** redesigns. A2 reuses the CT rubric as a clean parallel to A1 and bakes in **none** of B's dimension decision, holding the same discipline A1 held (the 9-vs-10-vs-6 dimension inconsistency is B's to settle).

### DEC-A2.5 — Opt-in, student-triggered only (铁律 2)
The report is generated only when the student explicitly asks. **No auto-generation, no nudge, no badge.** This is why chat's trigger differs from A1/course (which finishes) and A3/project (which finishes): a chat thread never ends, so the student chooses when a thread is worth a print. Re-clicking regenerates (a fresh flagship call) — an explicit, consented choice, latest-wins.

### DEC-A2.6 — In-surface `ChatReport`, modeled on `CourseReport`
A `生成本次对话的思维印记` button appears in the chat thread once the thread has a real exchange (**≥1 assistant message** — a light client-side gate so flagship calls stay meaningful; an empty thread produces an all-`NA` report and wastes a flagship call). Click → opens the report panel; the panel GETs on mount, shows a stored report if one exists, else runs one `POST` (the opening click is the opt-in), with an explicit `重新生成` for a fresh run and `返回对话` to close. The panel renders a `本次对话评估` block: per-dimension L1–L4 bars, `未涉及` for `NA` dims, and the narrative.

**Scope trim (plan reconciliation):** the `收集到的工具` tile is **not** in A2's chat report. Rendering it would require a new thread-cards GET endpoint (chat exposes none to the client today, unlike course's `getCourseSession().collectedCards`), and the card-disposition evidence already feeds the assessment's dimensions — a separate decorative tile is redundant with the diagnostic and not worth a new endpoint. Deferred to the same follow-up as the 成长报告 aggregate (which is where `触发思考工具` counts belong anyway).

### DEC-A2.7 — Skips stay status-only (YAGNI)
`skipChatCard` continues to emit no event. A skip is already visible to the assessor through `card_instances.status='skipped'` (the same channel A1 read course card dispositions from), so a dedicated `card_skipped` event would be redundant evidence. 铁律 4 (过程即数据) is satisfied by the card row; no new write.

### DEC-A2.8 — Endpoints, entitlement, ownership
- `GET /api/v1/chat/threads/{id}/assessment` → latest thread evaluation, or JSON `null` (200) when none — the report view's own empty state, never a 404. No model call, ever.
- `POST /api/v1/chat/threads/{id}/assessment` → `HasEntitlement`-gated (the token-consuming rule), one flagship call, cost recorded via `RecordChatLLMCall` (surface=`chat`) **regardless of outcome** (a rejected call still cost money), persist thread-scoped via `InsertThreadEvaluation`, return DTO.
- **`RecordChatLLMCall` gains a `purpose` param**, mirroring what A1 did to `RecordCourseLLMCall`. Today it hard-codes `Purpose: "coach"` (`chatstore.go:117`); recording the flagship assessment call through it unchanged would misattribute assessment cost as coach cost in `llm_usage`. The seam signature, the `ChatStore` interface (`chat_step.go:80`), the fake in tests, and the one existing caller (`chat_step.go:157`, passes `"coach"`) all update; the generate endpoint passes `"assessment"`.
- Ownership via the existing `loadOwnedThread` (404-not-403). Reuse the shared `dtoFromEvaluationRow` and `GetLatestThreadEvaluation`.

---

## 3. Red lines (compliance)

- **RL-5** — the report is diagnostic evidence, never a grade/rank/verdict: per-dimension levels + narrative only, no total/sum/aggregate field exists on the DTO or the row. (Aggregation across threads is C's job, not A2's.)
- **铁律 1 (AI 克制)** — the assessor diagnoses process; it does not judge a "correct answer." Same posture as A1.
- **铁律 2 (不操纵)** — opt-in only, no auto-trigger, no nudge, no streak/badge for generating (DEC-A2.5).
- **评估走旗舰模型绝不降级** — `EvalResolver` only; never the chat coach's chaperone resolver. (A1's whole-branch review found both assessors had been on the chaperone tier since Slice 10; A2 must not reintroduce that — assert the tier in a test.)
- **Privacy** — the report is the student's own, shown only in-surface (binding design `你的对话只属于你`). No teacher/aggregate exposure in A2. Secrets stay server-side (unchanged).
- **记录档位 + token + 成本** — every flagship call records tier/token/cost via `RecordChatLLMCall`, even on rejection.

---

## 4. Testing

- **Migration 0025 up + seeded Down** — Down test seeds a thread-scoped evaluation before `goose.DownContext` and asserts the rollback succeeds (the load-bearing case A1's Down test originally missed).
- **Event scoping** — all three writes carry `thread_id`; and a new chat event inserted **without** a `thread_id` is now **rejected** by `event_scope_ck` (proves the arm is gone and enforced for new rows). Run the FULL `store`/`api`/`agent` packages (a card/gate/projection-adjacent change), never `-run` subsets.
- **Assessor input builder** — `buildAssessmentInputFromThread` maps chat events + cards; output dims empty → `NA`; `Spont` empty, not fabricated.
- **Endpoints** — GET returns `null` when unassessed; POST generates + persists + returns; entitlement-false blocks (note the untestable seam — `HasEntitlement` is a package-level `return true, nil` with no injection point; carry-forward from A1, log rather than fake); ownership 404; tier is `flagship`.
- **Web** — button gated on ≥1 assistant message; opening the panel GET-then-POST-once renders the report; a stored report rehydrates via GET-on-mount without a second model call; `重新生成` triggers a fresh POST.

---

## 5. Carry-forwards (explicitly NOT built here)

- **成长报告 aggregate + history entrance** — chat's `对话数/被追问后返工/触发思考工具` counters and the per-thread report drill-in are **C**'s work. A2 produces the per-thread evaluation rows C will aggregate; it does not touch 成长报告.
- **B's chat lens** — the 提示词透镜 / 追问深度 chat-specific dimensions are B's rubric redesign; A2 stays on the CT rubric.
- **`收集到的工具` tile in the chat report** — needs a thread-cards client endpoint chat doesn't expose; redundant with the assessment's own card-disposition evidence. Deferred with the 成长报告 aggregate (C).
- **Entitlement-false test** — no injection seam exists (A1 carry-forward); logged, not faked.
- **Multimodal / off-record chat, thread evidence nodes in the process tree** — Slice 11 deferrals, still deferred.
- **Latent cost-validity bug (noted, not fixed here)** — both `RecordChatLLMCall` (`chatstore.go:120`) and `RecordCourseLLMCall` (`coursestore.go:221`) call `gateway.CostNumeric(cost, true)` — hard-coding `true` even when the model is unpriced, so an unknown model would record `$0` as a *valid* number rather than NULL. Latent post-V4-migration (every current model is priced). A2 adds a `purpose` param to `RecordChatLLMCall` but deliberately does **not** widen scope into a cross-cutting cost-correctness fix touching both recorders; logged for a dedicated follow-up.
