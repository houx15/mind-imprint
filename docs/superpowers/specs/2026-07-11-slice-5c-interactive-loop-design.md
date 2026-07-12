# Slice 5c — Interactive Conversational Loop (chat-aware coach) · Design

> Third Studio slice; makes the coach rail two-way live. Part of the whole-product
> refactor #2 (`docs/2026-07-11-whole-product-refactor-roadmap.md`,
> `docs/2026-07-11-agent-spec.md`). Follows Slice 5b (read path,
> `…-slice-5b-studio-readpath-design.md`).

## Goal

A student types in the Studio composer → the message persists to the project's
chat thread → the coach, now seeing the student's message + recent thread, runs
`RunAgentStep` and streams back **one** anchored intervention (or a gate check, or
silence) over SSE, live into the coach rail → the student can 三键处置
(accept/rewrite/reject) a coach **proposal**. **Conversational loop only** — no
tool-cards, no `card_instances`, no schema migration.

## Scope

**In:**
1. **Chat persistence** — author `queries/chat.sql` (none exists): `GetOrCreateThread`, `CreateChatMessage`, `ListChatMessagesByThread`; sqlc regen; `AgentStore` seam `LoadChatHistory` + `CreateChatMessage` + adapter.
2. **Chat-aware coach** — `ProposeIntervention` + `BuildCoachContext` take the recent thread; `RunAgentStep` loads it and threads it through; anchor/criterion stay classifier-driven; the enforcement stack is unchanged.
3. **Card-surfacing gated off** — additive `AgentDeps.SkipSurfaceCards bool` (zero value = current behavior; 5c sets `true`) so `RunAgentStep` skips `SurfaceCardCandidates` and yields only `post_intervention`/`check_gate`.
4. **Loop driver + SSE endpoint** — `POST /api/v1/projects/{id}/turn` (SSE), mirroring `postTurn`: ownership + entitlement gate before stream, persist the student message, run **one** `RunAgentStep`, stream the resulting action, `done`.
5. **New Studio SSE event contract** — `intervention` / `gate` / `done` / `error` + heartbeat, distinct from the legacy `text`/`card`/`done`.
6. **Disposition endpoint** — `POST /api/v1/projects/{id}/interventions/{iid}/disposition` → `RecordDisposition` (ownership + the existing ≥15-rune guard).
7. **Projection extension** — `studio.projectCoach` merges the student's `chat_message`s + interventions (the 5b "no student bubbles" deferral resolves here).
8. **Frontend** — `api/studioTurn.ts` SSE client + a studio conversation controller + wiring `StudioContainer`'s `onComposerSend`/`onDisposition` live, with live coach-rail updates.

**Out (later):**
- **`surface_card` / tool-card fill / refeed** + `card_instances` → **5c-2**.
- **Slice-3 debt** (`task_id` NULLABLE + project-scoped `GetCardInstance`) → a cleanup slice — NOT needed here (no `card_instances` minted).
- **Onboarding live producer** (`rubric_translation`/`milestone_plan` nodes) → its own slice.
- **Token-streaming** the coach body (coach is single-shot `gateway.Collect`); the **multi-step / debounced** loop (meaningful once card-fill mutates the graph) → **5c-2**.
- **Retiring legacy `RunTurn`/`turn.go` / the old chat `workspace/`** + the app-routing flip → **5d**.

## Global Constraints (verbatim; bind every task)

- **Client never calls the model directly.** The SSE turn goes through the Go gateway; keys server-side only.
- **AI restraint / 一次只问一个** (AGENTS.md red lines): **one** coach intervention per student turn (one `RunAgentStep`); the coach may stay **silent** (no candidate → `(nil,nil)` → no `intervention` event). No slot-machine mechanics.
- **AI never states conclusions for the student.** The chat-aware coach's body still passes the full enforcement stack unchanged: `ValidateOutput` (typed-output), `BannedPhrasing` (reject), `OutputCheck` (declarative-echo → rewritten as a question), authorship. Anchor + criterion come from the classifier candidate, never parsed from the model reply.
- **Single source of truth.** Reuse `RunAgentStep`/`ProposeIntervention`/enforcement — do not fork a parallel coach. The SSE endpoint is a thin driver.
- **Additive; legacy untouched.** No schema migration, no change to `card_instances`/`task_id`, and the legacy `postTurn`/`RunTurn`/`turn.go` path stays live (retired in 5d). `AgentDeps.SkipSurfaceCards` zero value preserves existing `RunAgentStep` behavior for all current callers/tests.
- **Studio discipline.** The new endpoint + frontend controller are additive; do not touch `AppShell`/`Root`/old `workspace/`.
- **Icons = inline SVG, never `lucide-react`. Binding Chinese design copy verbatim** — fix the test, never the copy. **Docs English** except literal Chinese UI copy.

## Architecture

```
CoachRail composer send → StudioContainer.onComposerSend(text)
  → api.studioTurn(projectID, text)  [SSE async generator]
     POST /api/v1/projects/{id}/turn
       loadOwnedProject (404-not-403) · HasEntitlement (JSON error before stream)
       NewSSEWriter + syncEmitter(+heartbeat 15s)
       persist student: GetOrCreateThread(project) → CreateChatMessage(user,text)
                        → AppendEvent(surface:"studio", type:"prompt_sent")
       action, _ := RunAgentStep(deps{ SkipSurfaceCards:true, chat-aware }, projectID, Trigger{"student_turn"})
         · intervention → (InsertIntervention already inside) → emit `intervention`{id,body,anchor,criterion,level}
         · check_gate   → emit `gate`{contract,status,passed,total,missing}
         · silence(nil) → emit nothing
       emit `done`
  → controller appends the student bubble + the coach reply to coach.messages (live)
  → CoachRail re-renders

three-key 处置 → StudioContainer.onDisposition(choice, reason)
  → POST /api/v1/projects/{id}/interventions/{iid}/disposition {action:choice, reason}
     loadOwnedProject · RecordDisposition (≥15 runes) → InsertDisposition
```

### The coach's turn is the intervention row (no duplication)

The coach's conversational reply **is** the `intervention` row (it carries the
anchor/criterion/level the rail needs for the chip + `锚定` label + disposition).
5c does **not** also persist an assistant `chat_message` for it — that would
double the reply. So:

- **student turn** → `chat_message(role="user")`.
- **coach turn** → `intervention` row (via `RunAgentStep`'s existing `InsertIntervention`).
- **the coach's context AND the projection thread** = student `chat_message`s ⋈ interventions, merged by `created_at`.

Every coach reply is graph-**anchored** (post_intervention anchors to a node); the
runtime has no anchor-less "chit-chat" verb, and that is correct — the coach stays
on-task or stays silent.

## 1. Chat persistence

`apps/api/internal/store/queries/chat.sql` (new). The tables exist (migration 0016):
`chat_thread(id, user_id NOT NULL, title, seeded_project_id → project, created_at)`,
`chat_message(id, thread_id NOT NULL → chat_thread CASCADE, role CHECK(user|assistant|system),
content, modality CHECK(text|voice|file|image) DEFAULT 'text', attachments, quoted_fragment, created_at)`.

- `GetOrCreateThread` — one thread per project: `SELECT` by `seeded_project_id`; if none, `INSERT` (`user_id` = the project's owner, `seeded_project_id` = the project). (A `:one` upsert, or a get-then-insert helper in the adapter.)
- `CreateChatMessage(thread_id, role, content, modality)` `:one`.
- `ListChatMessagesByThread(thread_id)` `:many` ordered `created_at, id`.

`make sqlc`. Add to the `AgentStore` seam (`loop.go`) + `sqlcAgentStore` adapter:
- `CreateChatMessage(ctx, projectID uuid.UUID, role, content string) error` (resolves/creates the thread internally).
- `LoadChatHistory(ctx, projectID uuid.UUID, limit int) ([]ChatTurn, error)` where `ChatTurn{Role string; Content string}` — the **merged** student `chat_message`s (role "user") + interventions (role "assistant", body as content), time-ordered, capped to the most-recent `limit` (≈12).

## 2. Chat-aware coach

- `type ChatTurn struct{ Role, Content string }` (agent package).
- `ProposeIntervention(ctx, prov, r, g GraphView, c Candidate, history []ChatTurn, sim) (AgentOutput, string, error)` — new `history` param.
- `BuildCoachContext(g GraphView, c Candidate, history []ChatTurn) string` — the user turn now includes a compact rendering of the recent thread (student + prior coach turns) **above** the existing graph-neighborhood + candidate framing, so the model's body responds to what the student said. System stays `coachPosturePrompt`; still `gateway.Collect` (single-shot); enforcement unchanged; anchor/criterion still from `c`.
- `RunAgentStep`: after `LoadGraph`, also `deps.Store.LoadChatHistory(ctx, projectID, 12)`; pass it to `ProposeIntervention`. (History load failure → treat as empty history, never fail the turn.)
- Existing `ProposeIntervention` callers/tests updated to pass `nil` history (behavior identical when history is empty).

## 3. Card-surfacing gated off

`AgentDeps.SkipSurfaceCards bool` (new). In `RunAgentStep`'s candidate assembly, skip `SurfaceCardCandidates(g)` when `deps.SkipSurfaceCards`. Zero value `false` = current behavior (surface cards produced) → every existing caller/test is unaffected. 5c's driver sets `true`. (5c-2 sets `false` and wires the card path.)

## 4. Loop driver + SSE endpoint

`apps/api/internal/api/studioturn.go` — `postProjectTurn`, registered `POST /api/v1/projects/{id}/turn` under `RequireUser`. Mirrors `postTurn` (`turn.go`):
- `loadOwnedProject(w,r)` (reuse the 5b helper) — 404 if not owned.
- `HasEntitlement(ctx,user)` → JSON `ErrNotEntitled()` **before** committing to the stream.
- decode `{ "user_input": string }` (non-empty required for 5c — empty-continuation is a card-refeed concern, 5c-2).
- `NewSSEWriter(w)` → wrap in a `syncEmitter`-style mutex wrapper implementing the **Studio** emitter (below) + heartbeat; 15s heartbeat goroutine with the `stop`/`ctx.Done()`/`defer close+wait` shape from `turn.go`.
- persist student: `Store.CreateChatMessage(projectID,"user",input)` + `AppendEvent(surface:"studio", type:"prompt_sent", payload:{len})`.
- `action, err := agent.RunAgentStep(ctx, deps, projectID, agent.Trigger{Kind:"student_turn"})` — `deps` = `AgentDeps{Store, Provider, Resolved: <chat key>, Sim, Skill:&writingSkill, SkipSurfaceCards:true}`.
- on `err`: `slog.Error` + emit `error` envelope. on `action==nil`: emit nothing (silence). on `intervention`: emit `intervention`. on `check_gate`: emit `gate`.
- emit `done`.

**One `RunAgentStep` per turn** (one action). No multi-step loop, no gate debounce in 5c (the graph is not mutated by a text turn; those land in 5c-2).

## 5. Studio SSE event contract

Add Studio emit methods to the gateway SSE writer (or a small `StudioSSEEmitter` wrapping `*SSEWriter`), and a shared TS type so the client parses safely. Events (`event:` name → `data:` JSON):
- `intervention` → `{ intervention_id, body, anchor, criterion, level }` (anchor is the decoded label string; criterion e.g. `"D5"`; level e.g. `"I2"`).
- `gate` → `{ contract, status, passed, total, missing }`.
- `done` → `{}`.
- `error` → `{ error: { code, message } }`.
- heartbeat → `: ping` comment.

The wire source of truth is the Go emitter. The frontend union is a **local**
`StudioTurnEvent` type in `api/studioTurn.ts` — mirroring how the legacy
`api/turn.ts` keeps its `TurnEvent` union local (NOT in `packages/contracts`).
No contracts-package addition; the client maps each parsed SSE frame to the union.

## 6. Disposition endpoint

`postInterventionDisposition`, `POST /api/v1/projects/{id}/interventions/{iid}/disposition`, `RequireUser`:
- `loadOwnedProject` (ownership) + parse `iid`.
- decode `{ action: "accept"|"reject"|"rewrite", reason }`.
- `agent.RecordDisposition(ctx, deps, iid, action, reason)` — the existing ≥15-**rune** guard rejects short reasons (400). (Optionally verify the intervention belongs to the project; low-risk since ownership is checked.)
- 204 on success.

## 7. Projection extension (`studio.projectCoach`)

Extend the 5b `projectCoach` to take the merged thread: student `chat_message`s → `{kind:"student", body}`, interventions → the existing flag/ai mapping, ordered by `created_at`. `studio.Load` also loads `ListChatMessagesByThread`. On reload the coach rail shows both sides. (5b produced interventions-only; this is purely additive to the projection.)

## 8. Frontend

- **`apps/web/src/api/studioTurn.ts`** — `studioTurn(projectId, userInput): AsyncGenerator<StudioTurnEvent>` mirroring `api/turn.ts`: POST, `Accept: text/event-stream`, `for await (parseSSE(res.body))` → map `intervention`/`gate`/`done`/`error` → `StudioTurnEvent`; non-OK → parse JSON error → `{type:"error"}`. Add `postDisposition(projectId, interventionId, action, reason)` (plain `apiFetch`).
- **A studio conversation controller** (mirrors `agent/createConversation.ts`, lean): holds the coach thread + a `sending` flag + the current disposable intervention id; `send(text)` appends a `{kind:"student"}` bubble, streams, appends the coach `{kind:"ai", …, anchor}` reply on `intervention` (tracks its `intervention_id`), sets `done`/`error`. Exposed to `StudioContainer` via `useSyncExternalStore` (the app's pattern).
- **`StudioContainer.tsx`** — replace the inert `onComposerSend`/`onDisposition` no-ops: `onComposerSend` → controller `send`; `onDisposition(choice,reason)` → `postDisposition(projectId, currentInterventionId, choice, reason)`. Merge the controller's live thread into the rendered `coach.messages` (seed from the projection, then append live turns). Keep `onSelectStation`/`onToggleFocus`/`onOpenMethodology` as in 5b.
- `CoachRail`/composer/`DispositionCard` are already wired to the callbacks (5a) — no component change beyond passing the live thread; the composer disables while `sending`.

## Testing

- **Go — chat store** (`internal/store`): `chat.sql` round-trip (GetOrCreateThread idempotent per project, CreateChatMessage, ListChatMessagesByThread order) vs testcontainers.
- **Go — chat-aware coach** (`internal/agent`): `BuildCoachContext` includes the thread turns; `ProposeIntervention` with a stub provider echoes/uses history (assert the request messages carry the student turn); enforcement still runs (banned-phrasing/echo still rejected/rewritten). `SkipSurfaceCards` skips `SurfaceCardCandidates` (a graph that WOULD surface a card yields post_intervention/silence instead when the flag is set; and yields the card when unset — proving zero-value back-compat).
- **Go — loop driver / endpoint** (`internal/api`, testcontainers): `POST /projects/{id}/turn` with the seeded project — 401 unauth, 404 non-owned, and a happy path asserting the SSE stream carries an `intervention` (or `gate`/`done`) event and that a `chat_message(user)` + a `prompt_sent` event were persisted. Disposition endpoint: 204 happy, 400 on <15-rune reason, 404 non-owned.
- **Go — projection**: `projectCoach` merges student chat_messages + interventions in time order (a unit test over hand-built rows).
- **Frontend (vitest)**: `studioTurn` parses an `intervention`/`done` SSE sequence (mocked `fetch` streaming body) and yields the union; `postDisposition` posts the right shape; the conversation controller appends student + coach turns and tracks the disposable id; `StudioContainer` sends via the controller and renders the live coach turn (mocked controller/api). Error-path: an `error` frame surfaces a rail error state.
- **Gate:** full Go suite (testcontainers) green; full web suite + `tsc` green; legacy `postTurn`/`RunTurn` untouched and its tests still green; boundary held.

## Acceptance (live-verify walk)

Run the stack on a fresh migrated DB. `/?studio` (Phoebe demo project, S4). Type a
reply into the coach composer (e.g. “它想证明中国在认真转型”). Expect: the student
bubble appears; within a couple seconds the coach replies **once**, anchored to the
orphan-evidence / 治理决心 thread (chip + `锚定` label), genuinely responding to the
text — or stays silent if nothing to nudge. The reply persists (reload shows the
student bubble + coach turn merged in order). Disposing the coach's proposal with a
≥15-character reason succeeds (short reason rejected). The composer disables while
the coach is thinking. The old chat workspace is untouched.

## Deferred / carry-forward → 5c-2, 5d, cleanup

- **5c-2:** `surface_card` (set `SkipSurfaceCards:false` + wire the card sheet), tool-card fill → graph mutation → **the multi-step / gate-debounced loop** + live `gate` re-checks; refeed. Card-fill mints `card_instances`.
- **Slice-3 debt cleanup:** `task_id` NULLABLE (regenerates `Material/CardInstance.TaskID` → `pgtype.UUID`, ripples ~8 call sites incl. `agent/eval.go`, `agent/turn.go`, `api/dto.go`) + project-scoped `GetCardInstance`.
- **Onboarding producer:** a live decode flow minting reader-shaped `rubric_translation`/`milestone_plan` nodes (else `projectOnboarding` stays seed-only).
- **5d:** retire `RunTurn`/`turn.go`/old `workspace/`; the routing flip; gate `StudioContainer.defaultEnsureSession` (currently signs in as Phoebe on any `getMe` failure).
