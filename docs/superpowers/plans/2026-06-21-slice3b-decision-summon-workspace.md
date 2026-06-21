# Slice 3b — 决策层 + summon_card 循环 + 工作区 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the Phoebe main artery work end-to-end on a real LLM: the student chats in the workspace, the AI calls `summon_card` when a moment genuinely fits, the student opens/fills/skips the card, the filled content re-feeds, and the AI keeps coaching — all frontend-direct, persisted to the B2.5 store, routing over all 33 cards.

**Architecture:** Five layers, reusing existing scaffolding, no frozen-contract changes. (1) LLM tool-use added to `apps/web/src/llm/` (both provider formats). (2) `SummonCardCall` contract + a schema-derived re-feed serializer in `@mind-imprint/contracts`. (3) Decision layer `apps/web/src/agent/` (system prompt + 33-card catalog + `summon_card` tool def). (4) A `createConversation` orchestrator (state machine + store→LLM message mapping) + `useConversation` hook. (5) The pixel-faithful workspace `apps/web/src/workspace/` mounting the existing S1 `CardRenderer` in a bottom sheet.

**Tech Stack:** TypeScript 5 (ES2022), Zod 3, React 18 (`useSyncExternalStore`), Vitest + @testing-library/react (jsdom). Non-streaming, BYO-key.

## Global Constraints

- **Acceptance gate:** `pnpm -r typecheck` AND `pnpm -r test` both green. `vite build`/`vitest` do NOT typecheck — run `pnpm --filter @mind-imprint/contracts typecheck` and/or `pnpm --filter web typecheck` in every task that touches that package.
- **No backend, BYO-key, key-safety:** all LLM calls stay frontend-direct; the API key goes only in request headers, **never** into `raw`/errors/logs/render (preserve the S2-verified boundary).
- **Frozen contracts unchanged:** `CardInstance`/`Message` field shapes do not change. `Message.tool_call` carries a `SummonCardCall` (its shape is defined THIS slice in contracts and **parsed on read**, never `as`).
- **Registry is the single source of truth:** the catalog is a projection (`deriveCatalog`); `summon_card` is ONE tool, not one-per-card. `card_id` is validated against the registry (dangling/unknown ids are dropped gracefully, never thrown).
- **Reuse, don't rewrite:** card activation/fill/submit reuses S1 `envelopeReducer` + `<CardRenderer>`; conversation/messages/cards persist via the B2.5 store (`createStore`/`useStore`); LLM calls go through the S2 `chat()`.
- **Four 铁律:** AI 克制 (mentor prompt + deterministic re-feed, never concludes for the student) · 不操纵 (card auto-proposed but "open" is the student's click) · 一次只聚焦一步 · 过程即数据 (skip also re-feeds as a `skipped` signal).
- **UI is pixel-faithful** to `docs/design/思维印记_工作区.dc.html` (the binding source); use `mk-*` tokens + real Phoebe content (NASA / Nature Sustainability / 中国碳排放), never lorem.
- Contracts tests live in `packages/contracts/test/*`; web tests beside source in `apps/web/src/**`.

---

### Task 1: `SummonCardCall` contract

**Files:**
- Create: `packages/contracts/src/summonCard.ts`
- Modify: `packages/contracts/src/index.ts` (barrel export)
- Test: `packages/contracts/test/summonCard.test.ts`

**Interfaces — Produces:** `SummonCardArgs`, `SummonCardCall` (Zod + inferred types).

- [ ] **Step 1: Write the failing test** — `packages/contracts/test/summonCard.test.ts`:

```ts
import { describe, it, expect } from "vitest";
import { SummonCardCall } from "../src/summonCard";

const valid = {
  id: "call_1", name: "summon_card",
  args: { card_id: "sift_craap", reason: "学生要直接采信一个来源", nudge_text: "先核一下这个来源？" },
  card_instance_id: "ci_1",
};

describe("SummonCardCall", () => {
  it("accepts a valid summon_card call", () => {
    expect(SummonCardCall.safeParse(valid).success).toBe(true);
  });
  it("rejects a wrong tool name", () => {
    expect(SummonCardCall.safeParse({ ...valid, name: "other" }).success).toBe(false);
  });
  it("rejects when args are incomplete", () => {
    expect(SummonCardCall.safeParse({ ...valid, args: { card_id: "x" } }).success).toBe(false);
  });
  it("rejects a missing card_instance_id", () => {
    const { card_instance_id, ...rest } = valid;
    expect(SummonCardCall.safeParse(rest).success).toBe(false);
  });
});
```

- [ ] **Step 2: Run to verify it fails** — `pnpm --filter @mind-imprint/contracts exec vitest run test/summonCard.test.ts` → FAIL (module missing).

- [ ] **Step 3: Implement** — `packages/contracts/src/summonCard.ts`:

```ts
import { z } from "zod";

export const SummonCardArgs = z.object({
  card_id: z.string(),
  reason: z.string(),
  nudge_text: z.string(),
});

export const SummonCardCall = z.object({
  id: z.string(),
  name: z.literal("summon_card"),
  args: SummonCardArgs,
  card_instance_id: z.string(),
});

export type SummonCardArgs = z.infer<typeof SummonCardArgs>;
export type SummonCardCall = z.infer<typeof SummonCardCall>;
```

Add `export * from "./summonCard";` to `packages/contracts/src/index.ts`.

- [ ] **Step 4: Run test + typecheck** — test PASS (4); `pnpm --filter @mind-imprint/contracts typecheck` clean.
- [ ] **Step 5: Commit** — `git commit -am "feat(contracts): SummonCardCall schema"`

---

### Task 2: Re-feed serializer

**Files:**
- Create: `packages/contracts/src/refeed.ts`
- Modify: `packages/contracts/src/index.ts`
- Test: `packages/contracts/test/refeed.test.ts`

**Interfaces:**
- Consumes: `CardSpec`, `CardInstance` from contracts.
- Produces: `RefeedPayload`, `serializeCardForRefeed(spec: CardSpec, instance: CardInstance): RefeedPayload`.

- [ ] **Step 1: Write the failing test** — `packages/contracts/test/refeed.test.ts`:

```ts
import { describe, it, expect } from "vitest";
import { serializeCardForRefeed } from "../src/refeed";
import { CARD_REGISTRY } from "../src/registry";
import type { CardInstance } from "../src/envelope";

const sift = CARD_REGISTRY.sift_craap;

function inst(status: CardInstance["status"], field_values: Record<string, unknown>): CardInstance {
  return {
    id: "ci_1", card_id: "sift_craap", task_id: "t_1", parent_node_id: null,
    status, field_values, event_trace: [], rubric_tags: [],
    created_at: "2026-06-21T10:00:00.000Z", completed_at: null,
  };
}

describe("serializeCardForRefeed", () => {
  it("skipped → only identity + status, no steps", () => {
    const p = serializeCardForRefeed(sift, inst("skipped", {}));
    expect(p).toEqual({ card_id: "sift_craap", card_name: sift.name, status: "skipped" });
  });

  it("completed → labels paired with field_values, repeatable_group remapped to label:value", () => {
    const p = serializeCardForRefeed(sift, inst("completed", {
      sift: {
        stop: "证明中国让地球更可持续",
        sources: [
          { name: "NASA", type: "官方", verdict: "可信" },
          { name: "Nature Sustainability", type: "学者/机构", verdict: "可信" },
        ],
        better: "原始研究来自 NASA / Nature Sustainability",
        trace: "https://www.nature.com/...",
      },
    }));
    expect(p.card_id).toBe("sift_craap");
    expect(p.status).toBe("completed");
    const siftStep = p.steps!.find((s) => s.title === sift.steps[0]!.title)!;
    // the Stop textarea answer is paired with its label
    expect(siftStep.answers.find((a) => a.label.startsWith("Stop"))!.value).toBe("证明中国让地球更可持续");
    // the repeatable_group is an array of {item-label: value}
    const sources = siftStep.answers.find((a) => a.label.startsWith("Investigate"))!.value as Array<Record<string, unknown>>;
    expect(sources[0]).toEqual({ "来源": "NASA", "类型": "官方", "可信？": "可信" });
  });

  it("omits unfilled fields (no empty answers)", () => {
    const p = serializeCardForRefeed(sift, inst("completed", { sift: { stop: "x" } }));
    const siftStep = p.steps!.find((s) => s.title === sift.steps[0]!.title)!;
    expect(siftStep.answers.every((a) => a.value !== undefined && a.value !== "")).toBe(true);
  });
});
```

- [ ] **Step 2: Run to verify it fails** — `pnpm --filter @mind-imprint/contracts exec vitest run test/refeed.test.ts` → FAIL.

- [ ] **Step 3: Implement** — `packages/contracts/src/refeed.ts`:

```ts
import type { CardSpec } from "./cardSpec";
import type { CardInstance } from "./envelope";

export interface RefeedAnswer { label: string; value: unknown }
export interface RefeedStep { title: string; answers: RefeedAnswer[] }
export interface RefeedPayload {
  card_id: string;
  card_name: string;
  status: "completed" | "skipped";
  steps?: RefeedStep[];
}

function isEmpty(v: unknown): boolean {
  return v === undefined || v === null || v === "" || (Array.isArray(v) && v.length === 0);
}

export function serializeCardForRefeed(spec: CardSpec, instance: CardInstance): RefeedPayload {
  const card_name = spec.name;
  if (instance.status === "skipped") {
    return { card_id: spec.id, card_name, status: "skipped" };
  }
  const steps: RefeedStep[] = [];
  for (const step of spec.steps) {
    const stepValues = (instance.field_values[step.key] ?? {}) as Record<string, unknown>;
    const answers: RefeedAnswer[] = [];
    for (const field of step.fields) {
      const raw = stepValues[field.key];
      if (isEmpty(raw)) continue;
      if (field.type === "repeatable_group") {
        // remap each row's keys to the item-field labels
        const rows = (raw as Array<Record<string, unknown>>).map((row) => {
          const out: Record<string, unknown> = {};
          for (const item of field.item_fields) {
            if (!isEmpty(row[item.key])) out[item.label] = row[item.key];
          }
          return out;
        }).filter((r) => Object.keys(r).length > 0);
        if (rows.length > 0) answers.push({ label: field.label, value: rows });
      } else {
        answers.push({ label: field.label, value: raw });
      }
    }
    if (answers.length > 0) steps.push({ title: step.title, answers });
  }
  return { card_id: spec.id, card_name, status: "completed", steps };
}
```

(Note: `field.item_fields`/`field.label` come from the `repeatable_group` primitive in `primitives.ts`; read that file to confirm the exact property names before implementing.)

- [ ] **Step 4: Run test + typecheck** — test PASS; `pnpm --filter @mind-imprint/contracts typecheck` clean.
- [ ] **Step 5: Commit** — `git add -A && git commit -m "feat(contracts): schema-derived re-feed serializer"`

---

### Task 3: LLM tool types + OpenAI adapter tool support

**Files:**
- Modify: `apps/web/src/llm/types.ts`
- Modify: `apps/web/src/llm/openaiAdapter.ts`
- Test: `apps/web/src/llm/openaiAdapter.test.ts` (extend)

**Interfaces — Produces (types.ts):**
```ts
export type ChatRole = "system" | "user" | "assistant" | "tool";
export interface ChatTool { name: string; description: string; parameters: Record<string, unknown> }
export interface ToolCall { id: string; name: string; args: Record<string, unknown> }
export interface ChatMessage { role: ChatRole; content: string; toolCalls?: ToolCall[]; toolCallId?: string }
export interface ChatRequest { messages: ChatMessage[]; tools?: ChatTool[]; maxTokens?: number; temperature?: number }
export type StopReason = "stop" | "tool_call" | "length" | "other";
export interface ChatResult { text: string; toolCalls?: ToolCall[]; stopReason?: StopReason; usage?: ChatUsage; raw?: unknown }
```
(Keep existing `LlmFormat`/`LlmConfig`/`ChatUsage`. `ChatRole` gains `"tool"`; `ChatMessage` gains optional `toolCalls`/`toolCallId`; `ChatResult` gains optional `toolCalls`/`stopReason`.)

- [ ] **Step 1: Write the failing tests** — extend `openaiAdapter.test.ts`. Read the existing file first to match its mock-fetch idiom. Add cases:
  - Request with `tools` includes `tools:[{type:"function",function:{name,description,parameters}}]` and `tool_choice:"auto"` in the POST body.
  - A response whose `choices[0].message.tool_calls` is `[{id:"c1",type:"function",function:{name:"summon_card",arguments:"{\"card_id\":\"sift_craap\",\"reason\":\"r\",\"nudge_text\":\"n\"}"}}]` parses to `result.toolCalls = [{id:"c1",name:"summon_card",args:{card_id:"sift_craap",reason:"r",nudge_text:"n"}}]`, `result.stopReason==="tool_call"`, `result.text===""`.
  - A `ChatMessage` with `role:"tool"` + `toolCallId` serializes to `{role:"tool",tool_call_id,content}`; a `role:"assistant"` message with `toolCalls` serializes to `{role:"assistant",content:null|"",tool_calls:[{id,type:"function",function:{name,arguments:JSON.stringify(args)}}]}`.
  - `finish_reason:"stop"` → `stopReason:"stop"`; `"length"` → `"length"`.

  Write each as a concrete test with the exact request/response JSON and assertions, following the existing test style (`vi.stubGlobal("fetch", ...)`, parse the captured request body).

- [ ] **Step 2: Run to verify they fail** — `pnpm --filter web exec vitest run src/llm/openaiAdapter.test.ts` → FAIL.

- [ ] **Step 3: Implement** — extend `openaiAdapter.ts`:
  - When building the request body: if `req.tools?.length`, add `tools: req.tools.map(t => ({ type: "function", function: { name: t.name, description: t.description, parameters: t.parameters } }))` and `tool_choice: "auto"`.
  - When serializing `req.messages`: map each `ChatMessage` to OpenAI shape — `role:"tool"` → `{ role:"tool", tool_call_id: m.toolCallId, content: m.content }`; assistant with `toolCalls` → `{ role:"assistant", content: m.content || null, tool_calls: m.toolCalls.map(c => ({ id:c.id, type:"function", function:{ name:c.name, arguments: JSON.stringify(c.args) } })) }`; others unchanged.
  - When parsing the response: read `choices[0].message.tool_calls`; if present, set `toolCalls = tc.map(c => ({ id:c.id, name:c.function.name, args: JSON.parse(c.function.arguments) }))`. Map `finish_reason`: `"tool_calls"→"tool_call"`, `"stop"→"stop"`, `"length"→"length"`, else `"other"`. `text` = `choices[0].message.content ?? ""`.
  - Keep key-safety (apiKey only in header; never echo into result beyond existing `raw` policy).

- [ ] **Step 4: Run test + typecheck** — `pnpm --filter web exec vitest run src/llm/openaiAdapter.test.ts` PASS; also run the full `src/llm` suite (no regression); `pnpm --filter web typecheck` clean.
- [ ] **Step 5: Commit** — `git commit -am "feat(llm): tool-use types + OpenAI adapter tool support"`

---

### Task 4: Anthropic adapter tool support

**Files:**
- Modify: `apps/web/src/llm/anthropicAdapter.ts`
- Test: `apps/web/src/llm/anthropicAdapter.test.ts` (extend)

**Interfaces:** consumes the Task 3 types; same `ChatRequest`/`ChatResult` contract, Anthropic wire format.

- [ ] **Step 1: Write the failing tests** — extend `anthropicAdapter.test.ts` (read it first). Cases:
  - Request with `tools` includes `tools:[{name,description,input_schema}]` (note: `input_schema`, not `parameters`).
  - A response `content:[{type:"tool_use",id:"tu1",name:"summon_card",input:{card_id:"sift_craap",reason:"r",nudge_text:"n"}}]` with `stop_reason:"tool_use"` parses to `toolCalls=[{id:"tu1",name:"summon_card",args:{...}}]`, `stopReason:"tool_call"`, `text` = the joined text blocks (or "").
  - A `ChatMessage` `role:"tool"` + `toolCallId` serializes into a `{role:"user", content:[{type:"tool_result", tool_use_id, content}]}` message; an assistant message with `toolCalls` serializes to `{role:"assistant", content:[{type:"tool_use",id,name,input}]}` (plus any text block).
  - System messages still lifted to top-level `system` (existing S2 behavior preserved).

- [ ] **Step 2: Run to verify they fail** — `pnpm --filter web exec vitest run src/llm/anthropicAdapter.test.ts` → FAIL.

- [ ] **Step 3: Implement** — extend `anthropicAdapter.ts`:
  - Request: if `req.tools?.length`, add `tools: req.tools.map(t => ({ name:t.name, description:t.description, input_schema:t.parameters }))`.
  - Message serialization: keep system-lift. For the remaining user/assistant/tool messages, build `content` blocks: `role:"tool"` → emit a `{role:"user", content:[{type:"tool_result", tool_use_id:m.toolCallId, content:m.content}]}`; assistant with `toolCalls` → `{role:"assistant", content:[...(m.content?[{type:"text",text:m.content}]:[]), ...m.toolCalls.map(c=>({type:"tool_use",id:c.id,name:c.name,input:c.args}))]}`; plain user/assistant → `{role, content:m.content}` (string form is fine).
  - Response parse: iterate `content[]`; collect `type:"text"` into `text` (join with "\n"); collect `type:"tool_use"` into `toolCalls=[{id,name,args:block.input}]`. Map `stop_reason:"tool_use"→"tool_call"`, `"end_turn"→"stop"`, `"max_tokens"→"length"`, else `"other"`.

- [ ] **Step 4: Run test + typecheck** — adapter test PASS; full `src/llm` suite green; `pnpm --filter web typecheck` clean.
- [ ] **Step 5: Commit** — `git commit -am "feat(llm): Anthropic adapter tool support"`

---

### Task 5: Decision layer — prompt, catalog, demoCatalog, summon_card tool

**Files:**
- Create: `apps/web/src/agent/prompt.ts`
- Test: `apps/web/src/agent/prompt.test.ts`

**Interfaces:**
- Consumes: `Catalog`/`CatalogEntry` from contracts; `ChatTool` from `../llm/types`.
- Produces:
  - `buildCatalogText(catalog: Catalog): string`
  - `buildSystemPrompt(catalog: Catalog): string`
  - `demoCatalog(full: Catalog): Catalog`  (drops `sift`, `craap`, `steelman`)
  - `summonCardTool(catalog: Catalog): ChatTool`
  - `DEMO_TWINS = ["sift", "craap", "steelman"]` (exported const)

- [ ] **Step 1: Write the failing test** — `apps/web/src/agent/prompt.test.ts`:

```ts
import { describe, it, expect } from "vitest";
import { deriveCatalog, CARD_REGISTRY } from "@mind-imprint/contracts";
import { buildCatalogText, buildSystemPrompt, demoCatalog, summonCardTool } from "./prompt";

const full = deriveCatalog(CARD_REGISTRY);

describe("decision layer", () => {
  it("catalog text groups by category and lists each card's trigger_condition", () => {
    const txt = buildCatalogText(full);
    expect(txt).toContain("信息素养");
    expect(txt).toContain("sift_craap");
    // each line carries the trigger_condition
    expect(txt).toMatch(/sift_craap.*：.+/);
  });
  it("system prompt embeds the catalog and the restraint rules", () => {
    const p = buildSystemPrompt(full);
    expect(p).toContain("不替他定论");
    expect(p).toContain("summon_card");
    expect(p).toContain("sift_craap");
  });
  it("demoCatalog drops the 3 library twins but keeps the demo cards", () => {
    const d = demoCatalog(full).map((c) => c.id);
    expect(d).not.toContain("sift");
    expect(d).not.toContain("craap");
    expect(d).not.toContain("steelman");
    expect(d).toContain("sift_craap");
    expect(d).toContain("concession");
    expect(d.length).toBe(full.length - 3);
  });
  it("summon_card tool exposes card_id enum = catalog ids and the 3 required args", () => {
    const tool = summonCardTool(demoCatalog(full));
    expect(tool.name).toBe("summon_card");
    const params = tool.parameters as any;
    expect(params.properties.card_id.enum).toEqual(demoCatalog(full).map((c) => c.id));
    expect(params.required).toEqual(["card_id", "reason", "nudge_text"]);
  });
});
```

- [ ] **Step 2: Run to verify it fails** — `pnpm --filter web exec vitest run src/agent/prompt.test.ts` → FAIL.

- [ ] **Step 3: Implement** — `apps/web/src/agent/prompt.ts`:
  - `buildCatalogText`: group entries by `category`; for each category emit `【{category}】\n` then for each card `· {id} — {name} [{disclosure_tier}·{priority}]：{trigger_condition}\n`.
  - `buildSystemPrompt`: the exact restraint/mentor prompt from the spec §5 (the reviewed copy), ending with the catalog text in place of `{{catalog}}`. Copy the prompt VERBATIM from `docs/superpowers/specs/2026-06-21-slice3-decision-summon-workspace-design.md` §5 (the fenced block) — it is the binding wording.
  - `demoCatalog(full)`: `full.filter(c => !DEMO_TWINS.includes(c.id))`.
  - `summonCardTool(catalog)`: returns `{ name:"summon_card", description:"…当且仅当此刻命中某卡适用情形时提议一张卡…", parameters:{ type:"object", properties:{ card_id:{ type:"string", enum: catalog.map(c=>c.id) }, reason:{type:"string"}, nudge_text:{type:"string"} }, required:["card_id","reason","nudge_text"] } }`.

- [ ] **Step 4: Run test + typecheck** — test PASS; `pnpm --filter web typecheck` clean.
- [ ] **Step 5: Commit** — `git commit -am "feat(agent): decision-layer prompt + 33-card catalog + summon_card tool"`

---

### Task 6: store→LLM message mapping (pure)

**Files:**
- Create: `apps/web/src/agent/messageMapping.ts`
- Test: `apps/web/src/agent/messageMapping.test.ts`

**Interfaces:**
- Consumes: `Message`, `CardInstance`, `CardSpec`, `serializeCardForRefeed`, `SummonCardCall` from contracts; `ChatMessage` from `../llm/types`.
- Produces:
  ```ts
  buildLlmMessages(opts: {
    systemPrompt: string;
    messages: Message[];                          // task-scoped, insertion order
    cardById: (id: string) => CardInstance | undefined;
    specById: (cardId: string) => CardSpec | undefined;
  }): ChatMessage[]
  ```

Mapping rules (the invariant): prepend `{role:"system",content:systemPrompt}`. For each store `Message` in order:
- `role:"user"` → `{role:"user",content}`.
- `role:"assistant"` with a parseable `SummonCardCall` in `tool_call` → `{role:"assistant", content: m.content, toolCalls:[{id:call.id,name:"summon_card",args:call.args}]}`; THEN look up `cardById(call.card_instance_id)`: if its `status` is `completed`/`skipped`, append `{role:"tool", toolCallId:call.id, content: JSON.stringify(serializeCardForRefeed(specById(ci.card_id)!, ci))}`. If `proposed`/`active` (unresolved), append nothing (such a proposal is always the tail).
- plain `role:"assistant"` (no tool_call) → `{role:"assistant",content}`.
- `role:"system"` store messages (rare) → `{role:"system",content}`.

- [ ] **Step 1: Write the failing test** — `apps/web/src/agent/messageMapping.test.ts`:

```ts
import { describe, it, expect } from "vitest";
import { CARD_REGISTRY } from "@mind-imprint/contracts";
import type { Message, CardInstance } from "@mind-imprint/contracts";
import { buildLlmMessages } from "./messageMapping";

const call = { id: "c1", name: "summon_card", args: { card_id: "sift_craap", reason: "r", nudge_text: "n" }, card_instance_id: "ci_1" };
function msg(p: Partial<Message>): Message {
  return { id: "m", task_id: "t", role: "user", content: "", tool_call: null, created_at: "2026-06-21T10:00:00.000Z", ...p };
}
function ci(status: CardInstance["status"]): CardInstance {
  return { id: "ci_1", card_id: "sift_craap", task_id: "t", parent_node_id: null, status, field_values: { sift: { stop: "x" } }, event_trace: [], rubric_tags: [], created_at: "2026-06-21T10:00:00.000Z", completed_at: null };
}
const specById = (id: string) => CARD_REGISTRY[id];

it("a completed proposal yields assistant tool_call + paired tool result", () => {
  const messages = [
    msg({ role: "user", content: "我看到一篇文章" }),
    msg({ role: "assistant", content: "先核来源？", tool_call: call }),
  ];
  const out = buildLlmMessages({ systemPrompt: "SYS", messages, cardById: () => ci("completed"), specById });
  expect(out[0]).toEqual({ role: "system", content: "SYS" });
  expect(out[2].role).toBe("assistant");
  expect(out[2].toolCalls![0].id).toBe("c1");
  expect(out[3].role).toBe("tool");
  expect(out[3].toolCallId).toBe("c1");
  expect(JSON.parse(out[3].content).status).toBe("completed");
});

it("an unresolved proposal is the tail (no tool result appended)", () => {
  const messages = [msg({ role: "assistant", content: "先核来源？", tool_call: call })];
  const out = buildLlmMessages({ systemPrompt: "SYS", messages, cardById: () => ci("proposed"), specById });
  expect(out[out.length - 1].role).toBe("assistant");
  expect(out.some((m) => m.role === "tool")).toBe(false);
});
```

- [ ] **Step 2: Run to verify it fails** — FAIL.
- [ ] **Step 3: Implement** — `messageMapping.ts` per the rules above. Parse `m.tool_call` with `SummonCardCall.safeParse`; only treat as a proposal when it succeeds.
- [ ] **Step 4: Run test + typecheck** — PASS; `pnpm --filter web typecheck` clean.
- [ ] **Step 5: Commit** — `git commit -am "feat(agent): store→LLM message mapping with tool-result pairing"`

---

### Task 7: `createConversation` orchestrator + `useConversation`

**Files:**
- Create: `apps/web/src/agent/createConversation.ts`
- Create: `apps/web/src/agent/useConversation.ts`
- Create: `apps/web/src/agent/index.ts` (barrel)
- Test: `apps/web/src/agent/createConversation.test.ts`

**Interfaces:**
- Consumes: `Store` (B2.5), `chat` + `LlmConfig` (llm), `CARD_REGISTRY`/`deriveCatalog`/`SummonCardCall`/`CardInstance` (contracts), `buildSystemPrompt`/`demoCatalog`/`summonCardTool` (Task 5), `buildLlmMessages` (Task 6), S1 `envelopeReducer` for activation.
- Produces:
  ```ts
  interface ConversationDeps { store: Store; chat: ChatFn; config: Partial<LlmConfig>; registry: Record<string,CardSpec>; catalog: Catalog; now?: ()=>string; genId?: ()=>string }
  type ConvPhase = "idle" | "awaiting_llm" | "proposal_pending" | "card_active" | "error";
  interface ConvState { taskId: string; phase: ConvPhase; pendingCardId?: string; error?: string }
  interface Conversation { getSnapshot(): ConvState; subscribe(l:()=>void):()=>void;
    send(text: string): Promise<void>;
    openCard(cardInstanceId: string): void;
    submitCard(cardInstanceId: string, finalInstance: CardInstance): Promise<void>;
    skipCard(cardInstanceId: string): Promise<void>; }
  function createConversation(deps: ConversationDeps): Conversation
  ```
  `useConversation(conv): ConvState` via `useSyncExternalStore`.

Behavior (drive the store; call the LLM via `buildLlmMessages` + `chat`):
- `send(text)`: append user message; set `phase:"awaiting_llm"`; build LLM messages (system from `buildSystemPrompt(catalog)`, tool from `summonCardTool(catalog)`); `chat(...)`. If `toolCalls` has a `summon_card` (take the FIRST; warn + ignore extras): validate `card_id` against registry (unknown → drop, treat as text); create a proposed `CardInstance` (envelope) via `genId`/`now`; append an assistant `Message` with `tool_call = SummonCardCall{ id: toolCall.id, name, args, card_instance_id }` and `content = args.nudge_text`; set `phase:"proposal_pending"`, `pendingCardId`. Else append assistant text message; `phase:"idle"`. On `LlmError`: `phase:"error"`, store `error`.
- `openCard(id)`: activate the instance (envelope reducer `activate`) via `store.putCard`; `phase:"card_active"`.
- `submitCard(id, finalInstance)`: `store.putCard(finalInstance)` (status completed); then re-feed: build LLM messages (now the proposal's card resolves to a tool result) + `chat()`; append the AI's follow-up; `phase:"idle"`.
- `skipCard(id)`: mark instance skipped (`store.putCard`); re-feed (skipped payload) + `chat()`; append follow-up; `phase:"idle"`.
- `phase:"awaiting_llm"` while a `chat()` is in flight (UI disables composer).

- [ ] **Step 1: Write the failing test** — `createConversation.test.ts` against a **fake `chat`** (scripted) + a memory store:

```ts
// fake chat: first call returns a summon_card toolCall; second call returns text.
// Assert across the flow:
//  - after send: store has user msg + assistant proposal msg (tool_call is a valid SummonCardCall) + a proposed CardInstance; phase "proposal_pending".
//  - openCard → instance active, phase "card_active".
//  - submitCard(completed instance) → instance completed; the SECOND chat() received messages whose last tool message content JSON has status "completed" and the right card_instance pairing; an assistant follow-up message is appended; phase "idle".
//  - a second scripted run: skip path → instance skipped, tool result status "skipped".
//  - multiple toolCalls in one result → only the first summon_card is used (assert one CardInstance created).
//  - unknown card_id → no CardInstance, treated as text.
//  - chat throws LlmError → phase "error", error message set, nothing thrown.
```
Write these as concrete `it(...)` blocks with a `makeFakeChat(script)` helper capturing the `req.messages` it receives, a `makeMemoryStorage()`-backed `createStore`, and deterministic `now`/`genId`.

- [ ] **Step 2: Run to verify it fails** — FAIL.
- [ ] **Step 3: Implement** — `createConversation.ts` (reactive controller mirroring `createStore`: immutable `ConvState`, `getSnapshot`/`subscribe`/notify) + `useConversation.ts` + `index.ts` barrel. Reuse `envelopeReducer` for `activate`. Validate `tool_call` via `SummonCardCall`; validate `card_id ∈ registry`.
- [ ] **Step 4: Run test + typecheck** — PASS; `pnpm --filter web typecheck` clean.
- [ ] **Step 5: Commit** — `git commit -am "feat(agent): createConversation orchestrator + useConversation"`

---

### Task 8: Chat view-models + `ChatLog`

**Files:**
- Create: `apps/web/src/workspace/viewModel.ts`
- Create: `apps/web/src/workspace/ChatLog.tsx`
- Test: `apps/web/src/workspace/viewModel.test.ts`, `apps/web/src/workspace/ChatLog.test.tsx`

**Interfaces:**
- Produces: `messagesToItems(messages: Message[], cardById, specById): ChatItem[]` where `ChatItem` is a discriminated union `{kind:"student",text,link?} | {kind:"ai_text",text} | {kind:"proposal", cardInstanceId, category, cardName, nudge, status}` (status from the linked `CardInstance`). `ChatLog` renders items per the binding HTML (student right bubble + link chip; AI avatar+bubble; inline proposal with 建议工具卡/category/cardName/nudge and the three states 打开卡/暂不 · 已完成 · 已跳过).

- [ ] **Step 1: Write failing tests** — `viewModel.test.ts`: a user message with a URL → `{kind:"student", link}`; an assistant proposal message + a `proposed` CardInstance → `{kind:"proposal", status:"proposed", cardName, nudge}`; same with `completed`/`skipped`. `ChatLog.test.tsx` (RTL): renders a student bubble, an AI bubble, and a proposal showing 打开卡 + 暂不，先继续 when proposed; shows 已完成 · 已钉到过程树 when completed.
- [ ] **Step 2: Run to verify they fail** — FAIL.
- [ ] **Step 3: Implement** — `viewModel.ts` (parse `tool_call` via `SummonCardCall`; derive category/name from `specById`); `ChatLog.tsx` lifting structure/copy/styling verbatim from `docs/design/思维印记_工作区.dc.html` (the `showWorkspace` chat block: `m.isStudent`/`m.isAIText`/`m.isProposal` with `isProposed`/`isCompleted`/`isSkipped`). Use `mk-*` tokens; wire `onOpen`/`onSkip` callbacks (props). No card-specific branches.
- [ ] **Step 4: Run tests + typecheck** — PASS; `pnpm --filter web typecheck` clean.
- [ ] **Step 5: Commit** — `git commit -am "feat(workspace): chat view-models + ChatLog"`

---

### Task 9: `WorkspaceView` shell (composer + tree skeleton + bottom-sheet host)

**Files:**
- Create: `apps/web/src/workspace/Composer.tsx`, `apps/web/src/workspace/TreePanel.tsx`, `apps/web/src/workspace/CardSheetHost.tsx`, `apps/web/src/workspace/WorkspaceView.tsx`
- Create: `apps/web/src/workspace/index.ts`
- Test: `apps/web/src/workspace/WorkspaceView.test.tsx`

**Interfaces:**
- Consumes: `useStore`/`Store` (B2.5), `useConversation`/`Conversation` (Task 7), `messagesToItems`/`ChatLog` (Task 8), S1 `<CardRenderer>` + `envelopeReducer`, `CARD_REGISTRY`.
- Produces: `WorkspaceView({ store, conversation, taskId })` rendering the full `showWorkspace` surface; `Composer` (footerComposer); `TreePanel` (treeOpen/treeClosed skeleton + empty state «边做边长 · 随评估归并枝节», NO derived nodes — that's S4); `CardSheetHost` (`hasActiveCard`) mounting the S1 `<CardRenderer>` active state and wiring submit→`conversation.submitCard` / close-as-skip semantics.

- [ ] **Step 1: Write failing test** — `WorkspaceView.test.tsx` (RTL + a fake conversation/store): breadcrumb shows the task title + 返回所有任务; composer textarea + send button present (disabled when phase `awaiting_llm`); tree panel shows the 只读 empty-state copy and toggles open/closed; when a card is active the bottom sheet mounts the renderer (a known field label appears). Keep it behavioral.
- [ ] **Step 2: Run to verify it fails** — FAIL.
- [ ] **Step 3: Implement** — the four components, pixel-faithful to the HTML's `showWorkspace`/`footerComposer`/`treeOpen`·`treeClosed`/`hasActiveCard` blocks (lift verbatim). `CardSheetHost` drives the S1 `CardRenderer` with an envelope from the store and calls `conversation.submitCard(id, finalInstance)` on submit. `TreePanel` body is the empty-state only.
- [ ] **Step 4: Run test + typecheck** — PASS; `pnpm --filter web typecheck` clean.
- [ ] **Step 5: Commit** — `git commit -am "feat(workspace): WorkspaceView shell + bottom-sheet host"`

---

### Task 10: Dev entry — workspace tab + seeded Phoebe task

**Files:**
- Create: `apps/web/src/dev/WorkspaceDev.tsx`
- Modify: `apps/web/src/dev/DevApp.tsx` (+ a 工作区 tab)
- Test: `apps/web/src/dev/DevApp.test.tsx` (extend)

**Interfaces:** assembles a real-localStorage `createStore` + real `chat` (config from `loadConfig()`) + `createConversation({ catalog: demoCatalog(deriveCatalog(CARD_REGISTRY)), ... })`, seeds the Phoebe task (title 中国是否让地球变得更可持续？ + the seed message), and renders `<WorkspaceView/>`.

- [ ] **Step 1: Write failing test** — extend `DevApp.test.tsx`: clicking 工作区 shows the workspace breadcrumb / composer. (LLM calls are gated by config — the test only asserts the workspace shell renders, not a live call.)
- [ ] **Step 2: Run to verify it fails** — FAIL.
- [ ] **Step 3: Implement** — `WorkspaceDev.tsx` (module-level store + conversation, seeded once) + add the `工作区` tab to `DevApp` (follow the existing tab pattern). Use `demoCatalog` so the demo biases toward `sift_craap`/`concession`.
- [ ] **Step 4: Run test + typecheck** — PASS; `pnpm --filter web typecheck` clean.
- [ ] **Step 5: Commit** — `git commit -am "feat(dev): workspace tab + seeded Phoebe task"`

---

### Final verification

```bash
pnpm -r typecheck
pnpm -r test
```
Both green. Optional manual smoke (with a real key in `apps/web/.env.local`): open the 工作区 tab, paste the Phoebe seed, confirm the AI proposes a source-check card, open+fill it, and the AI follows up on the filled content.

## Self-Review

**Spec coverage:** SummonCardCall (T1) ✓; re-feed serializer (T2) ✓; LLM tool types + both adapters (T3,T4) ✓; decision-layer prompt + 33-card catalog + demoCatalog + summon_card tool (T5) ✓; store→LLM mapping with tool-result pairing (T6) ✓; orchestrator state machine + re-feed loop + skip + first-tool-only + unknown-id + error (T7) ✓; chat view-models + inline proposal three-states (T8) ✓; workspace shell + tree skeleton + bottom-sheet mounting CardRenderer (T9) ✓; dev entry + demo biasing (T10) ✓. Out of scope held: tree body (S4), eval (S5), shell/auth (S6), rich interactions (3c).

**Placeholder scan:** The two UI tasks (T8,T9) intentionally instruct lifting markup verbatim from the binding HTML rather than re-printing ~200 lines of styled JSX here — this matches how S1's UI was planned and the project's "HTML is the source of truth" rule; the precise behavior + tests + interfaces are fully specified. No "TBD"/"add error handling"-style gaps in any logic step.

**Type consistency:** `ChatTool`/`ToolCall`/`ChatMessage` (T3) are reused unchanged by T4/T5/T6/T7. `SummonCardCall` (T1) is the single shape used by T6/T7/T8. `serializeCardForRefeed` (T2) signature matches its T6 call. `Conversation`/`ConvState`/`ConvPhase` (T7) are consumed by T8/T9/T10. `demoCatalog`/`summonCardTool` (T5) feed T7/T10.
