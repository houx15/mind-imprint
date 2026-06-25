# P1.4 · Frontend Rewire Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. Each task ends green (`pnpm -r typecheck && pnpm -r test`; backend tasks add `go vet`/`go test`).

**Goal:** Cut the React SPA (`apps/web`) over from localStorage + browser-direct LLM to a thin renderer + API client against the live Go `/api/v1` backend — task creation, the streaming agent turn, the card-envelope lifecycle, the refeed, and inline evaluation all flow through the server, and the browser holds no LLM key.

**Architecture:** Preserve two client interface shapes — the `Store` (a `useSyncExternalStore` cache) and the `Conversation`/`Evaluator` controllers — and swap **only their internals** from localStorage + browser turn-loop to a new `src/api/` module + SSE. The card runtime, chat, process tree, records, and eval views stay essentially untouched. One small backend addition (the *continuation turn*) lets a submitted/skipped card trigger the AI's refeed reply with no typed message.

**Tech Stack:** React 18 + Vite + TypeScript + Zod (frontend, vitest); Go 1.26 (one backend change, `go test` + testcontainers). SSE consumed via `fetch` + `ReadableStream` (not `EventSource` — it can't POST). DI: controllers take an `ApiClient` so tests inject fakes.

## Global Constraints

- **Client never holds an LLM key and never calls a model directly.** All model work goes through `/api/v1`; the browser only sends `credentials: 'include'`. Deleting `src/llm/` is mandatory, not optional.
- **Preserve the `Store` query surface and the `Conversation`/`Evaluator` interfaces** so the presentational layer (cards, chat, tree, records, eval) stays untouched. Swap internals only.
- **The turn is SSE over `POST /api/v1/tasks/:id/turn`** with body `{user_input}`. Events: `text {delta}`, `card {card_instance_id, card_id, nudge_text}`, `done {message_id}`, `error {error:{code,message}}`. Gate failures (entitlement 403, ownership 404) come back as a JSON error envelope (non-SSE) — handle both.
- **Continuation turn:** `POST …/turn` with empty/omitted `user_input` runs a turn that appends **no** user message and replies from existing history (which ends in the resolved card's tool_result). Card submit/skip stays a clean JSON call, then the frontend fires a continuation turn.
- **No login in P1.4.** Backend runs as the seeded student via `ActAsSeed`; `AuthScreen` is a visual pass-through; real auth is P2. Keep `credentials: 'include'` everywhere so P2 cookies flow unchanged.
- **Contract types are the wire types.** `@mind-imprint/contracts` `Task`/`Message`/`CardInstance`/`Evaluation` match the backend DTOs; Zod strips extra DTO fields (e.g. evaluation `id`/`model`/`status`). The backend omits `tool_call` when absent — normalize absent `tool_call` to `null` on hydration.
- **Known limitation (accepted):** `RecordsView` aggregates across all the student's tasks but there's no aggregate endpoint — it works off whatever tasks are hydrated into the cache (list + opened details). A summary endpoint is post-P1.
- New env: `VITE_API_BASE_URL` (origin; client prefixes `/api/v1`). Remove all `VITE_LLM_*`.
- **Branch:** the executor works on `feat/p1-4-frontend-rewire` (already created, spec committed). No repo-root `package.json` staging surprises — but note `package.json` does carry a pre-existing unrelated change; never include it in a commit. Commit trailer on every commit: `Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>`.
- The repo hook BLOCKS shell redirects outside the repo (`>/dev/null`, `2>/tmp/...`) — use `$(cmd 2>&1)`. The repo hook BLOCKS the bare 4-letter shell exec builtin (e-v-a-l) as a standalone word in bash (paths/words containing it like "evaluate"/"evaluation" are fine).
- Backend testcontainers env (run-time only, never commit): `DOCKER_HOST=unix:///var/run/docker.sock`, and if Ryuk fails `TESTCONTAINERS_RYUK_DISABLED=true`.

---

## File Structure

| File | Responsibility |
|---|---|
| `apps/api/internal/agent/turn.go` *(modify)* | `RunTurn` skips `AppendUserMessage` when `userInput == ""` (continuation turn). |
| `apps/api/internal/api/turn.go` *(modify)* | `postTurn` drops the empty-`user_input` 400; empty = continuation. |
| `apps/api/internal/agent/turn_test.go` *(modify)* | Add continuation-turn test. |
| `apps/api/internal/api/turn_test.go` *(modify)* | Replace `TestTurnEmptyInputReturns400` with a continuation-accept test. |
| `apps/web/src/api/client.ts` *(new)* | `API_BASE`, `ApiError`, `apiFetch<T>` (credentials, error-envelope → `ApiError`). |
| `apps/web/src/api/sse.ts` *(new)* | `parseSSE(stream)` → async generator of `{event, data}` frames. |
| `apps/web/src/api/turn.ts` *(new)* | `runTurn(taskId, userInput?)` → async generator of `TurnEvent`. |
| `apps/web/src/api/tasks.ts` *(new)* | `listTasks`, `createTask`, `getTask` (+ `TaskDetail`). |
| `apps/web/src/api/cards.ts` *(new)* | `activateCard`, `submitCard`, `skipCard`. |
| `apps/web/src/api/evaluate.ts` *(new)* | `runEvaluation`, `getEvaluation`. |
| `apps/web/src/api/index.ts` *(new)* | `ApiClient` interface + the default `api` object + re-exports. |
| `apps/web/src/api/*.test.ts` *(new)* | client/sse/turn/tasks/cards/evaluate unit tests (mock `fetch`). |
| `apps/web/src/store/storage.ts` *(modify)* | Default storage becomes in-memory; localStorage no longer required. |
| `apps/web/src/store/createStore.ts` *(modify)* | Default in-memory; add `putTask`, `putMessage`, `hydrateTask`. |
| `apps/web/src/store/*.test.ts` *(modify)* | Drop localStorage-persistence assertions; add upsert/hydrate tests. |
| `apps/web/src/agent/createConversation.ts` *(rewrite)* | API/SSE internals; same `Conversation` interface minus `kickoff`. |
| `apps/web/src/agent/createEvaluator.ts` *(rewrite)* | Calls `api.runEvaluation`; same `Evaluator` interface. |
| `apps/web/src/agent/createConversation.test.ts` / `createEvaluator.test.ts` *(rewrite)* | New controller tests with a fake `ApiClient`. |
| `apps/web/src/agent/index.ts` *(modify)* | Prune to the surviving controllers + hooks. |
| `apps/web/src/shell/WorkspaceContainer.tsx` *(rewrite)* | Build api-backed controllers; hydrate on mount; send opening message. |
| `apps/web/src/dev/WorkspaceDev.tsx` *(delete)* | Constructed the old controller. |
| `apps/web/src/shell/AppShell.tsx` *(modify)* | In-memory store; drop key gate/LLM props; thread `openingMessage`. |
| `apps/web/src/shell/directory/DirectoryView.tsx` *(modify)* | `api.createTask`; list from server; `onOpenTask(id, openingText)`. |
| `apps/web/src/shell/settings/SettingsView.tsx` *(modify)* | Remove the model/API-key card + `chat` prop. |
| `apps/web/src/shell/auth/AuthScreen.tsx` *(modify if needed)* | Pass-through (no LLM); usually unchanged. |
| `apps/web/src/llm/**` *(delete)* | All adapters/config/types/tests — browser never calls models. |
| `apps/web/src/agent/{prompt,messageMapping,runEvaluation,evalInput,evalPrompt}.ts` + tests *(delete)* | Server owns prompt/refeed/eval. |
| `apps/web/src/shell/KeyGateModal.tsx`, `src/shell/settings/LlmConfigForm.tsx`, `src/dev/SettingsPanel.tsx` + tests *(delete)* | LLM-key UI. |
| `apps/web/.env.example` *(new/modify)*, `apps/web/vite.config.ts` *(modify, optional proxy)* | `VITE_API_BASE_URL`. |

---

## Shared reference — exact current signatures (read before any task)

```ts
// @mind-imprint/contracts (wire types; match backend DTOs)
type Task = { id: string; title: string; seed: string | null; status: "active"|"evaluated"; created_at: string; last_active_at: string };
type MessageRole = "user"|"assistant"|"system";
type Message = { id: string; task_id: string; role: MessageRole; content: string; tool_call: unknown|null; created_at: string };
type TraceEvent = { kind: "field_change"; path: string; at: string } | { kind:"step_expand"; step_key:string; at:string } | { kind:"note_open"; step_key:string; at:string } | { kind:"skip"; at:string } | { kind:"submit"; at:string };
type CardStatus = "proposed"|"active"|"completed"|"skipped";
type CardInstance = { id:string; card_id:string; task_id:string; parent_node_id:string|null; status:CardStatus; field_values:Record<string,unknown>; event_trace:TraceEvent[]; rubric_tags:string[]; created_at:string; completed_at:string|null };
type Evaluation = { task_id:string; scores:{dim_id:string;level:string;note:string}[]; narrative:string; created_at:string };
const SummonCardArgs: ZodType<{ card_id:string; reason:string; nudge_text:string }>;
const CARD_REGISTRY: Record<string, CardSpec>;

// src/store/createStore.ts  (interface PRESERVED + additions in Task 4)
interface Store {
  getSnapshot(): StoreState; subscribe(l:()=>void):()=>void;
  createTask(i:{title:string;seed:string|null}): Task;
  getTask(id:string): Task|undefined; listTasks(): Task[];
  updateTask(id:string, patch:Partial<Pick<Task,"title"|"seed"|"status">>): Task;
  appendMessage(i:{task_id:string;role:MessageRole;content:string;tool_call?:unknown}): Message;
  listMessages(task_id:string): Message[];
  putCard(c:CardInstance): void; getCard(id:string): CardInstance|undefined; listCards(task_id:string): CardInstance[];
  putEvaluation(e:Evaluation): void; listEvaluations(task_id:string): Evaluation[]; getLatestEvaluation(task_id:string): Evaluation|undefined;
}

// src/agent/createConversation.ts  (interface PRESERVED, minus kickoff)
type ConvPhase = "idle"|"awaiting_llm"|"proposal_pending"|"card_active"|"error";
interface ConvState { taskId:string; phase:ConvPhase; pendingCardId?:string; error?:string }
interface Conversation { getSnapshot():ConvState; subscribe(l:()=>void):()=>void; send(t:string):Promise<void>; openCard(id:string):void; closeCard(id:string):void; submitCard(id:string, final:CardInstance):Promise<void>; skipCard(id:string):Promise<void> }

// src/agent/createEvaluator.ts  (interface PRESERVED)
type EvalPhase = "idle"|"running"|"done"|"error";
interface EvalState { phase:EvalPhase; evaluation?:Evaluation; error?:string }
interface Evaluator { getSnapshot():EvalState; subscribe(l:()=>void):()=>void; run():Promise<void> }

// src/cards/envelopeReducer.ts (UNCHANGED — used by the controller)
function newEnvelope(card_id:string, task_id:string, now?:()=>string, genId?:()=>string): CardInstance;
function envelopeReducer(env:CardInstance, action:{type:"activate"}|{type:"skip"}|..., now?:()=>string): CardInstance;
```

The backend SSE frames (from `apps/api/internal/gateway/sse.go`):
`event: text\ndata: {"delta":"…"}` · `event: card\ndata: {"card_instance_id":"…","card_id":"…","nudge_text":"…"}` · `event: done\ndata: {"message_id":"…"}` · `event: error\ndata: {"error":{"code":"…","message":"…"}}` · heartbeat comment lines start with `:`.

---

## Task 1: Backend — the continuation turn

**Files:**
- Modify: `apps/api/internal/agent/turn.go`
- Modify: `apps/api/internal/api/turn.go`
- Test: `apps/api/internal/agent/turn_test.go`, `apps/api/internal/api/turn_test.go`

**Interfaces:**
- Produces: `POST /api/v1/tasks/:id/turn` accepts an empty/omitted `user_input` as a continuation turn (no user message appended; model replies from existing history). Non-empty `user_input` behavior unchanged.

- [ ] **Step 1: Update the failing test in `agent/turn_test.go`.** Read the existing test file for its fake `TurnStore` + stub provider wiring. Add a test that a continuation turn appends NO user message yet still produces an assistant reply:

```go
func TestRunTurnContinuationAppendsNoUserMessage(t *testing.T) {
	// fake store records AppendUserMessage calls; seed history ending in a resolved card.
	store := newFakeTurnStore(t) // reuse the existing fake; if it counts user appends, assert 0
	prov := gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: "基于你刚填的卡，我们继续。"},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 5, OutputTokens: 7}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
	rec := &recordingEmitter{} // existing test SSE recorder
	err := RunTurn(context.Background(), TurnDeps{
		Store: store, Provider: prov,
		KeyResolver: func(context.Context) (gateway.Resolved, error) {
			return gateway.Resolved{Provider: "deepseek", Model: "deepseek-chat", Tier: "chaperone"}, nil
		},
		Catalog: nil, SpecByID: func(string) (cards.Spec, bool) { return cards.Spec{}, false },
		SSE: rec,
	}, someTaskID, "") // empty userInput
	if err != nil { t.Fatal(err) }
	if store.userAppends != 0 { t.Fatalf("continuation turn must not append a user message, got %d", store.userAppends) }
	// assistant reply + done still emitted
}
```

> Adapt names to the existing fake store / emitter in `turn_test.go`. The essential assertions: `AppendUserMessage` is NOT called when `userInput == ""`, and an assistant message + `done` are still produced.

- [ ] **Step 2: Run it, watch it fail**

Run: `cd apps/api && go test ./internal/agent/ -run TestRunTurnContinuation -v`
Expected: FAIL — `RunTurn` currently always appends the user message.

- [ ] **Step 3: Implement in `agent/turn.go`.** Guard the append:

```go
func RunTurn(ctx context.Context, deps TurnDeps, taskID uuid.UUID, userInput string) error {
	// A continuation turn (empty userInput) appends no user message and replies
	// from existing history — used after a card is submitted/skipped so the model
	// responds to the refed card (the tool_result already sits in the transcript).
	if userInput != "" {
		if _, err := deps.Store.AppendUserMessage(ctx, taskID, userInput); err != nil {
			return err
		}
	}

	history, err := deps.Store.ListMessages(ctx, taskID)
	// … rest unchanged …
```

- [ ] **Step 4: Run it, watch it pass**

Run: `cd apps/api && go test ./internal/agent/ -run TestRunTurnContinuation -v`
Expected: PASS

- [ ] **Step 5: Update the handler in `api/turn.go`.** Remove the empty-input 400 so empty = continuation. Delete this block from `postTurn`:

```go
	if strings.TrimSpace(body.UserInput) == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "user_input 不能为空", nil))
		return
	}
```

Keep decoding the body; just pass `body.UserInput` (possibly "") into `RunTurn`. If `strings` is now unused in the file, drop the import.

- [ ] **Step 6: Replace the stale handler test.** In `api/turn_test.go`, the P1.3 test `TestTurnEmptyInputReturns400` now asserts the OLD behavior and will FAIL. Replace it with a continuation-accept test:

```go
func TestTurnEmptyInputIsContinuation(t *testing.T) {
	q := newAPITestQueries(t)
	ctx := context.Background()
	task, _ := q.CreateTask(ctx, sqlc.CreateTaskParams{UserID: SeedUserID, Title: "T"})
	// seed a prior user message so history isn't empty (optional but realistic)
	_, _ = q.AppendMessage(ctx, sqlc.AppendMessageParams{TaskID: task.ID, Role: "user", Content: "hi"})

	prov := gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: "继续"},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
	catalog, _ := cards.Catalog()
	idx := map[string]cards.Spec{}
	for _, s := range catalog { idx[s.ID] = s }
	h := New(Deps{
		Queries: q, Provider: prov,
		ChatResolver: func(context.Context) (gateway.Resolved, error) {
			return gateway.Resolved{Provider: "deepseek", Model: "deepseek-chat", Tier: "chaperone"}, nil
		},
		Catalog: catalog, SpecByID: func(id string) (cards.Spec, bool) { s, ok := idx[id]; return s, ok },
	}).Handler()

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("POST", "/api/v1/tasks/"+task.ID.String()+"/turn", strings.NewReader(`{"user_input":""}`)))
	if rr.Code != 200 { t.Fatalf("continuation: want 200, got %d %s", rr.Code, rr.Body.String()) }
	if !strings.Contains(rr.Body.String(), "event: done") { t.Fatalf("want done event, got %s", rr.Body.String()) }
}
```

- [ ] **Step 7: Run the backend gate**

Run: `cd apps/api && go vet ./... && DOCKER_HOST=unix:///var/run/docker.sock go test -p 1 ./internal/agent/ ./internal/api/ -v`
Expected: PASS (continuation tests green; the old 400 test is gone).

- [ ] **Step 8: Commit**

```bash
git add apps/api/internal/agent/turn.go apps/api/internal/agent/turn_test.go apps/api/internal/api/turn.go apps/api/internal/api/turn_test.go
git commit -m "feat(api): continuation turn — empty user_input replies from history"
```

---

## Task 2: Web — API client foundation + SSE parser

**Files:**
- Create: `apps/web/src/api/client.ts`, `apps/web/src/api/sse.ts`
- Test: `apps/web/src/api/client.test.ts`, `apps/web/src/api/sse.test.ts`

**Interfaces:**
- Produces:
  - `API_BASE: string`, `class ApiError extends Error { code: string; status: number }`, `apiFetch<T>(path: string, init?: RequestInit): Promise<T>`
  - `parseSSE(stream: ReadableStream<Uint8Array>): AsyncGenerator<{ event: string; data: string }>`

- [ ] **Step 1: Write failing tests.** `apps/web/src/api/client.test.ts`:

```ts
import { describe, it, expect, vi } from "vitest";
import { apiFetch, ApiError } from "./client";

describe("apiFetch", () => {
  it("returns parsed JSON on 200", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify({ ok: 1 }), { status: 200 })));
    expect(await apiFetch<{ ok: number }>("/x")).toEqual({ ok: 1 });
  });
  it("throws ApiError carrying the envelope code on non-2xx", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify({ error: { code: "not_entitled", message: "无额度" } }), { status: 403 })));
    await expect(apiFetch("/x")).rejects.toMatchObject({ code: "not_entitled", status: 403 });
  });
  it("sends credentials and JSON content-type", async () => {
    const spy = vi.fn(async () => new Response("{}", { status: 200 }));
    vi.stubGlobal("fetch", spy);
    await apiFetch("/x", { method: "POST", body: "{}" });
    const init = spy.mock.calls[0]![1];
    expect(init.credentials).toBe("include");
    expect((init.headers as Record<string,string>)["Content-Type"]).toBe("application/json");
  });
});
```

`apps/web/src/api/sse.test.ts`:

```ts
import { describe, it, expect } from "vitest";
import { parseSSE } from "./sse";

function streamOf(...chunks: string[]): ReadableStream<Uint8Array> {
  const enc = new TextEncoder();
  return new ReadableStream({ start(c) { for (const ch of chunks) c.enqueue(enc.encode(ch)); c.close(); } });
}

describe("parseSSE", () => {
  it("parses event+data frames split across chunks, skipping heartbeats", async () => {
    const s = streamOf("event: text\ndata: {\"delta\":\"a\"}\n\n: ping\n\nevent: do", "ne\ndata: {\"message_id\":\"m1\"}\n\n");
    const got: { event: string; data: string }[] = [];
    for await (const f of parseSSE(s)) got.push(f);
    expect(got).toEqual([
      { event: "text", data: '{"delta":"a"}' },
      { event: "done", data: '{"message_id":"m1"}' },
    ]);
  });
});
```

- [ ] **Step 2: Run, watch fail**

Run: `pnpm --filter @mind-imprint/web test -- src/api/client.test.ts src/api/sse.test.ts`
Expected: FAIL — modules don't exist.

- [ ] **Step 3: Implement `client.ts`:**

```ts
export const API_BASE = (import.meta.env.VITE_API_BASE_URL ?? "").replace(/\/+$/, "");

export class ApiError extends Error {
  constructor(public readonly code: string, message: string, public readonly status: number) {
    super(message);
    this.name = "ApiError";
  }
}

export async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...init,
    credentials: "include",
    headers: { "Content-Type": "application/json", ...(init?.headers ?? {}) },
  });
  if (!res.ok) {
    let code = "internal_error";
    let message = `HTTP ${res.status}`;
    try {
      const body = await res.json();
      if (body?.error) { code = body.error.code ?? code; message = body.error.message ?? message; }
    } catch { /* non-JSON error body */ }
    throw new ApiError(code, message, res.status);
  }
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}
```

- [ ] **Step 4: Implement `sse.ts`:**

```ts
export interface SSEFrame { event: string; data: string }

function parseFrame(raw: string): SSEFrame | null {
  let event = "message";
  const dataLines: string[] = [];
  for (const line of raw.split("\n")) {
    if (line === "" || line.startsWith(":")) continue; // blank / heartbeat comment
    if (line.startsWith("event:")) event = line.slice(6).trim();
    else if (line.startsWith("data:")) dataLines.push(line.slice(5).replace(/^ /, ""));
  }
  if (dataLines.length === 0) return null;
  return { event, data: dataLines.join("\n") };
}

export async function* parseSSE(stream: ReadableStream<Uint8Array>): AsyncGenerator<SSEFrame> {
  const reader = stream.getReader();
  const decoder = new TextDecoder();
  let buf = "";
  try {
    for (;;) {
      const { done, value } = await reader.read();
      if (done) break;
      buf += decoder.decode(value, { stream: true });
      let idx: number;
      while ((idx = buf.indexOf("\n\n")) !== -1) {
        const frame = parseFrame(buf.slice(0, idx));
        buf = buf.slice(idx + 2);
        if (frame) yield frame;
      }
    }
  } finally {
    reader.releaseLock();
  }
}
```

- [ ] **Step 5: Run, watch pass**

Run: `pnpm --filter @mind-imprint/web test -- src/api/client.test.ts src/api/sse.test.ts`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add apps/web/src/api/client.ts apps/web/src/api/sse.ts apps/web/src/api/client.test.ts apps/web/src/api/sse.test.ts
git commit -m "feat(web): api client wrapper + SSE frame parser"
```

---

## Task 3: Web — API resource modules + turn stream + ApiClient

**Files:**
- Create: `apps/web/src/api/tasks.ts`, `cards.ts`, `evaluate.ts`, `turn.ts`, `index.ts`
- Test: `apps/web/src/api/tasks.test.ts`, `cards.test.ts`, `evaluate.test.ts`, `turn.test.ts`

**Interfaces:**
- Consumes: `apiFetch`, `ApiError`, `parseSSE`, `API_BASE` (Task 2); contract types.
- Produces:
  - `tasks.ts`: `listTasks(): Promise<Task[]>`, `createTask(i:{title:string;seed:string|null}): Promise<Task>`, `getTask(id:string): Promise<TaskDetail>`, `interface TaskDetail { task:Task; messages:Message[]; cards:CardInstance[] }`
  - `cards.ts`: `activateCard(taskId,cardId): Promise<CardInstance>`, `submitCard(taskId,cardId,env:CardInstance): Promise<CardInstance>`, `skipCard(taskId,cardId,eventTrace:TraceEvent[]): Promise<CardInstance>`
  - `evaluate.ts`: `runEvaluation(taskId): Promise<Evaluation>`, `getEvaluation(taskId): Promise<Evaluation|null>`
  - `turn.ts`: `type TurnEvent`, `runTurn(taskId, userInput?): AsyncGenerator<TurnEvent>`
  - `index.ts`: `interface ApiClient`, `const api: ApiClient`, re-exports of types

- [ ] **Step 1: Write failing tests.** `apps/web/src/api/tasks.test.ts`:

```ts
import { describe, it, expect, vi } from "vitest";
import { listTasks, createTask, getTask } from "./tasks";

const ok = (body: unknown) => vi.fn(async () => new Response(JSON.stringify(body), { status: 200 }));

describe("tasks api", () => {
  it("listTasks unwraps {tasks}", async () => {
    vi.stubGlobal("fetch", ok({ tasks: [{ id: "t1" }] }));
    expect(await listTasks()).toEqual([{ id: "t1" }]);
  });
  it("createTask POSTs and unwraps {task}", async () => {
    const spy = ok({ task: { id: "t2", title: "x" } });
    vi.stubGlobal("fetch", spy);
    const t = await createTask({ title: "x", seed: null });
    expect(t.id).toBe("t2");
    expect(spy.mock.calls[0]![1].method).toBe("POST");
  });
  it("getTask normalizes absent tool_call to null", async () => {
    vi.stubGlobal("fetch", ok({ task: { id: "t3" }, messages: [{ id: "m1", task_id: "t3", role: "assistant", content: "hi", created_at: "z" }], cards: [] }));
    const d = await getTask("t3");
    expect(d.messages[0]!.tool_call).toBeNull();
  });
});
```

`apps/web/src/api/cards.test.ts`:

```ts
import { describe, it, expect, vi } from "vitest";
import { submitCard, skipCard, activateCard } from "./cards";

const ok = (body: unknown) => vi.fn(async () => new Response(JSON.stringify(body), { status: 200 }));
const env = { id: "c1", card_id: "x", task_id: "t1", parent_node_id: null, status: "completed", field_values: { a: 1 }, event_trace: [{ kind: "submit", at: "z" }], rubric_tags: [], created_at: "z", completed_at: "z" } as const;

describe("cards api", () => {
  it("submitCard PUTs status completed + field_values + event_trace", async () => {
    const spy = ok({ card: env });
    vi.stubGlobal("fetch", spy);
    await submitCard("t1", "c1", env as never);
    const [url, init] = spy.mock.calls[0]!;
    expect(url).toContain("/api/v1/tasks/t1/cards/c1");
    expect(init.method).toBe("PUT");
    expect(JSON.parse(init.body)).toMatchObject({ status: "completed", field_values: { a: 1 } });
  });
  it("skipCard POSTs to /skip with event_trace", async () => {
    const spy = ok({ card: { ...env, status: "skipped" } });
    vi.stubGlobal("fetch", spy);
    await skipCard("t1", "c1", env.event_trace as never);
    const [url, init] = spy.mock.calls[0]!;
    expect(url).toContain("/cards/c1/skip");
    expect(init.method).toBe("POST");
  });
  it("activateCard PATCHes status active", async () => {
    const spy = ok({ card: { ...env, status: "active" } });
    vi.stubGlobal("fetch", spy);
    await activateCard("t1", "c1");
    expect(spy.mock.calls[0]![1].method).toBe("PATCH");
  });
});
```

`apps/web/src/api/evaluate.test.ts`:

```ts
import { describe, it, expect, vi } from "vitest";
import { runEvaluation, getEvaluation } from "./evaluate";

describe("evaluate api", () => {
  it("runEvaluation POSTs and unwraps {evaluation}", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify({ evaluation: { task_id: "t1", scores: [], narrative: "n", created_at: "z" } }), { status: 200 })));
    expect((await runEvaluation("t1")).narrative).toBe("n");
  });
  it("getEvaluation returns null on 404", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify({ error: { code: "not_found", message: "无" } }), { status: 404 })));
    expect(await getEvaluation("t1")).toBeNull();
  });
});
```

`apps/web/src/api/turn.test.ts`:

```ts
import { describe, it, expect, vi } from "vitest";
import { runTurn } from "./turn";

function sseResponse(body: string): Response {
  const enc = new TextEncoder();
  const stream = new ReadableStream({ start(c) { c.enqueue(enc.encode(body)); c.close(); } });
  return new Response(stream, { status: 200, headers: { "Content-Type": "text/event-stream" } });
}

describe("runTurn", () => {
  it("maps SSE frames to TurnEvents", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => sseResponse(
      'event: text\ndata: {"delta":"hi"}\n\nevent: card\ndata: {"card_instance_id":"c1","card_id":"sift_craap","nudge_text":"溯源?"}\n\nevent: done\ndata: {"message_id":"m1"}\n\n',
    )));
    const events = [];
    for await (const e of runTurn("t1", "hello")) events.push(e);
    expect(events).toEqual([
      { type: "text", delta: "hi" },
      { type: "card", cardInstanceId: "c1", cardId: "sift_craap", nudgeText: "溯源?" },
      { type: "done", messageId: "m1" },
    ]);
  });
  it("yields a single error event when the gate returns a JSON error (non-SSE)", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify({ error: { code: "not_entitled", message: "无额度" } }), { status: 403 })));
    const events = [];
    for await (const e of runTurn("t1", "hello")) events.push(e);
    expect(events).toEqual([{ type: "error", code: "not_entitled", message: "无额度" }]);
  });
});
```

- [ ] **Step 2: Run, watch fail**

Run: `pnpm --filter @mind-imprint/web test -- src/api/tasks.test.ts src/api/cards.test.ts src/api/evaluate.test.ts src/api/turn.test.ts`
Expected: FAIL — modules missing.

- [ ] **Step 3: Implement `tasks.ts`:**

```ts
import type { Task, Message, CardInstance } from "@mind-imprint/contracts";
import { apiFetch } from "./client";

export interface TaskDetail { task: Task; messages: Message[]; cards: CardInstance[] }

export async function listTasks(): Promise<Task[]> {
  const r = await apiFetch<{ tasks: Task[] }>("/api/v1/tasks");
  return r.tasks;
}

export async function createTask(input: { title: string; seed: string | null }): Promise<Task> {
  const r = await apiFetch<{ task: Task }>("/api/v1/tasks", { method: "POST", body: JSON.stringify(input) });
  return r.task;
}

export async function getTask(id: string): Promise<TaskDetail> {
  const r = await apiFetch<TaskDetail>(`/api/v1/tasks/${id}`);
  // Backend omits tool_call when absent; the Message type carries it as nullable.
  return { ...r, messages: r.messages.map((m) => ({ ...m, tool_call: m.tool_call ?? null })) };
}
```

- [ ] **Step 4: Implement `cards.ts`:**

```ts
import type { CardInstance, TraceEvent } from "@mind-imprint/contracts";
import { apiFetch } from "./client";

export async function activateCard(taskId: string, cardId: string): Promise<CardInstance> {
  const r = await apiFetch<{ card: CardInstance }>(`/api/v1/tasks/${taskId}/cards/${cardId}`, {
    method: "PATCH", body: JSON.stringify({ status: "active" }),
  });
  return r.card;
}

export async function submitCard(taskId: string, cardId: string, env: CardInstance): Promise<CardInstance> {
  const r = await apiFetch<{ card: CardInstance }>(`/api/v1/tasks/${taskId}/cards/${cardId}`, {
    method: "PUT",
    body: JSON.stringify({ status: "completed", field_values: env.field_values, event_trace: env.event_trace }),
  });
  return r.card;
}

export async function skipCard(taskId: string, cardId: string, eventTrace: TraceEvent[]): Promise<CardInstance> {
  const r = await apiFetch<{ card: CardInstance }>(`/api/v1/tasks/${taskId}/cards/${cardId}/skip`, {
    method: "POST", body: JSON.stringify({ event_trace: eventTrace }),
  });
  return r.card;
}
```

- [ ] **Step 5: Implement `evaluate.ts`:**

```ts
import type { Evaluation } from "@mind-imprint/contracts";
import { apiFetch, ApiError } from "./client";

export async function runEvaluation(taskId: string): Promise<Evaluation> {
  const r = await apiFetch<{ evaluation: Evaluation }>(`/api/v1/tasks/${taskId}/evaluate`, { method: "POST" });
  return r.evaluation;
}

export async function getEvaluation(taskId: string): Promise<Evaluation | null> {
  try {
    const r = await apiFetch<{ evaluation: Evaluation }>(`/api/v1/tasks/${taskId}/evaluation`);
    return r.evaluation;
  } catch (e) {
    if (e instanceof ApiError && e.status === 404) return null;
    throw e;
  }
}
```

- [ ] **Step 6: Implement `turn.ts`:**

```ts
import { API_BASE } from "./client";
import { parseSSE } from "./sse";

export type TurnEvent =
  | { type: "text"; delta: string }
  | { type: "card"; cardInstanceId: string; cardId: string; nudgeText: string }
  | { type: "done"; messageId: string }
  | { type: "error"; code: string; message: string };

export async function* runTurn(taskId: string, userInput?: string): AsyncGenerator<TurnEvent> {
  const res = await fetch(`${API_BASE}/api/v1/tasks/${taskId}/turn`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json", Accept: "text/event-stream" },
    body: JSON.stringify({ user_input: userInput ?? "" }),
  });

  // Gate failures (403/404/…) come back as a JSON error envelope, not SSE.
  if (!res.ok || !res.body) {
    let code = "internal_error";
    let message = `HTTP ${res.status}`;
    try { const b = await res.json(); if (b?.error) { code = b.error.code ?? code; message = b.error.message ?? message; } } catch { /* */ }
    yield { type: "error", code, message };
    return;
  }

  for await (const frame of parseSSE(res.body)) {
    let d: Record<string, unknown>;
    try { d = JSON.parse(frame.data); } catch { continue; }
    if (frame.event === "text") yield { type: "text", delta: String(d.delta ?? "") };
    else if (frame.event === "card") yield { type: "card", cardInstanceId: String(d.card_instance_id), cardId: String(d.card_id), nudgeText: String(d.nudge_text ?? "") };
    else if (frame.event === "done") yield { type: "done", messageId: String(d.message_id ?? "") };
    else if (frame.event === "error") {
      const err = (d.error ?? {}) as { code?: string; message?: string };
      yield { type: "error", code: err.code ?? "internal_error", message: err.message ?? "出错了" };
    }
  }
}
```

- [ ] **Step 7: Implement `index.ts` (the `ApiClient` seam):**

```ts
import type { Task, CardInstance, Evaluation, TraceEvent } from "@mind-imprint/contracts";
import { listTasks, createTask, getTask, type TaskDetail } from "./tasks";
import { activateCard, submitCard, skipCard } from "./cards";
import { runEvaluation, getEvaluation } from "./evaluate";
import { runTurn, type TurnEvent } from "./turn";

export type { TaskDetail, TurnEvent };
export { ApiError } from "./client";

export interface ApiClient {
  listTasks(): Promise<Task[]>;
  createTask(input: { title: string; seed: string | null }): Promise<Task>;
  getTask(id: string): Promise<TaskDetail>;
  activateCard(taskId: string, cardId: string): Promise<CardInstance>;
  submitCard(taskId: string, cardId: string, env: CardInstance): Promise<CardInstance>;
  skipCard(taskId: string, cardId: string, eventTrace: TraceEvent[]): Promise<CardInstance>;
  runEvaluation(taskId: string): Promise<Evaluation>;
  getEvaluation(taskId: string): Promise<Evaluation | null>;
  runTurn(taskId: string, userInput?: string): AsyncGenerator<TurnEvent>;
}

export const api: ApiClient = {
  listTasks, createTask, getTask, activateCard, submitCard, skipCard, runEvaluation, getEvaluation, runTurn,
};
```

- [ ] **Step 8: Run, watch pass + typecheck**

Run: `pnpm --filter @mind-imprint/web test -- src/api/ && pnpm -r typecheck`
Expected: PASS

- [ ] **Step 9: Commit**

```bash
git add apps/web/src/api/tasks.ts apps/web/src/api/cards.ts apps/web/src/api/evaluate.ts apps/web/src/api/turn.ts apps/web/src/api/index.ts apps/web/src/api/tasks.test.ts apps/web/src/api/cards.test.ts apps/web/src/api/evaluate.test.ts apps/web/src/api/turn.test.ts
git commit -m "feat(web): api resource modules, turn stream, ApiClient seam"
```

---

## Task 4: Web — store as server-hydrated cache

**Files:**
- Modify: `apps/web/src/store/storage.ts`, `apps/web/src/store/createStore.ts`
- Test: `apps/web/src/store/createStore.test.ts` (+ the messages-cards / storage tests as needed)

**Interfaces:**
- Consumes: contract types.
- Produces (added to `Store`): `putTask(task: Task): void` (upsert by id), `putMessage(message: Message): void` (upsert by id), `hydrateTask(taskId: string, data: { task: Task; messages: Message[]; cards: CardInstance[]; evaluation?: Evaluation }): void` (replace that task's messages+cards, upsert task + evaluation). Default storage becomes in-memory; existing methods/queries unchanged.

- [ ] **Step 1: Make storage default in-memory.** Edit `storage.ts` — keep `RawStorage` + `makeMemoryStorage`, and make `CreateStoreOptions.storage` optional in `createStore` (Step 3). No localStorage requirement remains.

- [ ] **Step 2: Write failing tests** in `apps/web/src/store/createStore.test.ts` (add cases; keep query-method tests, drop any that assert `window.localStorage` writes):

```ts
import { describe, it, expect } from "vitest";
import { createStore } from "./createStore";

const task = (id: string) => ({ id, title: id, seed: null, status: "active" as const, created_at: "1", last_active_at: "1" });
const msg = (id: string, tid: string, content: string) => ({ id, task_id: tid, role: "assistant" as const, content, tool_call: null, created_at: "1" });
const card = (id: string, tid: string, status: any) => ({ id, card_id: "x", task_id: tid, parent_node_id: null, status, field_values: {}, event_trace: [], rubric_tags: [], created_at: "1", completed_at: null });

describe("server-hydrated store", () => {
  it("defaults to in-memory (no storage arg)", () => {
    const s = createStore({});
    expect(s.listTasks()).toEqual([]);
  });
  it("putTask upserts by id", () => {
    const s = createStore({});
    s.putTask(task("t1"));
    s.putTask({ ...task("t1"), title: "renamed" });
    expect(s.listTasks()).toHaveLength(1);
    expect(s.getTask("t1")!.title).toBe("renamed");
  });
  it("putMessage upserts by id (streaming updates)", () => {
    const s = createStore({});
    s.putTask(task("t1"));
    s.putMessage(msg("m1", "t1", "He"));
    s.putMessage(msg("m1", "t1", "Hello"));
    expect(s.listMessages("t1")).toHaveLength(1);
    expect(s.listMessages("t1")[0]!.content).toBe("Hello");
  });
  it("hydrateTask replaces that task's messages+cards and upserts evaluation", () => {
    const s = createStore({});
    s.putTask(task("t1"));
    s.putMessage(msg("temp", "t1", "optimistic"));
    s.putCard(card("ctmp", "t1", "proposed"));
    s.hydrateTask("t1", {
      task: task("t1"),
      messages: [msg("srv1", "t1", "server")],
      cards: [card("csrv", "t1", "completed")],
      evaluation: { task_id: "t1", scores: [], narrative: "n", created_at: "z" },
    });
    expect(s.listMessages("t1").map((m) => m.id)).toEqual(["srv1"]);
    expect(s.listCards("t1").map((c) => c.id)).toEqual(["csrv"]);
    expect(s.getLatestEvaluation("t1")!.narrative).toBe("n");
  });
  it("hydrateTask leaves other tasks untouched", () => {
    const s = createStore({});
    s.putTask(task("t1")); s.putMessage(msg("a", "t1", "keep"));
    s.putTask(task("t2")); s.putMessage(msg("b", "t2", "keep2"));
    s.hydrateTask("t2", { task: task("t2"), messages: [], cards: [] });
    expect(s.listMessages("t1")).toHaveLength(1);
    expect(s.listMessages("t2")).toHaveLength(0);
  });
});
```

- [ ] **Step 3: Run, watch fail**

Run: `pnpm --filter @mind-imprint/web test -- src/store/createStore.test.ts`
Expected: FAIL — `createStore({})` (storage required), `putTask`/`putMessage`/`hydrateTask` undefined.

- [ ] **Step 4: Implement.** In `createStore.ts`: make `storage` optional (default memory), add the three methods. Change the options + the `storage` line:

```ts
import { makeMemoryStorage } from "./storage";
export interface CreateStoreOptions {
  storage?: RawStorage;
  now?: () => string;
  genId?: () => string;
}
// inside createStore:
  const storage = opts.storage ?? makeMemoryStorage();
```

Add to the `Store` interface:

```ts
  putTask(task: Task): void;
  putMessage(message: Message): void;
  hydrateTask(taskId: string, data: { task: Task; messages: Message[]; cards: CardInstance[]; evaluation?: Evaluation }): void;
```

And to the returned object:

```ts
    putTask(task) {
      const idx = state.tasks.findIndex((t) => t.id === task.id);
      const tasks = idx === -1 ? [...state.tasks, task] : state.tasks.map((t) => (t.id === task.id ? task : t));
      commit({ ...state, tasks });
    },
    putMessage(message) {
      const idx = state.messages.findIndex((m) => m.id === message.id);
      const messages = idx === -1 ? [...state.messages, message] : state.messages.map((m) => (m.id === message.id ? message : m));
      commit({ ...state, messages });
    },
    hydrateTask(taskId, data) {
      const tasks = state.tasks.some((t) => t.id === data.task.id)
        ? state.tasks.map((t) => (t.id === data.task.id ? data.task : t))
        : [...state.tasks, data.task];
      const messages = state.messages.filter((m) => m.task_id !== taskId).concat(data.messages);
      const cards = state.cards.filter((c) => c.task_id !== taskId).concat(data.cards);
      const evaluations = data.evaluation
        ? state.evaluations.filter((e) => e.task_id !== taskId).concat(data.evaluation)
        : state.evaluations;
      commit({ ...state, tasks, messages, cards, evaluations });
    },
```

- [ ] **Step 5: Run, watch pass.** Fix/adjust the other store tests (`storage.test.ts`, `createStore.messages-cards.test.ts`, `createStore.evaluations.test.ts`) if they constructed with `{ storage }` — they still work since storage is optional; only DELETE assertions that specifically check `localStorage` round-tripping (none should remain mandatory). Run the whole store suite:

Run: `pnpm --filter @mind-imprint/web test -- src/store/`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add apps/web/src/store/createStore.ts apps/web/src/store/storage.ts apps/web/src/store/createStore.test.ts
git commit -m "feat(web): store as in-memory server-hydrated cache (putTask/putMessage/hydrateTask)"
```

---

## Task 5: Web — API/SSE controllers + WorkspaceContainer rewire

**Files:**
- Rewrite: `apps/web/src/agent/createConversation.ts`, `apps/web/src/agent/createEvaluator.ts`
- Rewrite: `apps/web/src/shell/WorkspaceContainer.tsx`
- Delete: `apps/web/src/dev/WorkspaceDev.tsx` (+ `WorkspaceDev.test.tsx` if present)
- Modify: `apps/web/src/agent/index.ts` (prune exports)
- Rewrite: `apps/web/src/agent/createConversation.test.ts`, `apps/web/src/agent/createEvaluator.test.ts`

**Interfaces:**
- Consumes: `ApiClient`/`api`/`TurnEvent` (Task 3); `Store` + `putMessage`/`putCard`/`appendMessage`/`hydrateTask` (Task 4); `newEnvelope`/`envelopeReducer` (unchanged).
- Produces:
  - `ConversationDeps = { api: Pick<ApiClient,"runTurn"|"activateCard"|"submitCard"|"skipCard">; store: Store; taskId: string; now?: ()=>string; genId?: ()=>string }`; `createConversation(deps): Conversation` (same `Conversation` interface MINUS `kickoff`).
  - `EvaluatorDeps = { api: Pick<ApiClient,"runEvaluation">; store: Store; taskId: string }`; `createEvaluator(deps): Evaluator` (same `Evaluator` interface).
  - `WorkspaceContainer` props `{ store: Store; taskId: string; onBack: ()=>void; openingMessage?: string; chat?: unknown; config?: unknown }` (the `chat`/`config` are accepted-but-ignored so `AppShell` keeps compiling until Task 6 removes them).

- [ ] **Step 1: Write failing controller tests.** `apps/web/src/agent/createConversation.test.ts` (replace the old file):

```ts
import { describe, it, expect } from "vitest";
import { createStore } from "../store/createStore";
import { createConversation } from "./createConversation";
import type { TurnEvent } from "../api";

function fakeApi(script: TurnEvent[]) {
  const calls: { userInput?: string }[] = [];
  return {
    calls,
    async *runTurn(_t: string, userInput?: string) { calls.push({ userInput }); for (const e of script) yield e; },
    async activateCard(_t: string, _c: string) { return undefined as never; },
    async submitCard(_t: string, _c: string, env: any) { return env; },
    async skipCard(_t: string, _c: string, _e: any) { return undefined as never; },
  };
}

const task = { id: "t1", title: "t", seed: null, status: "active" as const, created_at: "1", last_active_at: "1" };

describe("createConversation (API/SSE)", () => {
  it("send streams text into a single assistant message and ends idle", async () => {
    const store = createStore({}); store.putTask(task);
    const api = fakeApi([{ type: "text", delta: "He" }, { type: "text", delta: "llo" }, { type: "done", messageId: "m1" }]);
    const conv = createConversation({ api: api as never, store, taskId: "t1" });
    await conv.send("hi");
    const msgs = store.listMessages("t1");
    expect(msgs.map((m) => m.role)).toEqual(["user", "assistant"]);
    expect(msgs[1]!.content).toBe("Hello");
    expect(conv.getSnapshot().phase).toBe("idle");
    expect(api.calls[0]!.userInput).toBe("hi");
  });

  it("a card event creates a proposed card + proposal_pending and ends the turn", async () => {
    const store = createStore({}); store.putTask(task);
    const api = fakeApi([{ type: "text", delta: "先溯源" }, { type: "card", cardInstanceId: "c1", cardId: "sift_craap", nudgeText: "一起?" }]);
    const conv = createConversation({ api: api as never, store, taskId: "t1" });
    await conv.send("引用公众号");
    expect(store.getCard("c1")!.status).toBe("proposed");
    expect(conv.getSnapshot()).toMatchObject({ phase: "proposal_pending", pendingCardId: "c1" });
    const assistant = store.listMessages("t1").find((m) => m.role === "assistant")!;
    expect((assistant.tool_call as any).card_instance_id).toBe("c1");
  });

  it("submitCard fires a continuation turn (no user input)", async () => {
    const store = createStore({}); store.putTask(task);
    store.putCard({ id: "c1", card_id: "sift_craap", task_id: "t1", parent_node_id: null, status: "active", field_values: {}, event_trace: [], rubric_tags: [], created_at: "1", completed_at: null });
    const api = fakeApi([{ type: "text", delta: "很好" }, { type: "done", messageId: "m2" }]);
    const conv = createConversation({ api: api as never, store, taskId: "t1" });
    const final = { ...store.getCard("c1")!, status: "completed" as const, field_values: { sift: { stop: "x" } } };
    await conv.submitCard("c1", final);
    expect(api.calls.at(-1)!.userInput).toBeUndefined(); // continuation turn
    expect(conv.getSnapshot().phase).toBe("idle");
  });

  it("an error event sets phase error with the safe message", async () => {
    const store = createStore({}); store.putTask(task);
    const api = fakeApi([{ type: "error", code: "not_entitled", message: "无额度" }]);
    const conv = createConversation({ api: api as never, store, taskId: "t1" });
    await conv.send("hi");
    expect(conv.getSnapshot()).toMatchObject({ phase: "error", error: "无额度" });
  });
});
```

`apps/web/src/agent/createEvaluator.test.ts` (replace):

```ts
import { describe, it, expect } from "vitest";
import { createStore } from "../store/createStore";
import { createEvaluator } from "./createEvaluator";

const task = { id: "t1", title: "t", seed: null, status: "active" as const, created_at: "1", last_active_at: "1" };

describe("createEvaluator (API)", () => {
  it("run() calls the API, stores the evaluation, and ends done", async () => {
    const store = createStore({}); store.putTask(task);
    const evaluation = { task_id: "t1", scores: [{ dim_id: "D1", level: "L3", note: "n" }], narrative: "印记", created_at: "z" };
    const api = { async runEvaluation() { return evaluation; } };
    const ev = createEvaluator({ api: api as never, store, taskId: "t1" });
    await ev.run();
    expect(ev.getSnapshot()).toMatchObject({ phase: "done", evaluation });
    expect(store.getLatestEvaluation("t1")!.narrative).toBe("印记");
  });
  it("run() surfaces an error", async () => {
    const store = createStore({}); store.putTask(task);
    const api = { async runEvaluation() { throw new Error("评估失败"); } };
    const ev = createEvaluator({ api: api as never, store, taskId: "t1" });
    await ev.run();
    expect(ev.getSnapshot()).toMatchObject({ phase: "error", error: "评估失败" });
  });
});
```

- [ ] **Step 2: Run, watch fail**

Run: `pnpm --filter @mind-imprint/web test -- src/agent/createConversation.test.ts src/agent/createEvaluator.test.ts`
Expected: FAIL — old factories have different deps / `kickoff` etc.

- [ ] **Step 3: Rewrite `createConversation.ts`:**

```ts
import type { CardInstance } from "@mind-imprint/contracts";
import type { ApiClient } from "../api";
import type { Store } from "../store/createStore";
import { newEnvelope, envelopeReducer } from "../cards/envelopeReducer";

export type ConvPhase = "idle" | "awaiting_llm" | "proposal_pending" | "card_active" | "error";
export interface ConvState { taskId: string; phase: ConvPhase; pendingCardId?: string; error?: string }

export interface ConversationDeps {
  api: Pick<ApiClient, "runTurn" | "activateCard" | "submitCard" | "skipCard">;
  store: Store;
  taskId: string;
  now?: () => string;
  genId?: () => string;
}

export interface Conversation {
  getSnapshot(): ConvState;
  subscribe(listener: () => void): () => void;
  send(text: string): Promise<void>;
  openCard(cardInstanceId: string): void;
  closeCard(cardInstanceId: string): void;
  submitCard(cardInstanceId: string, finalInstance: CardInstance): Promise<void>;
  skipCard(cardInstanceId: string): Promise<void>;
}

export function createConversation(deps: ConversationDeps): Conversation {
  const { api, store, taskId } = deps;
  const now = deps.now ?? (() => new Date().toISOString());
  const genId = deps.genId ?? (() => crypto.randomUUID());

  let state: ConvState = { taskId, phase: "idle" };
  const listeners = new Set<() => void>();
  function setState(next: Partial<ConvState>): void {
    const merged = { ...state, ...next };
    if ((Object.keys(merged) as (keyof ConvState)[]).some((k) => merged[k] !== state[k])) {
      state = merged;
      listeners.forEach((l) => l());
    }
  }

  // Stream one turn; userInput undefined ⇒ continuation turn (after a card).
  async function streamTurn(userInput?: string): Promise<void> {
    setState({ phase: "awaiting_llm", error: undefined, pendingCardId: undefined });
    const assistantId = genId();
    let text = "";
    try {
      for await (const ev of api.runTurn(taskId, userInput)) {
        if (ev.type === "text") {
          text += ev.delta;
          store.putMessage({ id: assistantId, task_id: taskId, role: "assistant", content: text, tool_call: null, created_at: now() });
        } else if (ev.type === "card") {
          const ci = newEnvelope(ev.cardId, taskId, now, () => ev.cardInstanceId);
          store.putCard(ci);
          store.putMessage({
            id: assistantId, task_id: taskId, role: "assistant", content: text, created_at: now(),
            tool_call: { id: ev.cardInstanceId, name: "summon_card", args: { card_id: ev.cardId, reason: "", nudge_text: ev.nudgeText }, card_instance_id: ev.cardInstanceId },
          });
          setState({ phase: "proposal_pending", pendingCardId: ev.cardInstanceId });
          return; // one card per turn — the card ends the turn
        } else if (ev.type === "done") {
          store.putMessage({ id: assistantId, task_id: taskId, role: "assistant", content: text, tool_call: null, created_at: now() });
          setState({ phase: "idle", pendingCardId: undefined });
          return;
        } else if (ev.type === "error") {
          setState({ phase: "error", error: ev.message });
          return;
        }
      }
      setState({ phase: "idle" });
    } catch (err) {
      setState({ phase: "error", error: err instanceof Error ? err.message : String(err) });
    }
  }

  return {
    getSnapshot: () => state,
    subscribe(listener) { listeners.add(listener); return () => { listeners.delete(listener); }; },

    async send(text) {
      store.appendMessage({ task_id: taskId, role: "user", content: text });
      await streamTurn(text);
    },

    openCard(cardInstanceId) {
      const ci = store.getCard(cardInstanceId);
      if (!ci) throw new Error(`[conversation] unknown card "${cardInstanceId}"`);
      store.putCard(envelopeReducer(ci, { type: "activate" }, now));
      setState({ phase: "card_active", pendingCardId: cardInstanceId });
      // Mark opened on the server so the live tree reflects it (non-blocking).
      void api.activateCard(taskId, cardInstanceId).then((c) => store.putCard(c)).catch(() => {});
    },

    closeCard() {
      setState({ phase: "idle", pendingCardId: undefined }); // close ≠ skip
    },

    async submitCard(cardInstanceId, finalInstance) {
      store.putCard(finalInstance); // optimistic completed
      setState({ phase: "awaiting_llm", pendingCardId: undefined });
      try {
        const server = await api.submitCard(taskId, cardInstanceId, finalInstance);
        store.putCard(server);
      } catch (err) {
        setState({ phase: "error", error: err instanceof Error ? err.message : String(err) });
        return;
      }
      await streamTurn(); // continuation turn
    },

    async skipCard(cardInstanceId) {
      const ci = store.getCard(cardInstanceId);
      if (!ci) throw new Error(`[conversation] unknown card "${cardInstanceId}"`);
      const skipped = envelopeReducer(ci, { type: "skip" }, now);
      store.putCard(skipped);
      setState({ phase: "awaiting_llm", pendingCardId: undefined });
      try {
        const server = await api.skipCard(taskId, cardInstanceId, skipped.event_trace);
        store.putCard(server);
      } catch (err) {
        setState({ phase: "error", error: err instanceof Error ? err.message : String(err) });
        return;
      }
      await streamTurn(); // continuation turn
    },
  };
}
```

- [ ] **Step 4: Rewrite `createEvaluator.ts`:**

```ts
import type { Evaluation } from "@mind-imprint/contracts";
import type { ApiClient } from "../api";
import type { Store } from "../store/createStore";

export type EvalPhase = "idle" | "running" | "done" | "error";
export interface EvalState { phase: EvalPhase; evaluation?: Evaluation; error?: string }

export interface EvaluatorDeps { api: Pick<ApiClient, "runEvaluation">; store: Store; taskId: string }

export interface Evaluator {
  getSnapshot(): EvalState;
  subscribe(listener: () => void): () => void;
  run(): Promise<void>;
}

export function createEvaluator(deps: EvaluatorDeps): Evaluator {
  const { api, store, taskId } = deps;
  let state: EvalState = { phase: "idle" };
  const listeners = new Set<() => void>();
  function setState(next: Partial<EvalState>): void {
    const merged = { ...state, ...next };
    if ((Object.keys(merged) as (keyof EvalState)[]).some((k) => merged[k] !== state[k])) {
      state = merged;
      listeners.forEach((l) => l());
    }
  }
  return {
    getSnapshot: () => state,
    subscribe(listener) { listeners.add(listener); return () => { listeners.delete(listener); }; },
    async run() {
      setState({ phase: "running", error: undefined });
      try {
        const evaluation = await api.runEvaluation(taskId);
        store.putEvaluation(evaluation);
        setState({ phase: "done", evaluation });
      } catch (e) {
        setState({ phase: "error", error: e instanceof Error ? e.message : String(e) });
      }
    },
  };
}
```

- [ ] **Step 5: Rewrite `WorkspaceContainer.tsx`:**

```tsx
import { useMemo, useEffect } from "react";
import { api } from "../api";
import { createConversation } from "../agent/createConversation";
import { createEvaluator } from "../agent/createEvaluator";
import { WorkspaceView } from "../workspace";
import type { Store } from "../store/createStore";

interface Props {
  store: Store;
  taskId: string;
  onBack: () => void;
  openingMessage?: string;
  // Accepted-but-ignored until AppShell stops passing them (Task 6).
  chat?: unknown;
  config?: unknown;
}

export function WorkspaceContainer({ store, taskId, onBack, openingMessage }: Props) {
  const { conversation, evaluator } = useMemo(
    () => ({
      conversation: createConversation({ api, store, taskId }),
      evaluator: createEvaluator({ api, store, taskId }),
    }),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [taskId],
  );

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const detail = await api.getTask(taskId);
        if (cancelled) return;
        const evaluation = await api.getEvaluation(taskId);
        if (cancelled) return;
        store.hydrateTask(taskId, { ...detail, evaluation: evaluation ?? undefined });
      } catch {
        // hydration failure leaves the cache as-is; a fresh task simply has none
      }
      if (cancelled) return;
      // Fresh task created from the directory: no server messages yet — send the opening line.
      if (openingMessage && store.listMessages(taskId).length === 0) {
        void conversation.send(openingMessage);
      }
    })();
    return () => { cancelled = true; };
  }, [taskId, conversation, openingMessage, store]);

  return <WorkspaceView store={store} conversation={conversation} evaluator={evaluator} taskId={taskId} onBack={onBack} />;
}
```

- [ ] **Step 6: Delete `WorkspaceDev`** (it constructs the old controller and would break typecheck):

```bash
git rm apps/web/src/dev/WorkspaceDev.tsx
# also remove its test if present:
git rm apps/web/src/dev/WorkspaceDev.test.tsx 2>&1 || true
```

If `apps/web/src/dev/DevApp.tsx` imports `WorkspaceDev`, remove that route/import (read it; keep the `?demo` `Harness` path intact).

- [ ] **Step 7: Prune `apps/web/src/agent/index.ts`** to the surviving modules only:

```ts
export { createConversation } from "./createConversation";
export type { Conversation, ConvState, ConvPhase, ConversationDeps } from "./createConversation";
export { createEvaluator } from "./createEvaluator";
export type { Evaluator, EvalState, EvalPhase, EvaluatorDeps } from "./createEvaluator";
export { useConversation } from "./useConversation";
export { useEvaluator } from "./useEvaluator";
```

(The old `prompt` / `messageMapping` / `runEvaluation` / `evalInput` / `evalPrompt` files still exist on disk and still compile standalone — they are deleted in Task 7. Just stop re-exporting them here.)

- [ ] **Step 8: Run tests + typecheck**

Run: `pnpm --filter @mind-imprint/web test -- src/agent/createConversation.test.ts src/agent/createEvaluator.test.ts && pnpm -r typecheck`
Expected: PASS. (Typecheck must be green — `AppShell` still passes `chat`/`config` to `WorkspaceContainer`, which now accepts-and-ignores them; the old `prompt.ts` etc. still compile.)

- [ ] **Step 9: Commit**

```bash
git add apps/web/src/agent/createConversation.ts apps/web/src/agent/createEvaluator.ts apps/web/src/agent/createConversation.test.ts apps/web/src/agent/createEvaluator.test.ts apps/web/src/agent/index.ts apps/web/src/shell/WorkspaceContainer.tsx
git add -u apps/web/src/dev
git commit -m "feat(web): API/SSE conversation + evaluator controllers; rewire WorkspaceContainer"
```

---

## Task 6: Web — shell rewire (AppShell, DirectoryView, SettingsView, AuthScreen)

**Files:**
- Modify: `apps/web/src/shell/AppShell.tsx`, `apps/web/src/shell/directory/DirectoryView.tsx`, `apps/web/src/shell/settings/SettingsView.tsx`, `apps/web/src/shell/WorkspaceContainer.tsx` (drop the now-unused `chat`/`config` props)
- Modify if needed: `apps/web/src/shell/auth/AuthScreen.tsx`
- Test: `apps/web/src/shell/AppShell.test.tsx`, `apps/web/src/shell/directory/DirectoryView.test.tsx`, `apps/web/src/shell/settings/SettingsView.test.tsx`

**Interfaces:**
- Consumes: `api` (Task 3), the rewired controllers + `WorkspaceContainer` (Task 5), the store (Task 4).
- Produces: `DirectoryView` prop `onOpenTask(taskId: string, openingText?: string) => void`; `AppShell` holds `openingMessage` and passes it to `WorkspaceContainer`; no LLM-key gate; `WorkspaceContainer` props lose `chat`/`config`.

- [ ] **Step 1: Write failing tests.** `DirectoryView.test.tsx` — creating a task calls `api.createTask` and navigates with the opening text (mock the api module):

```tsx
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { createStore } from "../../store/createStore";
import { DirectoryView } from "./DirectoryView";

vi.mock("../../api", () => ({
  api: {
    listTasks: vi.fn(async () => []),
    createTask: vi.fn(async (i: any) => ({ id: "t-new", title: i.title, seed: i.seed, status: "active", created_at: "1", last_active_at: "1" })),
  },
}));

describe("DirectoryView", () => {
  beforeEach(() => vi.clearAllMocks());
  it("creates a task via the API and opens it with the opening text", async () => {
    const store = createStore({});
    const onOpenTask = vi.fn();
    render(<DirectoryView store={store} onOpenTask={onOpenTask} />);
    fireEvent.change(screen.getByPlaceholderText(/把你正在纠结的问题/), { target: { value: "中国是否让地球更可持续？https://x" } });
    fireEvent.click(screen.getByText("开始"));
    await waitFor(() => expect(onOpenTask).toHaveBeenCalledWith("t-new", "中国是否让地球更可持续？https://x"));
  });
});
```

`AppShell.test.tsx` — renders the app (authed) without any LLM key gate (mock `api.listTasks`):

```tsx
import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { AppShell } from "./AppShell";
import { createStore } from "../store/createStore";
import { createSession } from "./session";

vi.mock("../api", () => ({ api: { listTasks: vi.fn(async () => []), createTask: vi.fn() } }));

describe("AppShell", () => {
  it("shows the directory directly when authed — no key gate", () => {
    const store = createStore({});
    const session = createSession({ storage: { getItem: () => JSON.stringify({ authed: true, aiAvatar: "" }), setItem: () => {} } });
    render(<AppShell store={store} session={session} />);
    expect(screen.getByText("今天你在尝试什么？")).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run, watch fail**

Run: `pnpm --filter @mind-imprint/web test -- src/shell/directory/DirectoryView.test.tsx src/shell/AppShell.test.tsx`
Expected: FAIL — DirectoryView still uses `store.createTask` (no api); AppShell still imports the key gate.

- [ ] **Step 3: Rewrite `DirectoryView.tsx`** — load tasks from the server, create via the API:

```tsx
import { useSyncExternalStore, useState, useEffect } from "react";
import type { Store } from "../../store/createStore";
import { api } from "../../api";
import { taskCardView } from "./taskCardView";

export function DirectoryView({
  store,
  onOpenTask,
  now,
}: {
  store: Store;
  onOpenTask: (taskId: string, openingText?: string) => void;
  now?: () => Date;
}) {
  const _now = now ?? (() => new Date());
  useSyncExternalStore(store.subscribe, store.getSnapshot);
  useEffect(() => {
    void api.listTasks().then((ts) => ts.forEach((t) => store.putTask(t))).catch(() => {});
  }, [store]);
  const tasks = store.listTasks();
  const taskCount = tasks.length;
  const [input, setInput] = useState("");

  async function handleStart() {
    const trimmed = input.trim();
    if (!trimmed) return;
    const urlMatch = trimmed.match(/(https?:\/\/\S+)/);
    const seed = urlMatch ? urlMatch[1]! : null;
    const t = await api.createTask({ title: trimmed, seed });
    store.putTask(t);
    onOpenTask(t.id, trimmed);
  }
  // … the rest of the JSX is UNCHANGED (button calls `void handleStart()`) …
}
```

> Keep the entire JSX body identical to the current file; only change the imports, add the `useEffect` list-load, swap `handleStart`'s body to the async API version, widen `onOpenTask`, and make the button `onClick={() => void handleStart()}`.

- [ ] **Step 4: Rewrite `AppShell.tsx`** — in-memory store, no key gate, thread `openingMessage`:

```tsx
import { useState } from "react";
import { createStore } from "../store";
import { createSession, useSession, type SessionStore } from "./session";
import { CARD_REGISTRY } from "@mind-imprint/contracts";
import type { Store } from "../store/createStore";
import { AuthScreen } from "./auth/AuthScreen";
import { LeftRail } from "./LeftRail";
import { DirectoryView } from "./directory/DirectoryView";
import { WorkspaceContainer } from "./WorkspaceContainer";
import { RecordsView } from "./records/RecordsView";
import { SettingsView } from "./settings/SettingsView";

const defaultStore = createStore({});
const defaultSession = createSession({ storage: window.localStorage });

type Tab = "tasks" | "records" | "settings";
type TaskView = "directory" | "workspace";

export function AppShell({
  store = defaultStore,
  session = defaultSession,
}: {
  store?: Store;
  session?: SessionStore;
}) {
  const sess = useSession(session);
  const screen = sess.authed ? "app" : "auth";

  const [tab, setTab] = useState<Tab>("tasks");
  const [taskView, setTaskView] = useState<TaskView>("directory");
  const [activeTaskId, setActiveTaskId] = useState<string | null>(null);
  const [openingMessage, setOpeningMessage] = useState<string | undefined>(undefined);

  if (screen === "auth") {
    return <AuthScreen onEnterApp={() => session.setAuthed(true)} />;
  }

  return (
    <div style={{ display: "flex", height: "100%", width: "100%", background: "#F3F4F8", overflow: "hidden" }}>
      <LeftRail tab={tab} onTab={setTab} />
      <div style={{ flex: 1, overflow: "hidden", position: "relative" }}>
        {tab === "tasks" && taskView === "directory" && (
          <DirectoryView
            store={store}
            onOpenTask={(id, opening) => {
              setActiveTaskId(id);
              setOpeningMessage(opening);
              setTaskView("workspace");
            }}
          />
        )}
        {tab === "tasks" && taskView === "workspace" && (
          <WorkspaceContainer
            store={store}
            taskId={activeTaskId!}
            openingMessage={openingMessage}
            onBack={() => { setTaskView("directory"); setOpeningMessage(undefined); }}
          />
        )}
        {tab === "records" && <RecordsView store={store} registry={CARD_REGISTRY} />}
        {tab === "settings" && (
          <SettingsView session={session} onLogout={() => session.setAuthed(false)} />
        )}
      </div>
    </div>
  );
}
```

- [ ] **Step 5: Drop the ignored props from `WorkspaceContainer.tsx`** — remove `chat?`/`config?` from `Props` (AppShell no longer passes them).

- [ ] **Step 6: Trim `SettingsView.tsx`** — remove the `chat` prop and the model/API-key card (the block embedding `LlmConfigForm`) and its import; keep avatar/profile/logout. Read the file; delete only the LLM config card + the `chat` prop + the `LlmConfigForm` import. If `SettingsView.test.tsx` asserted the LLM card, update it to assert the remaining profile/logout UI.

- [ ] **Step 7: AuthScreen** — confirm `AuthScreen.tsx` has no `src/llm` import and `onEnterApp` just flips the session (it already does). If it imports anything LLM-related, remove it. Usually no change.

- [ ] **Step 8: Run tests + typecheck**

Run: `pnpm --filter @mind-imprint/web test -- src/shell/ && pnpm -r typecheck`
Expected: PASS. AppShell/DirectoryView/SettingsView no longer import `src/llm`; the app compiles. (`src/llm/**` still exists on disk — deleted in Task 7.)

- [ ] **Step 9: Commit**

```bash
git add apps/web/src/shell/AppShell.tsx apps/web/src/shell/directory/DirectoryView.tsx apps/web/src/shell/settings/SettingsView.tsx apps/web/src/shell/WorkspaceContainer.tsx apps/web/src/shell/auth/AuthScreen.tsx apps/web/src/shell/AppShell.test.tsx apps/web/src/shell/directory/DirectoryView.test.tsx apps/web/src/shell/settings/SettingsView.test.tsx
git commit -m "feat(web): shell rewire to API — no key gate, server task list, opening-message threading"
```

---

## Task 7: Web — delete dead LLM/agent code, env, final gate

**Files:**
- Delete: `apps/web/src/llm/**` (all), `apps/web/src/agent/{prompt,messageMapping,runEvaluation,evalInput,evalPrompt}.ts` + their `.test.ts`, `apps/web/src/shell/KeyGateModal.tsx` (+ test), `apps/web/src/shell/settings/LlmConfigForm.tsx` (+ test), `apps/web/src/dev/SettingsPanel.tsx` (+ test)
- Modify/Create: `apps/web/.env.example`, `apps/web/vite.config.ts` (optional dev proxy), `README` note if present
- Verify: the whole repo gate

**Interfaces:** none produced — pure removal + env. Nothing should import the deleted modules after Tasks 5–6.

- [ ] **Step 1: Confirm no live imports remain.** From `apps/web`:

Run: `cd apps/web && echo "$(grep -rn "from \"\.\./llm\|from \"\.\./\.\./llm\|/agent/prompt\|/agent/messageMapping\|/agent/runEvaluation\|/agent/evalInput\|/agent/evalPrompt\|KeyGateModal\|LlmConfigForm\|SettingsPanel" src --include=*.ts --include=*.tsx 2>&1)"`
Expected: only matches inside the files being deleted (or none). If a live file still imports them, fix that import first.

- [ ] **Step 2: Delete the dead modules + tests:**

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git rm -r apps/web/src/llm
git rm apps/web/src/agent/prompt.ts apps/web/src/agent/messageMapping.ts apps/web/src/agent/runEvaluation.ts apps/web/src/agent/evalInput.ts apps/web/src/agent/evalPrompt.ts
git rm apps/web/src/agent/prompt.test.ts apps/web/src/agent/messageMapping.test.ts apps/web/src/agent/runEvaluation.test.ts apps/web/src/agent/evalInput.test.ts apps/web/src/agent/evalPrompt.test.ts
git rm apps/web/src/shell/KeyGateModal.tsx apps/web/src/shell/KeyGateModal.test.tsx
git rm apps/web/src/shell/settings/LlmConfigForm.tsx apps/web/src/shell/settings/LlmConfigForm.test.tsx
git rm apps/web/src/dev/SettingsPanel.tsx apps/web/src/dev/SettingsPanel.test.tsx
```

> If any listed path doesn't exist, drop it from the command. Use `git status` to confirm only intended deletions are staged (NOT the repo-root `package.json`).

- [ ] **Step 3: Add `apps/web/.env.example`:**

```
# Base origin of the Go API (the client prefixes /api/v1). Empty = same origin.
VITE_API_BASE_URL=http://localhost:8080
```

(Optional) add a Vite dev proxy in `vite.config.ts` so same-origin cookies work in dev without CORS:

```ts
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: { proxy: { "/api": { target: "http://localhost:8080", changeOrigin: true } } },
});
```

(If you add the proxy, set `VITE_API_BASE_URL=` empty in dev so calls go to `/api/v1` on the Vite origin and proxy through.)

- [ ] **Step 4: Run the FULL web gate**

Run: `cd /Users/houyuxin/08Coding/mind-imprint && pnpm -r typecheck && pnpm -r test`
Expected: PASS. No dangling imports; no `import.meta.env.VITE_LLM_*` references remain (grep to confirm: `grep -rn "VITE_LLM_" apps/web/src 2>&1` → empty).

- [ ] **Step 5: Build**

Run: `pnpm --filter @mind-imprint/web build`
Expected: succeeds (no unresolved imports).

- [ ] **Step 6: Commit**

```bash
git add -A apps/web/src apps/web/.env.example apps/web/vite.config.ts
git status --short   # verify package.json is NOT staged
git commit -m "chore(web): delete browser LLM/key code; add VITE_API_BASE_URL"
```

---

## Self-Review (completed during planning)

**Spec coverage (2026-06-25-p1-4-frontend-rewire-design.md):**
- Preserve `Store` + `Conversation`/`Evaluator` shapes, swap internals → Tasks 4/5. ✅
- Continuation-turn backend addition → Task 1. ✅
- New `src/api/` (client/sse/turn/tasks/cards/evaluate/index) → Tasks 2/3. ✅
- Delete `src/llm/`, browser turn-loop/eval, KeyGate/LlmConfigForm/SettingsPanel/WorkspaceDev → Tasks 5 (WorkspaceDev) + 7. ✅
- Data-flow lifecycle (hydrate / send / card submit→continuation / skip / evaluate) → Tasks 5/6. ✅
- Auth scope (no login, `credentials:'include'`, AuthScreen pass-through) → Tasks 2/6. ✅
- Card runtime / view-models untouched → confirmed (not in any task's edit list except imports). ✅
- `VITE_API_BASE_URL`, remove `VITE_LLM_*` → Task 7. ✅
- RecordsView N-fetch limitation → accepted; RecordsView reads the hydrated cache (DirectoryView lists tasks; opening a task hydrates its detail). Documented; no code beyond that in scope. ✅

**Placeholder scan:** no TBD/TODO; every code step carries complete code or an exact file-edit instruction. The "rest of JSX unchanged" notes (DirectoryView) name exactly what changes.

**Type consistency:** `Conversation`/`Evaluator`/`ConvState`/`EvalState` match across Tasks 5/6 and the preserved `WorkspaceView`; `ApiClient`/`TurnEvent`/`TaskDetail` are defined in Task 3 and consumed verbatim in Tasks 5/6; `Store` additions (`putTask`/`putMessage`/`hydrateTask`) are defined in Task 4 and used in Tasks 5/6; the continuation turn (empty `user_input`) is produced in Task 1 and consumed by `streamTurn()`'s no-arg `runTurn` in Task 5.

**Cross-task green:** each task ends with typecheck+test green. The one signature change that ripples (controller deps, Task 5) is contained by (a) deleting `WorkspaceDev` in Task 5 and (b) `WorkspaceContainer` accepting-and-ignoring `chat`/`config` until Task 6 removes them. `src/llm/**` and the dead `agent/*` files stay on disk (compiling standalone) until nothing imports them, then are deleted in Task 7.
