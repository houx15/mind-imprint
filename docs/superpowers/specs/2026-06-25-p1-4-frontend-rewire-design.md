# P1.4 · Frontend Rewire — Design

> **Phase spec.** The frontend (`apps/web`) cutover from a pure-SPA (localStorage +
> browser-direct LLM) to a thin renderer + API client against the live Go
> `/api/v1` backend. Authored 2026-06-25. Sub-project of the
> [backend platform architecture north-star](2026-06-24-backend-platform-architecture-design.md);
> consumes the contract in [api-design.md](../../architecture/api-design.md).
> Builds on P1.1–P1.3 (Go service, agent brain + gateway, HTTP API + inline eval +
> entitlement), all merged to `main`.

## Goal

Make the React SPA a **pure renderer + API client**: it holds no LLM keys, never
calls a model directly, and reads/writes all state through `/api/v1`. After P1.4 the
end-to-end Phoebe aorta runs against the real backend — task creation, the streaming
agent turn, the card-envelope lifecycle, the refeed, and inline evaluation — with the
same UI the SPA has today.

## Guiding principle — preserve interface shapes, swap internals

The rewire's central decision: keep the two load-bearing client interfaces and swap
**only their internals**.

- The **`Store`** stays a `useSyncExternalStore`-compatible cache with the same query
  surface (`listMessages` / `listCards` / `getLatestEvaluation` / …). Its backing
  changes from a localStorage blob to a server-hydrated in-memory cache.
- The **`Conversation`** and **`Evaluator`** controllers keep their existing surfaces
  (`send` / `openCard` / `closeCard` / `submitCard` / `skipCard` + `getSnapshot` /
  `subscribe`; `run` / state). Their internals change from a browser turn-loop +
  direct LLM calls to API calls + SSE consumption.

Because every presentational unit reads those two shapes, the churn concentrates in
three places — a new `api/` module, the store internals, and the two controllers —
while the card runtime, chat, process tree, records, and eval views stay essentially
untouched. This is also the YAGNI choice: no state-library swap (React Query/Redux),
no router introduction, no component-tree restructure.

## One backend addition — the continuation turn

Today the browser triggers the AI's post-card response (the refeed beat,
「回灌、一次只问一个」) by calling the model directly when a card is submitted, with no
new student message. The P1.3 backend's `POST /tasks/:id/turn` requires a non-empty
`user_input` and always appends a user message; `PUT /cards/:cid` only persists and
returns JSON. Nothing makes the AI respond to a *just-submitted* card unless the
student also types.

**Addition:** `POST /tasks/:id/turn` accepts an **empty / omitted `user_input`** as a
*continuation turn* — it appends **no** user message and runs the model on the
existing history, which (via `BuildLlmMessages`) now ends in the resolved card's
`tool_use` + `tool_result` pair, so the model responds to the refed card. When
`user_input` is present, behavior is unchanged (append, then run).

Concretely: `agent.RunTurn` skips `AppendUserMessage` when `userInput == ""`; the
`postTurn` handler drops the empty-input 400 and treats empty as continuation. This
is the only backend change in P1.4 (one conditional + tests). Task creation/kickoff
needs **no** special handling — the first turn carries the student's real opening
message as ordinary `user_input`.

## Component inventory (delete / gut / keep / build)

### Deleted — the browser must not hold keys or run the loop
- All of `src/llm/` — `client.ts`, `anthropicAdapter.ts`, `openaiAdapter.ts`,
  `config.ts` (raw key in localStorage), `types.ts`, `LlmError.ts`, `index.ts`,
  `live.smoke.test.ts`.
- Browser turn-loop + server-owned logic in `src/agent/` — `createConversation.ts`,
  `prompt.ts`, `messageMapping.ts`, `runEvaluation.ts`, `evalInput.ts`,
  `evalPrompt.ts`, and their tests. (`createConversation`/`createEvaluator` are
  re-implemented as API controllers — see Built.)
- `src/shell/KeyGateModal.tsx`, `src/shell/settings/LlmConfigForm.tsx`,
  `src/dev/SettingsPanel.tsx`, `src/dev/WorkspaceDev.tsx`.
- The `VITE_LLM_FORMAT` / `VITE_LLM_BASE_URL` / `VITE_LLM_MODEL` / `VITE_LLM_API_KEY`
  / `VITE_LLM_EVAL_MODEL` env vars and the `mk.llmConfig` localStorage key.

### Built — `src/api/`
- `client.ts` — a `fetch` wrapper: base URL from `VITE_API_BASE_URL`,
  `credentials: 'include'` (so the P2 session cookie will flow unchanged), JSON
  helpers, and error-envelope (`{error:{code,message}}`) → a typed `ApiError`.
- `tasks.ts` — `listTasks`, `createTask`, `getTask` (returns `{task, messages, cards}`).
- `cards.ts` — `activateCard` (`PATCH`), `submitCard` (`PUT`), `skipCard` (`POST …/skip`).
- `evaluate.ts` — `runEvaluation` (`POST …/evaluate`), `getEvaluation` (`GET …/evaluation`).
- `turn.ts` — `runTurn(taskId, userInput?)`: consumes the `text/event-stream` from
  `POST …/turn` via `fetch` + `ReadableStream` + a small SSE frame parser, yielding
  `text` / `card` / `done` / `error` events. (`EventSource` can't issue a POST, so a
  manual reader is required.) An empty/omitted `userInput` is the continuation turn.

### Gutted — shape kept, internals swapped
- `src/store/` — the cache loses localStorage; `createStore` writes become local
  optimistic updates hydrated from API responses + SSE. `storage.ts`'s `RawStorage`
  is removed (or reduced to an in-memory default for tests). `useStore` unchanged.
- `src/agent/createConversation.ts` → re-implemented as a thin controller with the
  same surface, driving `api/turn` (SSE) + `api/cards` and updating the store cache.
  `createEvaluator.ts` → calls `api/evaluate`. `useConversation` / `useEvaluator`
  unchanged. `index.ts` pruned.
- `src/shell/AppShell.tsx` — drop the `isVerified`/`KeyGateModal` gate and the
  `chat`/`config` prop threading; build the store as a server-hydrated cache.
- `src/shell/WorkspaceContainer.tsx` — replace `createConversation`/`createEvaluator`
  construction; the opening turn is an ordinary `send` of the student's first message.
- `src/shell/directory/DirectoryView.tsx` — `handleStart` → `createTask` API call,
  then navigate and send the opening message.
- `src/shell/settings/SettingsView.tsx` — remove the model/API-key card; keep
  avatar/profile/toggles.
- `src/workspace/WorkspaceView.tsx` / `CardSheetHost.tsx` — same components; the
  submit/skip callbacks now call the API controller (which fires the continuation
  turn) instead of the browser loop.

### Kept — essentially untouched
The whole card runtime (`CardRenderer`, `fieldRegistry` + `fields/*`,
`customRenderers` + `renderers/*`, `envelopeReducer`, card `states/*`, `teaching/*`),
every view-model (`processTree`, `viewModel`, `evalView`, `nodeView`, `taskCardView`,
all `records/*`: activityCalendar/growthReviews/cardUsage/ability/radarGeometry),
`ChatLog` / `Composer` / `TreePanel` / `EvalModal` / `EvalLoading`, the `?demo`
`Harness`, `StorePanel`, and `CARD_REGISTRY` / `FULL_RUBRIC` (still needed client-side
for rendering and `evalView`). `serializeCardForRefeed` / `deriveCatalog` are no longer
imported by the client (they live server-side now).

## Data flow — the new turn lifecycle

1. **Open task** — `GET /tasks/:id` hydrates the cache with `{task, messages, cards}`;
   `GET …/evaluation` (404 ⇒ none) hydrates any evaluation.
2. **Send** — push an optimistic user message into the cache → `POST …/turn
   {user_input}` → consume SSE: `text` deltas accumulate into a streaming assistant
   message; a `card` event adds a `proposed` card to the cache and opens the sheet;
   `done` finalizes the assistant message id; `error` surfaces a non-leaking message.
3. **Fill & submit a card** — the kept `CardSheetHost` collects the standard envelope
   via `envelopeReducer`; submit → `PUT …/cards/:cid` → update the cached card to
   `completed` → fire a **continuation turn** (`POST …/turn` empty) → SSE reply.
4. **Skip a card** — `POST …/cards/:cid/skip` → cache update → continuation turn.
   (Opening a card optionally `PATCH`es it `active` so the live tree reflects it; just
   closing the sheet records nothing — close ≠ skip, unchanged.)
5. **Evaluate** — `POST …/evaluate` → `201 {evaluation}` → cache → `EvalModal`.

## Error handling

`api/client` maps the backend error envelope to a typed `ApiError` (code + safe
message); the SSE `error` event is surfaced the same way. The UI shows the safe
message and never sees provider/internal detail (the backend already guarantees this).
Network/stream failures degrade to a retryable error state on the conversation
controller, mirroring today's `error` phase.

## Auth & key UI scope (P1.4)

No login: the backend runs as the seeded student via `ActAsSeed`, so the client just
sends `credentials: 'include'` and talks to the API. `AuthScreen` stays a visual
pass-through (no backend call); the LLM key gate and config UI are deleted. **Real
auth (signup / verify / signin, the session cookie) is P2** and slots in behind the
already-present `credentials: 'include'` + `AuthScreen` shell with no further handler
changes.

## Known limitation (accepted for P1.4)

`RecordsView` aggregates across *all* of the student's tasks (activity calendar,
growth reviews, ability radar), but the API has no cross-task aggregate endpoint —
only `GET /tasks` (list) and `GET /tasks/:id` (one task's detail). P1.4 fetches each
task's detail to populate the records cache (an N-fetch), which is fine for the single
seeded student / small N. A dedicated summary endpoint is a future optimization
(post-P1), explicitly out of scope here.

## Testing

- **Delete** the browser-LLM / agent-loop / eval tests under `src/llm/` and
  `src/agent/` (that logic now lives in Go and is tested there) plus the
  `KeyGateModal` / `LlmConfigForm` / `SettingsPanel` / `WorkspaceDev` tests.
- **Keep** every card-rendering, `envelopeReducer`, and view-model test
  (`processTree` / `viewModel` / `evalView` / records sub-modules / `taskCardView`).
- **Add**: `api/client` + SSE-frame-parser tests (mock `fetch` / a fake
  `ReadableStream`), conversation + evaluator controller tests (mock the api client),
  store-as-cache tests, and one wired `WorkspaceView` flow test driven by a fake SSE:
  send → `card` event → submit → continuation turn → reply → evaluate.
- **Backend**: a Go test for the continuation turn (`RunTurn` with empty `user_input`
  appends no user message and still produces an assistant reply from history; the
  handler no longer 400s on empty input).

## Out of scope (later phases)

Real auth / sessions (P2); rich org screens — teacher/admin (P3); async evaluation via
river (P4); a cross-task records aggregate endpoint; object storage / multimodal /
more cards. P1.4 is purely the client cutover plus the single continuation-turn
backend addition.
