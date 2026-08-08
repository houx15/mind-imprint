# 写作空间 onboarding + 单线程 + subagent 分层 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make first contact with a project a real 印记 conversation, keep the chat a single continuous thread the student can trust, gate the five rooms behind an explicit 开始, page long histories with a recap, redesign the tab buttons, and make the full title viewable.

**Architecture:** The 印记 orchestrator (`apps/api/internal/agent/orchestrator.go`) is the one posture; `studio_state` jsonb on `project` holds AI-managed workspace state. We add a `started` flag, two onboarding endpoints (opening + start), cursor pagination on the display history with a digest-fed recap, and frontend changes to the workspace shell (`apps/web/src/workspace/*`, `apps/web/src/studio/ai/*`). Contracts are the single shared truth (`packages/contracts/src/orchestrator.ts`).

**Tech Stack:** Go (`apps/api`, net/http + sqlc@v1.27.0 + goose + pgx + testcontainers) · Zod contracts (`packages/contracts`) · React 18 + Vite + TS + Tailwind (`apps/web`, vitest).

## Global Constraints

- **Design铁律:** AI 克制、绝不代写正文、一次只问一个、过程即数据. The opening is ONE framing message (not a barrage). No addictive mechanics.
- **12px rule:** only genuine *hints* may be `text-mk-small` (12px). Buttons/nav are ≥14px (`text-mk-body`). Subagent loading hints ARE hints → 12px allowed.
- **Subagent hint copy uses the English word `subagent`, never 子代理.** e.g. `subagent 正在检索来源…`, `subagent 正在整理研究计划…`. Compaction: `正在整理对话…` / `已整理较早的对话`.
- **GET must not spend tokens.** Recap reuses the stored `conversation_digest.prose`; never an LLM call on a history GET.
- **One thread:** the coach panel shows the single `studio`-surface thread across 提案/管理/写作. Explicit subagents (a specific paper in 阅读, and 回顾) keep their own presence by design; the *hidden* find_sources chat is removed.
- **Go on macOS:** `CGO_ENABLED=0`; Go tests run FOREGROUND (docker/testcontainers) — scoped `-run`. Pre-existing `TestWeeklyReportForSeededClass` is calendar-flaky — ignore. Never `git add -A`; stage explicit paths.
- **Web:** pnpm; `pnpm exec vitest run` (the `run` is mandatory — no watch); `pnpm exec tsc --noEmit` must be 0. Tests live in `apps/web/test/**`.
- **Migration number is 0059** (last is `0058_project_cover.sql`). The `started` flag lives inside the existing `studio_state` jsonb — no new column; the migration only changes the jsonb default and backfills.
- **Deploy after merge = `full`** (api + web; migration 0059 present).

---

## File Structure

- `packages/contracts/src/orchestrator.ts` — `StudioState.started`, `CoachHistoryPage`, `OrchestratorReply.planGenerated`/`compacted`.
- `apps/api/internal/agent/studiostate.go` — `Started` field + default.
- `apps/api/internal/agent/orchestrator.go` — opening system prompt + `ProposeOpeningTurn`.
- `apps/api/internal/store/migrations/0059_studio_started.sql` — jsonb default + backfill.
- `apps/api/internal/store/queries/chat.sql` — paginated display queries; regenerate sqlc.
- `apps/api/internal/api/coach.go` — `postCoachOpening`, `postCoachStart`, paginated `getCoachHistory`, `planGenerated`/`compacted` on reply.
- `apps/api/internal/api/api.go` — two new routes.
- `apps/web/src/workspace/api/workspace.ts` — paginated `getCoachHistory`, `coachOpening`, `coachStart`.
- `apps/web/src/studio/ai/StudioCoachChat.tsx` — remove `CHAT_INTRO`; recap card; 载入更早; 开始 button; subagent hints.
- `apps/web/src/workspace/WorkspaceContainer.tsx` — start gate, opening/start wiring, paginated load, one-thread panel, full-title top bar.
- `apps/web/src/workspace/blocks/PlanBlock.tsx` — remove `INTRO_ZH` + scripted chips.
- `apps/web/src/workspace/blocks/ReadingBlock.tsx` — find_sources loading + hint (hidden subagent), no chat.
- `apps/web/src/workspace/RoomSwitcher.tsx` (new) or `apps/web/src/ui/feedback.tsx` — tab buttons with icons + ≥14px.
- `apps/web/src/shell/home/HomePage.tsx`, `apps/web/src/workspace/Directory.tsx` — `title` attr on clamped titles.

---

## Task 1: Contracts — `started`, `CoachHistoryPage`, reply flags

**Files:** `packages/contracts/src/orchestrator.ts`; Test: `packages/contracts` build + a vitest in `apps/web/test/` if contracts are tested there, else a `packages/contracts` unit test.

**Interfaces — Produces:**
- `StudioStateSchema` gains `started: z.boolean()` (no `.default()` on the wire — the server always sends it; parsing legacy without it must fail loudly in tests, but the API always emits it).
- `CoachHistoryPageSchema` = `{ messages: CoachHistoryMsg[], hasMore: boolean, recap: string | null, nextCursor: string | null }`.
- `OrchestratorReplySchema` gains `planGenerated: z.boolean()` and `compacted: z.boolean()`.

- [ ] **Step 1: Read** `packages/contracts/src/orchestrator.ts` fully to match existing style (how `StudioState`, `OrchestratorReply`, and the existing coach-history message type are declared).

- [ ] **Step 2: Add `started` to `StudioState`.** In the `StudioState` schema (the object with `stage`, `openTool`, `widthTier`, `reference`, `updatedAtTurn`), add:
```ts
started: z.boolean(),
```
Place it after `updatedAtTurn` (field order is cosmetic; keep the type export in sync).

- [ ] **Step 3: Add the paginated history page type.** Find the existing coach-history message type (the `{ role, text, card? }` shape returned by `GET /coach/history`; if it isn't in this file, it may be in a `coach.ts` — search `packages/contracts/src` for `role` + `card`). Reuse it as `CoachHistoryMsg`. Add:
```ts
export const CoachHistoryPageSchema = z.object({
  messages: z.array(CoachHistoryMsgSchema),
  hasMore: z.boolean(),
  recap: z.string().nullable(),
  nextCursor: z.string().nullable(),
});
export type CoachHistoryPage = z.infer<typeof CoachHistoryPageSchema>;
```
(If the message schema lives elsewhere, import it; do not duplicate it.)

- [ ] **Step 4: Add reply flags.** In `OrchestratorReplySchema` add:
```ts
planGenerated: z.boolean(),
compacted: z.boolean(),
```

- [ ] **Step 5: Build + test.** Run `pnpm --filter @mindimprint/contracts build` (or the repo's contracts build script — check `package.json`), then `pnpm exec tsc --noEmit` at `apps/web`. Add/extend a vitest that parses a sample `CoachHistoryPage` and a `StudioState` with `started`, and asserts a `StudioState` missing `started` FAILS `.parse`. Run `pnpm exec vitest run` (scoped to the new test file).

- [ ] **Step 6: Commit** `feat(contracts): started flag, CoachHistoryPage, reply planGenerated/compacted`.

---

## Task 2: Go — `StudioState.Started` + migration 0059 (default + backfill)

**Files:** `apps/api/internal/agent/studiostate.go`; `apps/api/internal/store/migrations/0059_studio_started.sql`; Test scoped `internal/agent` + an `internal/store` migration test (docker foreground).

**Interfaces — Consumes:** none. **Produces:** `StudioState.Started bool` (`json:"started"`), default `false`; migration sets the jsonb default to include `"started":false` and backfills `started=true` for every project that already has a `studio`-surface chat message.

- [ ] **Step 1: Add the field.** In `studiostate.go`, add to `StudioState`:
```go
Started bool `json:"started"`
```
after `UpdatedAtTurn`. In `DefaultStudioState()` add `Started: false,`.

- [ ] **Step 2: Test the Go default + legacy unmarshal.** In `apps/api/internal/agent/studiostate_test.go` (create if absent):
```go
func TestStudioStateStartedDefault(t *testing.T) {
	if DefaultStudioState().Started {
		t.Fatal("default Started must be false")
	}
	// legacy jsonb without the key → Started false
	var s StudioState
	if err := json.Unmarshal([]byte(`{"stage":"topic_discussion","openTool":"chat","widthTier":"chat","reference":[],"updatedAtTurn":0}`), &s); err != nil {
		t.Fatal(err)
	}
	if s.Started {
		t.Fatal("legacy state must unmarshal Started=false")
	}
}
```
Run `CGO_ENABLED=0 go test ./internal/agent/ -run TestStudioStateStarted -count=1`.

- [ ] **Step 3: Write migration 0059.** Read `0057_studio_state.sql` first to copy the goose header/style and the exact current default JSON. Create `0059_studio_started.sql`:
```sql
-- +goose Up
-- studio_state gains a `started` flag: false until the student clicks 开始.
-- New projects default to false; existing projects that already have a
-- studio-surface conversation are mid-journey and must NOT be re-gated.
ALTER TABLE project
  ALTER COLUMN studio_state
  SET DEFAULT '{"stage":"topic_discussion","openTool":"chat","widthTier":"chat","reference":[],"updatedAtTurn":0,"started":false}'::jsonb;

-- Ensure the key exists on every row (false where absent)...
UPDATE project
  SET studio_state = jsonb_set(studio_state, '{started}', 'false'::jsonb, true)
  WHERE NOT (studio_state ? 'started');

-- ...then flip to true for projects with an existing studio thread.
UPDATE project p
  SET studio_state = jsonb_set(p.studio_state, '{started}', 'true'::jsonb, true)
  WHERE EXISTS (
    SELECT 1 FROM chat_thread ct
    JOIN chat_message cm ON cm.thread_id = ct.id
    WHERE ct.seeded_project_id = p.id AND cm.surface = 'studio'
  );

-- +goose Down
ALTER TABLE project
  ALTER COLUMN studio_state
  SET DEFAULT '{"stage":"topic_discussion","openTool":"chat","widthTier":"chat","reference":[],"updatedAtTurn":0}'::jsonb;
UPDATE project
  SET studio_state = studio_state - 'started';
```
(Verify the current default string against `0057` and match it exactly before appending `"started":false`.)

- [ ] **Step 4: Migration test (docker foreground).** Find the existing migration/store test harness (grep `internal/store` for a testcontainers helper, e.g. `newTestPool`/`applyMigrations`). Add a test: seed a project + a `chat_message` with `surface='studio'`; seed a second project with none; run migrations; assert project 1's `studio_state->>'started'` is `"true"` and project 2's is `"false"`. Run it foreground scoped: `CGO_ENABLED=0 go test ./internal/store/ -run TestStudioStartedBackfill -count=1`.

- [ ] **Step 5: Commit** `feat(api): studio_state.started flag + 0059 default & backfill migration`.

---

## Task 3: Backend — onboarding endpoints (opening + start)

**Files:** `apps/api/internal/agent/orchestrator.go` (opening prompt + `ProposeOpeningTurn`); `apps/api/internal/api/coach.go` (`postCoachOpening`, `postCoachStart`); `apps/api/internal/api/api.go` (routes); Test scoped `internal/api` + `internal/agent` foreground.

**Interfaces — Consumes:** `agent.StudioState` (with `Started`, Task 2); `agent.ProposeOrchestratorTurn`. **Produces:**
- `POST /api/v1/projects/{id}/coach/opening` → `orchestratorReplyDTO` with a welcome `narrate`, `directive.started=false`, `directive.openTool="chat"`. Idempotent: if the studio thread is non-empty, returns 200 with an empty `narrate` and the current directive (no new turn, no spend).
- `POST /api/v1/projects/{id}/coach/start` → `orchestratorReplyDTO` with `directive.started=true`, `directive.openTool="forming"`, and a narrate that begins the 提案 discussion.

- [ ] **Step 1: Opening system prompt + function.** In `orchestrator.go`, add a dedicated opening prompt constant (a crafted "manual system message" — this is the real-AI-opening requirement):
```go
const orchestratorOpeningPrompt = `你是「印记」，学生刚进入这个写作项目，还没开始。用一段话欢迎他，语气温暖、克制、不啰嗦。你必须：
1) 欢迎他来到写作空间；
2) 复述你看到的题目（用投影里的项目题目，别编造；若没有题目就说「你还没定题目」）；
3) 用一句话点明：完整做完一个写作项目，会一路经过 立项 → 阅读 → 写作 → 回顾；
4) 说明我们先一起把研究计划的四件事讨论清楚，并列成一个短清单：
   - 目标（research question）
   - 缘由（motivation）
   - 活动与时间（plan）
   - 资源（resources）
5) 最后问一句：准备好开始了吗？
只输出给学生看的这段话本身，不要 JSON、不要工具、不要列出多于四条、不要连问多个问题。`

// ProposeOpeningTurn makes ONE LLM call producing 印记's welcome message. It has
// no tools and no history — just the opening posture + the spine projection
// (which carries the project title). Returns the narrate text and usage.
func ProposeOpeningTurn(ctx context.Context, prov gateway.Provider, r gateway.Resolved, spineProjection string) (string, gateway.ChatUsage, error) {
	req := gateway.ChatRequest{Messages: []gateway.ChatMessage{
		{Role: gateway.RoleSystem, Content: orchestratorOpeningPrompt},
		{Role: gateway.RoleUser, Content: spineProjection + "\n\n（这是开场，学生还没说话。）"},
	}}
	res, err := gateway.Collect(ctx, prov, r, req)
	if err != nil {
		return "", res.Usage, err
	}
	return strings.TrimSpace(res.Text), res.Usage, nil
}
```
(Add `"strings"` import if not present; it is used elsewhere in the file already — verify.)

- [ ] **Step 2: `postCoachOpening` handler.** In `coach.go`, model it on `postCoach` (owned-project + entitlement gates, resolver). Then:
  1. Load history for the studio thread; if it already has ANY turn, return `orchestratorReplyDTO{Narrate:"", Directive: <current or default state>}` at 200 (idempotent, no spend). Use `store.LoadActiveCoachHistory(ctx, projectID, 1)` and check length, OR a lighter count query — prefer the existing `ListChatMessagesByProjectSurface` with surface `studio` and check `len==0`.
  2. Build spine projection (scope `forming`).
  3. `narrate, usage, err := agent.ProposeOpeningTurn(...)`. Meter when `usage>0` (surface `studio`, purpose `opening`). On error/empty, fall back to a restrained static welcome (a `const openingFallback` string that still names the four things — used ONLY when the LLM fails).
  4. Load or default the state; force `state.OpenTool = agent.ToolChat`, `state.WidthTier = agent.WidthChat`, keep `state.Started=false`; persist it (`SetStudioState`).
  5. Persist the assistant narrate under surface `studio` (NO student turn).
  6. Return `orchestratorReplyDTO{Narrate: narrate, Directive: state}` (note/card/question nil, planGenerated/compacted false).

- [ ] **Step 3: `postCoachStart` handler.** In `coach.go`:
  1. Owned + entitlement gates + resolver.
  2. Load state (default if absent). If `state.Started` already true, run the normal orchestrator path is NOT needed — just return current directive (idempotent). Otherwise continue.
  3. Set `state.Started = true`, `state.OpenTool = agent.ToolForming`, `state.WidthTier = agent.WidthForTool(agent.ToolForming)`, and if `state.Stage == agent.StageTopicDiscussion` set `state.Stage = agent.StageProposalForming`.
  4. Build history (studio) + append a synthetic student turn `"我准备好了，开始吧"` so 印记 opens the 提案 discussion; DO persist this synthetic student turn under `studio` (it is a real utterance the button stands in for).
  5. Build spine projection (scope `forming`), call `agent.ProposeOrchestratorTurn(...)`, meter, apply emitted tools onto `state` EXACTLY as `postCoach` does (reuse a shared helper if the tool-apply switch is extracted; otherwise duplicate the switch — but prefer extracting `applyOrchestratorTools(...)` in Task 3 so `postCoach` and `postCoachStart` share it and cannot drift). `state.Started` stays true and `open_tool` may move it; if the orchestrator opened no tool, keep `forming`.
  6. Persist state + assistant narrate (studio) + coach_turn event + `TouchProject` + `maybeCompactBackstop`, mirroring `postCoach`.
  7. Return the reply.

- [ ] **Step 4: Extract the tool-apply switch (DRY).** Move the `for _, tc := range dec.Tools { switch tc.Name … }` body from `postCoach` into a method `func (a *API) applyOrchestratorTools(ctx, projectID, dec, state, store) (state, replyBits)` returning the mutated state plus the note/card/question/reviewRequested/planGenerated bits, and call it from both `postCoach` and `postCoachStart`. Keep behavior identical for `postCoach` (verified by existing coach tests).

- [ ] **Step 5: Routes.** In `api.go` after line 119 add:
```go
mux.Handle("POST /api/v1/projects/{id}/coach/opening", protected(a.postCoachOpening))
mux.Handle("POST /api/v1/projects/{id}/coach/start", protected(a.postCoachStart))
```

- [ ] **Step 6: Tests (docker foreground, scoped).** In `apps/api/internal/api/` (reuse the harness from `coach_continuity_test.go`):
  - `TestCoachOpening`: fresh project → POST `/coach/opening` → 200, non-empty narrate, `directive.started==false`, `directive.openTool=="chat"`; a `chat_message` (assistant, surface studio) now exists and NO student turn. Second POST `/coach/opening` → narrate empty (idempotent).
  - `TestCoachStart`: after opening, POST `/coach/start` → 200, `directive.started==true`, `directive.openTool=="forming"`, narrate non-empty; GET `/studio-state` shows started true.
  - Reuse the existing test LLM stub/mock (grep how `coach_continuity_test.go` stubs the provider). Run `CGO_ENABLED=0 go test ./internal/api/ -run 'TestCoach' -count=1` foreground.

- [ ] **Step 7: Commit** `feat(api): coach opening + start endpoints (real AI opening, explicit start gate)`.

---

## Task 4: Backend — paginated coach history + recap

**Files:** `apps/api/internal/store/queries/chat.sql` (+ sqlc regen); `apps/api/internal/api/coach.go` (`getCoachHistory`); Test scoped `internal/api` foreground.

**Interfaces — Consumes:** `GetConversationDigest` (`chat.sql:85`). **Produces:** `GET /coach/history?surface=studio&limit=N&before=<cursor>` → `{ messages:[oldest→newest within page], hasMore:bool, recap:string|null, nextCursor:string|null }`. `recap` non-null only on the first page (no `before`) when `hasMore` and a digest row exists.

- [ ] **Step 1: Add paginated queries.** In `chat.sql`, add two queries that fetch newest-first with `LIMIT $n+1` (caller detects `hasMore`), including folded turns (visible conversation). Model column selection on `ListChatMessagesByProjectSurface`:
```sql
-- name: ListChatMessagesPageLatest :many
SELECT cm.* FROM chat_message cm
JOIN chat_thread ct ON cm.thread_id = ct.id
WHERE ct.seeded_project_id = $1 AND cm.surface = $2
ORDER BY cm.created_at DESC, cm.id DESC
LIMIT $3;

-- name: ListChatMessagesPageBefore :many
SELECT cm.* FROM chat_message cm
JOIN chat_thread ct ON cm.thread_id = ct.id
WHERE ct.seeded_project_id = $1 AND cm.surface = $2
  AND (cm.created_at, cm.id) < ($3, $4)
ORDER BY cm.created_at DESC, cm.id DESC
LIMIT $5;
```

- [ ] **Step 2: Regenerate sqlc.** From `apps/api`: `sqlc generate` (ensure sqlc **v1.27.0** — `sqlc version`; the repo pins it). Confirm the generated `ListChatMessagesPageLatest`/`Before` params structs compile.

- [ ] **Step 3: Cursor helpers.** In `coach.go` add opaque cursor encode/decode (base64 of `"<unixNano>|<uuid>"`):
```go
func encodeCoachCursor(t time.Time, id uuid.UUID) string {
	return base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("%d|%s", t.UnixNano(), id.String())))
}
func decodeCoachCursor(s string) (time.Time, uuid.UUID, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil { return time.Time{}, uuid.Nil, false }
	parts := strings.SplitN(string(raw), "|", 2)
	if len(parts) != 2 { return time.Time{}, uuid.Nil, false }
	ns, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil { return time.Time{}, uuid.Nil, false }
	id, err := uuid.Parse(parts[1])
	if err != nil { return time.Time{}, uuid.Nil, false }
	return time.Unix(0, ns).UTC(), id, true
}
```
(Match the row's `created_at`/`id` Go types — `pgtype.Timestamptz` and `pgtype.UUID` or `uuid.UUID`; convert accordingly when building the `Before` params and when encoding from a returned row.)

- [ ] **Step 4: Rework `getCoachHistory`.** Parse `limit` (default 20, clamp 1..100) and `before`. Fetch `limit+1` rows via Latest (no `before`) or Before (decoded cursor; on a bad cursor return 400 `invalid_cursor`). Compute `hasMore = len(rows) > limit`; trim to `limit`. Rows are newest-first; the oldest returned row (last after trim) provides `nextCursor` (encode its created_at,id) — set `nextCursor` only when `hasMore`. Reverse rows to oldest→newest and map to `coachHistoryMsg` (keep the card-attachments projection). recap: only when `before` is empty AND `hasMore` — `GetConversationDigest(projectID)`; if found, `recap = digest.Prose`, else nil. Respond:
```go
httpx.WriteJSON(w, http.StatusOK, map[string]any{
	"messages": msgs, "hasMore": hasMore, "recap": recap, "nextCursor": nextCursor,
})
```
(`recap`/`nextCursor` are `*string` so they serialize as JSON null when unset.)

- [ ] **Step 5: Tests (docker foreground, scoped).** In `internal/api`:
  - Seed 25 studio turns. First page (`limit=20`, no before) → 20 messages oldest→newest (assert first is older than last), `hasMore==true`, `nextCursor!=null`. Follow `before=nextCursor` → remaining 5, `hasMore==false`, `nextCursor==null`, `recap==null` (before-page).
  - With a `conversation_digest` row inserted → first page `recap` == the prose; without → `recap==null`.
  - Bad `before` → 400.
  Run `CGO_ENABLED=0 go test ./internal/api/ -run 'TestCoachHistory' -count=1`.

- [ ] **Step 6: Commit** `feat(api): paginate coach history with cursor + digest-fed recap`.

---

## Task 5: Frontend — history pagination + recap card + 载入更早

**Files:** `apps/web/src/workspace/api/workspace.ts`; `apps/web/src/workspace/WorkspaceContainer.tsx`; `apps/web/src/studio/ai/StudioCoachChat.tsx`; Test `apps/web/test/`.

**Interfaces — Consumes:** `CoachHistoryPage` (Task 1), the paginated `GET /coach/history` (Task 4). **Produces:** `getCoachHistory(projectId, surface, opts?)` returns a `CoachHistoryPage`; the studio thread loads the recent page on open, shows a recap card + 载入更早 that prepends older pages.

- [ ] **Step 1: API client.** In `workspace.ts`, change `getCoachHistory` to accept `(id: string, surface: string, opts?: { before?: string; limit?: number })`, build the query string, and parse with `CoachHistoryPageSchema`. (Grep callers of the old `getCoachHistory` and update them — reading/reflection callers pass their surface and can ignore `hasMore`/`recap`.)

- [ ] **Step 2: Container state + initial load.** In `WorkspaceContainer.tsx`, replace the whole-thread studio load (around `:518-529`) with a first-page load: store `historyCursor: string|null`, `historyHasMore: boolean`, `recap: string|null`. Seed `studioMessages` from `page.messages` (keep the optimistic-guard: if `prev.length` already has in-flight turns, don't clobber — merge by seeding only when empty). Add a `loadEarlier()` that calls `getCoachHistory(pid,"studio",{before: historyCursor})` and **prepends** `page.messages`, updates cursor/hasMore, and preserves scroll position (capture `scrollHeight` before, restore delta after — via the chat scroll ref).

- [ ] **Step 3: Recap card + 载入更早 in the chat.** In `StudioCoachChat.tsx`: **remove `CHAT_INTRO`** and its fallback (`displayChat` becomes just `messages`). At the top of the thread, when `recap` is set, render a visually distinct recap card (e.g. a soft `bg-mk-paper rounded-mk-sm p-3 text-mk-body` block prefixed 「我们之前聊到」 with the `spark` icon). Above it, when `hasMore`, a 载入更早的对话 button (`text-mk-body`, not a hint) that calls `loadEarlier()` and shows an inline loading state while fetching. Wire `recap`/`hasMore`/`loadEarlier` through the existing `StudioChatContext` value (add fields) or as props.

- [ ] **Step 4: Tests (vitest).** In `apps/web/test/`:
  - Mock `getCoachHistory` to return `{messages:[…20…], hasMore:true, recap:"之前聊到X", nextCursor:"c1"}`; render the chat; assert the recap text renders and a 载入更早 control is present.
  - Click 载入更早 → mock returns an older page with `hasMore:false`; assert older messages are prepended (appear above) and the 载入更早 control disappears.
  - Assert the old `CHAT_INTRO` string never renders.
  Run `pnpm exec vitest run apps/web/test/<file>` + `pnpm exec tsc --noEmit`.

- [ ] **Step 5: Commit** `feat(web): paginated studio history with recap card + 载入更早`.

---

## Task 6: Frontend — start gate + opening/start wiring

**Files:** `apps/web/src/workspace/api/workspace.ts` (`coachOpening`, `coachStart`); `apps/web/src/workspace/WorkspaceContainer.tsx`; `apps/web/src/studio/ai/StudioCoachChat.tsx`; `apps/web/src/workspace/blocks/PlanBlock.tsx`; Test `apps/web/test/`.

**Interfaces — Consumes:** `POST /coach/opening`, `POST /coach/start` (Task 3), `StudioState.started` (Task 1). **Produces:** a new project shows pure full-width chat with NO tabs; the opening is a real AI turn ending 准备好开始了吗 with an 开始 button; clicking 开始 reveals the interactive area + 5 tabs.

- [ ] **Step 1: API client.** Add `coachOpening(id): Promise<OrchestratorReply>` (POST `/coach/opening`) and `coachStart(id): Promise<OrchestratorReply>` (POST `/coach/start`), parsing `OrchestratorReplySchema`.

- [ ] **Step 2: Fire the opening.** In `WorkspaceContainer.tsx`, in the project-load effect: after loading studio-state and the first history page, if `!studioState.started` AND the thread is empty, call `coachOpening(pid)`, then append its `narrate` as the sole AI turn and apply its directive. Guard against double-fire (a ref keyed on projectId). For an already-`started` project, skip — go straight to the interactive area.

- [ ] **Step 3: Gate the switcher.** Derive `started = studioState?.started ?? false`. When `!started`: render pure full-width `<StudioCoachChat>` and **do not render the `<Segmented>`/switcher at all** (stronger than today's `chatOnly`). The composer is hidden pre-start (Step 4 provides 开始 instead). When `started`: render the interactive area + tabs as today.

- [ ] **Step 4: 开始 button.** In `StudioCoachChat.tsx`, when `!started` (pass the flag through context/props), replace the text composer with a primary **开始** button (`text-mk-body`, `arrow` or `spark` icon, focus-visible ring — a real button, not a 12px chip). On click: call `coachStart(pid)`, append its narrate, apply its directive (which sets `started=true`, opens `forming`) → the container re-renders with tabs. Show a pending state on the button during the call.

- [ ] **Step 5: Remove the hardcoded 提案 intro.** In `PlanBlock.tsx`, delete `INTRO_ZH` (`:39-41`), `introChat()`, and the scripted quick-reply chips (`chipsDismissed`, the chip that sends the scripted prompt at `:149`). The 提案 framing is now the live agent's job (delivered by `coachStart`'s narrate). Ensure `PlanBlock` still renders with an empty thread without the intro.

- [ ] **Step 6: Tests (vitest).** In `apps/web/test/`:
  - New project (studio-state `started:false`, empty history): assert the switcher/tabs are NOT in the DOM and an 开始 button IS. Mock `coachStart` → directive `started:true, openTool:"forming"`; click 开始; assert tabs now render.
  - Existing project (`started:true`): assert tabs render immediately and no 开始 button.
  - Assert `INTRO_ZH` never renders.
  Run vitest scoped + `tsc --noEmit`.

- [ ] **Step 7: Commit** `feat(web): explicit start gate — real AI opening + 开始 button reveals tabs`.

---

## Task 7: Frontend — subagent loading/hints + one-thread coach panel

**Files:** `apps/web/src/studio/ai/` (new `SubagentHint.tsx`); `apps/web/src/workspace/blocks/ReadingBlock.tsx`; `apps/web/src/studio/ai/StudioCoachChat.tsx`; `apps/web/src/workspace/WorkspaceContainer.tsx`; Test `apps/web/test/`.

**Interfaces — Consumes:** `OrchestratorReply.planGenerated`/`compacted` (Task 1). **Produces:** a `SubagentHint` component; find_sources renders as a hidden subagent (loading + `subagent 正在检索来源…`, result into the room, NOT a chat); the coach panel shows the single studio thread across 提案/管理/写作; plan-generation and compaction surface a transient hint line.

- [ ] **Step 1: `SubagentHint` component.** A single-line status: optional spinner + text, `text-mk-small` (12px hint — allowed here), muted color, `spark` icon. Props `{ text: string; done?: boolean }`. (12px is correct ONLY because this is a genuine hint.)

- [ ] **Step 2: find_sources = hidden subagent.** In `ReadingBlock.tsx`, find where the reading-list search (`coach(..., "find_sources")` / the source-search request) renders its exchange as chat. Replace the chat rendering with: while the request is in flight show `<SubagentHint text="subagent 正在检索来源…" />`; on completion, drop the returned sources into the room list (existing sources UI) and clear the hint. Do NOT render the find_sources conversation as a chat thread. (Entering a SPECIFIC paper keeps its existing explicit-subagent presence — do not touch that.)

- [ ] **Step 3: One-thread coach panel.** Verify the coach panel (`StudioCoachChat` / `AiPanel`) shows the `studio` thread on 提案/管理/写作 (it already shares `studioMessages`). Ensure switching among these three never triggers a surface swap or reload of a different thread. If any code path loads a non-studio surface into the main panel for these three rooms, remove it. (阅读 specific-paper and 回顾 keep their own presence — out of scope to merge.)

- [ ] **Step 4: plan/compaction transient hints.** In `sendStudioTurn` (`WorkspaceContainer.tsx:332`), after applying a reply: if `reply.planGenerated`, append a transient system line rendered via `SubagentHint text="subagent 已整理研究计划" done` into the thread (or show it inline near the plan open); if `reply.compacted`, append `SubagentHint text="已整理较早的对话" done`. Keep them subtle and non-blocking. (These are post-hoc acknowledgments — the work completed during the awaited turn; a true during-turn spinner would need per-phase SSE, out of scope.)

- [ ] **Step 5: Wire the reply flags.** Ensure the frontend `OrchestratorReply` type carries `planGenerated`/`compacted` (Task 1) and `sendStudioTurn` reads them. The backend sets `planGenerated=true` when `generate_plan` ran (in `applyOrchestratorTools`) and `compacted=true` when `maybeCompactBackstop` actually folded — wire these on the reply DTO (small addition to Task 3/4 handlers; if not already done, add here with a backend follow-up commit).

- [ ] **Step 6: Tests (vitest).** `SubagentHint` renders text + spinner; find_sources shows the hint during a mocked in-flight search and clears on resolve; a reply with `planGenerated:true` renders the 已整理研究计划 line; the coach thread is identical when toggling 提案/管理/写作. Run vitest scoped + `tsc --noEmit`.

- [ ] **Step 7: Commit** `feat(web): hidden-subagent loading hints + one-thread coach panel`.

---

## Task 8: Frontend — tab buttons redesign + full title viewable

**Files:** `apps/web/src/workspace/RoomSwitcher.tsx` (new) or a `Segmented` icon+size variant in `apps/web/src/ui/feedback.tsx`; `apps/web/src/workspace/WorkspaceContainer.tsx`; `apps/web/src/shell/home/HomePage.tsx`; `apps/web/src/workspace/Directory.tsx`; Test `apps/web/test/`.

**Interfaces — Consumes:** `BLOCK_META` + `Icon` (`apps/web/src/workspace/Icon.tsx`). **Produces:** the 5 tabs render icon + label at ≥14px with legible inactive contrast; the full project title is viewable in the top bar (tooltip + tap-to-reveal) and via `title` attr on home/directory cards.

- [ ] **Step 1: Room switcher with icons.** Build the tab switcher for the workspace using `BLOCK_META.map` → each button shows `<Icon name={b.key} size={16}/>` + `<span className="text-mk-body">{b.label}</span>`, active = white thumb (`bg-mk-surface text-mk-ink shadow-mk-xs`), inactive = `text-mk-ink/70` (legible, NOT `text-mk-muted`), `focus-visible` ring. Either a new `RoomSwitcher.tsx` or a `Segmented` variant that accepts `size="lg"` + icon nodes; do NOT silently reuse `Segmented`'s default 12px for these tabs. Replace the switcher usage in `WorkspaceContainer.tsx:732`.

- [ ] **Step 2: Full title in the top bar.** In `WorkspaceContainer.tsx:928`, wrap the `<h1 class="truncate …">` so the full title is reachable: add `title={workspace.title}` (baseline), wrap in the design-system `Tooltip` (from `feedback.tsx` §9) for hover/focus, and make it **tappable** to toggle the truncation (state `titleExpanded` → drop `truncate`, allow wrap) — a keyboard-focusable button/`<h1 role="button" tabIndex={0}>` with an accessible label. It must read as content the student can open, not a hint.

- [ ] **Step 3: Card title safety net.** In `HomePage.tsx:92` and `Directory.tsx:122`, add `title={project.title || "未命名项目"}` to the `line-clamp-2` element so a desktop hover reveals the full title.

- [ ] **Step 4: Tests (vitest).** Tabs render an icon (svg) + label per `BLOCK_META`, and the label element's class includes `text-mk-body` (assert not `text-mk-small`). Top-bar `<h1>` carries the full `title` attr; clicking it removes the `truncate` class (query by role/text and assert class toggles). Home/Directory card carries the `title` attr. Run vitest scoped + `tsc --noEmit`.

- [ ] **Step 5: Commit** `feat(web): tab buttons with icons ≥14px + full-title tooltip/tap-to-reveal`.

---

## Self-Review

**Spec coverage:**
- Change 1 (real AI opening) → Task 3 (opening prompt + endpoint) + Task 6 (wiring, remove CHAT_INTRO).
- Change 2 (start gate → tabs) → Task 2 (`started`) + Task 3 (start endpoint) + Task 6 (gate + 开始 button).
- Change 3 (recap + lazy pages) → Task 1 (contract) + Task 4 (backend) + Task 5 (frontend).
- Change 4 (tab redesign) → Task 8.
- Change 5 (full title) → Task 8.
- Subagent taxonomy + loading states + one thread → Task 7.
- Compaction loading/ack → Task 7 (post-hoc; true during-spinner noted as out of scope).

**Type consistency:** `started`, `planGenerated`, `compacted`, `CoachHistoryPage` are defined in Task 1 and consumed by Tasks 3/4/5/6/7. `applyOrchestratorTools` extracted in Task 3 is reused by `postCoach`/`postCoachStart` and sets `planGenerated`. Cursor is an opaque string end-to-end (`nextCursor` out, `before` in).

**Known limitation (documented, not a gap):** plan-generation and compaction complete server-side within the awaited coach turn, so their "loading" is a post-hoc acknowledgment line, not a during-turn spinner. A true per-phase spinner needs SSE streaming of tool intent — deliberately out of scope for this plan.

**Deploy:** `full` (migration 0059 + api + web).

---

## Task 9: Frontend — design-system conformance for scrollbars + text inputs (added 2026-08-08 per user feedback)

**Files:** the global stylesheet (`apps/web/src/index.css` or the app's Tailwind entry — grep for `@tailwind`/`:root`); shared input/textarea components (`apps/web/src/studio/ai/Composer.tsx` and any `apps/web/src/ui/*` input primitives); Test `apps/web/test/`.

**Interfaces — Produces:** every scroll container and every text input/textarea reads as part of the design system (mk tokens): custom-styled scrollbars (not the raw OS default) and consistent input chrome.

**Context:** The user reports (1) scrollbars don't follow the design and (2) many text boxes (inputs/textareas) don't follow the design. Fix both as a conformance sweep — do NOT redesign layouts.

- [ ] **Step 1: Global scrollbar styling.** In the global stylesheet add a themed scrollbar treatment applied to scrollable regions (thin, mk-token colors, rounded thumb): `::-webkit-scrollbar`, `::-webkit-scrollbar-thumb`, `::-webkit-scrollbar-track` + Firefox `scrollbar-width: thin; scrollbar-color: <thumb> <track>`. Use existing mk color tokens (e.g. a muted border/paper token for the thumb). Prefer a single reusable utility class (e.g. `.mk-scroll`) applied to scroll containers, plus a restrained global default — match how the design system is already organized. Respect `prefers-color-scheme`/theme tokens if the app themes.
- [ ] **Step 2: Audit + normalize text inputs.** Grep `apps/web/src` for `<input`, `<textarea>`, and `contentEditable` usages. Identify the design-system input treatment (border token, `rounded-mk-*`, focus-visible ring `ring-mk-accent`, `text-mk-body` ≥14px, placeholder color token). Where a shared primitive exists, route inputs through it; where inputs are ad-hoc and off-system, bring them onto the tokens. Do NOT introduce 12px body text in inputs — inputs are content, not hints (`text-mk-body` min). Focus on the high-traffic surfaces first: the coach `Composer`, the writing textarea, create-project fields, reading/notes inputs.
- [ ] **Step 3: Verify no regression.** `pnpm exec tsc --noEmit` = 0; `pnpm exec vitest run` green. Add a light test only where a shared input primitive gained a class contract worth locking (e.g. the Composer textarea carries `text-mk-body` and a focus ring class) — do not over-test global CSS.
- [ ] **Step 4: Commit** `feat(web): design-system scrollbars + text-input conformance sweep`.

**Constraints:** design-system tokens only; never `bg-mk-<token>/<opacity>` on a hex token (renders transparent); one Tailwind class per competing property; ≥14px for input/body text (12px only for genuine hints). Scope = styling conformance, not layout/behavior changes.

---

## Task 10: Backend — record the lifecycle stage on each chat turn (added 2026-08-08 per user feedback)

**Files:** `apps/api/internal/store/migrations/0060_chat_message_stage.sql` (new); `apps/api/internal/store/queries/chat.sql` (+ sqlc regen); the coach message-append helper (`apps/api/internal/agent/agentstore.go` `AppendProjectCoachMessage` + the card-message variant) and its callers in `apps/api/internal/api/coach.go` / `card_reflect.go`; Test scoped `internal/api` + `internal/store` (docker foreground).

**Interfaces — Produces:** `chat_message.stage text` (nullable); every studio coach turn (student + assistant + opening + start + card turns) is persisted with the `studio_state.stage` in effect at that moment, so the evaluation layer can read the per-turn lifecycle arc. Historical rows stay null.

**Rationale:** 过程即数据. `surface` says WHICH room/producer; `stage` says WHERE in the lifecycle (topic_discussion/proposal_forming/plan_generation/proposal_writing/proposal_review/body_writing/retrospective). Both are useful; stage enables an arc-of-thinking read in assessment. GET must not spend tokens; this is pure persistence metadata.

- [ ] **Step 1: Migration 0060.** `ALTER TABLE chat_message ADD COLUMN stage text;` (nullable — no default, historical rows null). Down: drop the column. Copy the goose header style from a recent migration.
- [ ] **Step 2: Thread `stage` through the insert.** Update the `CreateProjectCoachMessage` query in `chat.sql` (and the card-message insert if separate) to accept and store `stage` (nullable `*string`/`sqlc.narg`). Regenerate sqlc (`cd apps/api && CGO_ENABLED=0 go tool sqlc generate`). Update `AppendProjectCoachMessage` (and the card variant) in `agentstore.go` to take a `stage string` (pass "" → NULL) and forward it.
- [ ] **Step 3: Pass the stage at every persist site.** In `coach.go`, every `AppendProjectCoachMessage(..., "studio")` call (postCoach student+assistant, postCoachStart synthetic+assistant, postCoachOpening assistant, subagent turns) passes `string(state.Stage)` (opening: `string(agent.StageTopicDiscussion)` or the loaded state's stage). Card turns in `card_reflect.go` pass the current stage too. Keep the subagent surfaces' own stage as whatever `studio_state.stage` currently is (they don't mutate it). Do NOT change any behavior other than adding the stage argument.
- [ ] **Step 4: Tests (docker foreground, CGO_ENABLED=0, -timeout 600s).** A coach turn persists a `chat_message` row whose `stage` equals the project's current `studio_state.stage` (e.g. after start → `proposal_forming`); the opening turn persists `stage='topic_discussion'`. Assert via a direct `SELECT stage FROM chat_message ...` query. Run the existing `TestCoach*` suite to confirm no regression. Also `go build ./...` + `go vet`.
- [ ] **Step 5: Commit** `feat(api): record studio_state.stage on each chat turn (0060) for process evaluation`.

**Note:** this changes `AppendProjectCoachMessage`'s signature — update ALL callers (grep) so the build stays green. Never `git add -A`. Deploy remains `full` (now migrations 0059 + 0060).
