# Slice 2 — Provider-Agnostic LLM Client + Config (B2) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A frontend, provider-agnostic LLM client — given `{format, baseUrl, model, apiKey}`, send chat messages to an OpenAI-format or Anthropic-format endpoint and get a reply; key is BYO, stored in the browser, never committed.

**Architecture:** Pure frontend, no backend, non-streaming. A `apps/web/src/llm/` module with two per-format adapters behind a unified `chat()`, config persisted to localStorage (with injectable env fallback for dev pre-fill), and a minimal `SettingsPanel` mounted in a 2-tab `DevApp` alongside the existing card harness.

**Tech Stack:** TypeScript, React 18, Vite, Vitest + @testing-library/react (jsdom), browser `fetch`. No new dependencies.

## Global Constraints

- **All LLM access goes through `apps/web/src/llm/`.** No backend, non-streaming, BYO-key. (Roadmap §0.1 — this intentionally overrides PRD §16.)
- **`LlmConfig` shape (exact):** `{ format: "openai" | "anthropic"; baseUrl: string; model: string; apiKey: string; evalModel?: string }`. `baseUrl` includes `/v1`.
- **OpenAI adapter:** `POST {baseUrl}/chat/completions`, header `Authorization: Bearer {apiKey}`, body `{ model, messages, max_tokens }`; parse `choices[0].message.content`.
- **Anthropic adapter:** `POST {baseUrl}/messages`, headers `x-api-key: {apiKey}` + `anthropic-version: 2023-06-01` + `anthropic-dangerous-direct-browser-access: true`; lift `role:"system"` messages into a top-level `system` string; parse the first `content[]` block of `type:"text"`.
- **The apiKey MUST NEVER appear** in `console`, in any thrown error message, or in displayed output. (A test asserts this.)
- **`ChatRequest.tools` is reserved for Slice 3** (`summon_card`) — do NOT implement tool-calling here.
- **Default `max_tokens` = 1024** when `req.maxTokens` is absent.
- **Key never in git:** `apps/web/.env.example` is committed; `apps/web/.env.local` is git-ignored (covered by existing `.env.*` + `!.env.example`).
- **Verification gate:** `pnpm -r typecheck` + `pnpm -r test` must be green. (`vite build`/`vitest` do not typecheck — `tsc --noEmit` does.)
- Tests mock `fetch` via `vi.stubGlobal`; one gated live smoke test skips when no key is configured.

---

### Task 1: Types, LlmError, env template

**Files:**
- Create: `apps/web/src/llm/types.ts`
- Create: `apps/web/src/llm/LlmError.ts`
- Create: `apps/web/.env.example`
- Test: `apps/web/src/llm/LlmError.test.ts`

**Interfaces:**
- Consumes: nothing.
- Produces: `LlmFormat`, `LlmConfig`, `ChatRole`, `ChatMessage`, `ChatRequest`, `ChatUsage`, `ChatResult` (types); `LlmError` class with `{ status?: number; provider?: LlmFormat; body?: unknown }`.

- [ ] **Step 1: Write the types**

`apps/web/src/llm/types.ts`:
```ts
export type LlmFormat = "openai" | "anthropic";

export interface LlmConfig {
  format: LlmFormat;
  baseUrl: string; // API root incl. /v1, e.g. https://api.openai.com/v1 | https://api.anthropic.com/v1
  model: string;
  apiKey: string;
  evalModel?: string; // optional; consumed by B6/S5, only stored here
}

export type ChatRole = "system" | "user" | "assistant";
export interface ChatMessage { role: ChatRole; content: string }

export interface ChatRequest {
  messages: ChatMessage[];
  maxTokens?: number;
  temperature?: number;
  // tools?: reserved for Slice 3 (summon_card) — not implemented in Slice 2.
}

export interface ChatUsage { inputTokens?: number; outputTokens?: number }
export interface ChatResult { text: string; usage?: ChatUsage; raw?: unknown }
```

- [ ] **Step 2: Write the failing test**

`apps/web/src/llm/LlmError.test.ts`:
```ts
import { describe, it, expect } from "vitest";
import { LlmError } from "./LlmError";

describe("LlmError", () => {
  it("is an Error named LlmError carrying status/provider/body", () => {
    const e = new LlmError("boom", { status: 401, provider: "openai", body: { x: 1 } });
    expect(e).toBeInstanceOf(Error);
    expect(e.name).toBe("LlmError");
    expect(e.message).toBe("boom");
    expect(e.status).toBe(401);
    expect(e.provider).toBe("openai");
    expect(e.body).toEqual({ x: 1 });
  });
  it("defaults optional fields to undefined", () => {
    const e = new LlmError("x");
    expect(e.status).toBeUndefined();
    expect(e.provider).toBeUndefined();
  });
});
```

- [ ] **Step 3: Run test to verify it fails**

Run: `pnpm --filter web test -- LlmError`
Expected: FAIL — cannot resolve `./LlmError`.

- [ ] **Step 4: Write LlmError + the env template**

`apps/web/src/llm/LlmError.ts`:
```ts
import type { LlmFormat } from "./types";

export interface LlmErrorOptions { status?: number; provider?: LlmFormat; body?: unknown }

export class LlmError extends Error {
  status?: number;
  provider?: LlmFormat;
  body?: unknown;
  constructor(message: string, opts: LlmErrorOptions = {}) {
    super(message);
    this.name = "LlmError";
    this.status = opts.status;
    this.provider = opts.provider;
    this.body = opts.body;
  }
}
```

`apps/web/.env.example`:
```
# 思维印记 LLM 配置 — 复制为 apps/web/.env.local（git-ignored）并填写。
# 注意：VITE_* 变量会被打进前端构建包，仅用于本地 BYO-key 开发演示。
VITE_LLM_FORMAT=openai
VITE_LLM_BASE_URL=https://api.openai.com/v1
VITE_LLM_MODEL=gpt-4o-mini
VITE_LLM_API_KEY=
VITE_LLM_EVAL_MODEL=
```

- [ ] **Step 5: Run test to verify it passes**

Run: `pnpm --filter web test -- LlmError`
Expected: PASS (2 tests).

- [ ] **Step 6: Commit**

```bash
git add apps/web/src/llm/types.ts apps/web/src/llm/LlmError.ts apps/web/src/llm/LlmError.test.ts apps/web/.env.example
git commit -m "feat(llm): config/message types, LlmError, env template"
```

---

### Task 2: OpenAI adapter

**Files:**
- Create: `apps/web/src/llm/openaiAdapter.ts`
- Test: `apps/web/src/llm/openaiAdapter.test.ts`

**Interfaces:**
- Consumes: `LlmConfig`, `ChatRequest`, `ChatResult` (types); `LlmError`.
- Produces: `openaiAdapter(config: LlmConfig, req: ChatRequest): Promise<ChatResult>`.

- [ ] **Step 1: Write the failing test**

`apps/web/src/llm/openaiAdapter.test.ts`:
```ts
import { describe, it, expect, vi, afterEach } from "vitest";
import { openaiAdapter } from "./openaiAdapter";
import type { LlmConfig } from "./types";

const cfg: LlmConfig = { format: "openai", baseUrl: "https://api.openai.com/v1", model: "gpt-4o-mini", apiKey: "sk-test-123" };

function mockFetch(status: number, body: unknown) {
  const fn = vi.fn().mockResolvedValue({ ok: status >= 200 && status < 300, status, json: () => Promise.resolve(body) });
  vi.stubGlobal("fetch", fn);
  return fn;
}
afterEach(() => vi.unstubAllGlobals());

describe("openaiAdapter", () => {
  it("POSTs to /chat/completions with bearer auth and parses the reply", async () => {
    const fetchMock = mockFetch(200, { choices: [{ message: { content: "OK" } }], usage: { prompt_tokens: 5, completion_tokens: 2 } });
    const result = await openaiAdapter(cfg, { messages: [{ role: "user", content: "hi" }] });
    const [url, init] = fetchMock.mock.calls[0]!;
    expect(url).toBe("https://api.openai.com/v1/chat/completions");
    expect((init.headers as Record<string, string>)["Authorization"]).toBe("Bearer sk-test-123");
    const sent = JSON.parse(init.body as string);
    expect(sent).toMatchObject({ model: "gpt-4o-mini", messages: [{ role: "user", content: "hi" }], max_tokens: 1024 });
    expect(result.text).toBe("OK");
    expect(result.usage).toEqual({ inputTokens: 5, outputTokens: 2 });
  });
  it("throws LlmError with status + provider message on non-2xx", async () => {
    mockFetch(401, { error: { message: "invalid key" } });
    await expect(openaiAdapter(cfg, { messages: [] })).rejects.toMatchObject({ name: "LlmError", status: 401, message: "invalid key" });
  });
  it("never leaks the apiKey in the error message", async () => {
    mockFetch(500, { error: { message: "boom" } });
    const err = await openaiAdapter(cfg, { messages: [] }).catch((e) => e as Error);
    expect(err.message).not.toContain("sk-test-123");
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm --filter web test -- openaiAdapter`
Expected: FAIL — cannot resolve `./openaiAdapter`.

- [ ] **Step 3: Write the implementation**

`apps/web/src/llm/openaiAdapter.ts`:
```ts
import type { LlmConfig, ChatRequest, ChatResult } from "./types";
import { LlmError } from "./LlmError";

export async function openaiAdapter(config: LlmConfig, req: ChatRequest): Promise<ChatResult> {
  let res: Response;
  try {
    res = await fetch(`${config.baseUrl}/chat/completions`, {
      method: "POST",
      headers: {
        "Authorization": `Bearer ${config.apiKey}`,
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        model: config.model,
        messages: req.messages,
        max_tokens: req.maxTokens ?? 1024,
        ...(req.temperature !== undefined ? { temperature: req.temperature } : {}),
      }),
    });
  } catch {
    throw new LlmError("网络请求失败（检查 baseUrl / CORS）", { provider: "openai" });
  }
  const data = (await res.json().catch(() => ({}))) as {
    choices?: { message?: { content?: string } }[];
    usage?: { prompt_tokens?: number; completion_tokens?: number };
    error?: { message?: string };
  };
  if (!res.ok) {
    throw new LlmError(data.error?.message ?? `OpenAI 接口返回 ${res.status}`, { status: res.status, provider: "openai", body: data });
  }
  return {
    text: data.choices?.[0]?.message?.content ?? "",
    usage: { inputTokens: data.usage?.prompt_tokens, outputTokens: data.usage?.completion_tokens },
    raw: data,
  };
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm --filter web test -- openaiAdapter`
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/llm/openaiAdapter.ts apps/web/src/llm/openaiAdapter.test.ts
git commit -m "feat(llm): openai-format adapter"
```

---

### Task 3: Anthropic adapter

**Files:**
- Create: `apps/web/src/llm/anthropicAdapter.ts`
- Test: `apps/web/src/llm/anthropicAdapter.test.ts`

**Interfaces:**
- Consumes: `LlmConfig`, `ChatRequest`, `ChatResult` (types); `LlmError`.
- Produces: `anthropicAdapter(config: LlmConfig, req: ChatRequest): Promise<ChatResult>`.

- [ ] **Step 1: Write the failing test**

`apps/web/src/llm/anthropicAdapter.test.ts`:
```ts
import { describe, it, expect, vi, afterEach } from "vitest";
import { anthropicAdapter } from "./anthropicAdapter";
import type { LlmConfig } from "./types";

const cfg: LlmConfig = { format: "anthropic", baseUrl: "https://api.anthropic.com/v1", model: "claude-3-5-haiku", apiKey: "sk-ant-xyz" };

function mockFetch(status: number, body: unknown) {
  const fn = vi.fn().mockResolvedValue({ ok: status >= 200 && status < 300, status, json: () => Promise.resolve(body) });
  vi.stubGlobal("fetch", fn);
  return fn;
}
afterEach(() => vi.unstubAllGlobals());

describe("anthropicAdapter", () => {
  it("POSTs to /messages, lifts system out, sets anthropic headers, parses content", async () => {
    const fetchMock = mockFetch(200, { content: [{ type: "text", text: "OK" }], usage: { input_tokens: 7, output_tokens: 3 } });
    const result = await anthropicAdapter(cfg, { messages: [{ role: "system", content: "be brief" }, { role: "user", content: "hi" }] });
    const [url, init] = fetchMock.mock.calls[0]!;
    expect(url).toBe("https://api.anthropic.com/v1/messages");
    const h = init.headers as Record<string, string>;
    expect(h["x-api-key"]).toBe("sk-ant-xyz");
    expect(h["anthropic-version"]).toBe("2023-06-01");
    expect(h["anthropic-dangerous-direct-browser-access"]).toBe("true");
    const sent = JSON.parse(init.body as string);
    expect(sent.system).toBe("be brief");
    expect(sent.messages).toEqual([{ role: "user", content: "hi" }]);
    expect(sent.max_tokens).toBe(1024);
    expect(result.text).toBe("OK");
    expect(result.usage).toEqual({ inputTokens: 7, outputTokens: 3 });
  });
  it("throws LlmError on non-2xx and never leaks the key", async () => {
    mockFetch(400, { error: { message: "bad model" } });
    const err = await anthropicAdapter(cfg, { messages: [] }).catch((e) => e as Error);
    expect(err).toMatchObject({ name: "LlmError", status: 400, message: "bad model" });
    expect(err.message).not.toContain("sk-ant-xyz");
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm --filter web test -- anthropicAdapter`
Expected: FAIL — cannot resolve `./anthropicAdapter`.

- [ ] **Step 3: Write the implementation**

`apps/web/src/llm/anthropicAdapter.ts`:
```ts
import type { LlmConfig, ChatRequest, ChatResult } from "./types";
import { LlmError } from "./LlmError";

export async function anthropicAdapter(config: LlmConfig, req: ChatRequest): Promise<ChatResult> {
  const system = req.messages.filter((m) => m.role === "system").map((m) => m.content).join("\n\n");
  const messages = req.messages.filter((m) => m.role !== "system");
  let res: Response;
  try {
    res = await fetch(`${config.baseUrl}/messages`, {
      method: "POST",
      headers: {
        "x-api-key": config.apiKey,
        "anthropic-version": "2023-06-01",
        "anthropic-dangerous-direct-browser-access": "true",
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        model: config.model,
        max_tokens: req.maxTokens ?? 1024,
        ...(system ? { system } : {}),
        ...(req.temperature !== undefined ? { temperature: req.temperature } : {}),
        messages,
      }),
    });
  } catch {
    throw new LlmError("网络请求失败（检查 baseUrl / CORS）", { provider: "anthropic" });
  }
  const data = (await res.json().catch(() => ({}))) as {
    content?: { type?: string; text?: string }[];
    usage?: { input_tokens?: number; output_tokens?: number };
    error?: { message?: string };
  };
  if (!res.ok) {
    throw new LlmError(data.error?.message ?? `Anthropic 接口返回 ${res.status}`, { status: res.status, provider: "anthropic", body: data });
  }
  const textBlock = data.content?.find((b) => b.type === "text");
  return {
    text: textBlock?.text ?? "",
    usage: { inputTokens: data.usage?.input_tokens, outputTokens: data.usage?.output_tokens },
    raw: data,
  };
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm --filter web test -- anthropicAdapter`
Expected: PASS (2 tests).

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/llm/anthropicAdapter.ts apps/web/src/llm/anthropicAdapter.test.ts
git commit -m "feat(llm): anthropic-format adapter (system extraction + browser-direct header)"
```

---

### Task 4: Config store (+ Vite env typing)

**Files:**
- Create: `apps/web/src/llm/config.ts`
- Create: `apps/web/src/vite-env.d.ts`
- Test: `apps/web/src/llm/config.test.ts`

**Interfaces:**
- Consumes: `LlmConfig`, `LlmFormat` (types).
- Produces: `loadConfig(env?): Partial<LlmConfig>`, `saveConfig(cfg: LlmConfig): void`, `isConfigured(cfg: Partial<LlmConfig>): cfg is LlmConfig`.

- [ ] **Step 1: Add Vite client typing**

`apps/web/src/vite-env.d.ts` (so `import.meta.env` is typed — `apps/web/tsconfig.json` sets explicit `types`, and this triple-slash reference pulls Vite's client types in regardless):
```ts
/// <reference types="vite/client" />
```

- [ ] **Step 2: Write the failing test**

`apps/web/src/llm/config.test.ts`:
```ts
import { describe, it, expect, beforeEach } from "vitest";
import { loadConfig, saveConfig, isConfigured } from "./config";
import type { LlmConfig } from "./types";

beforeEach(() => localStorage.clear());

describe("config", () => {
  it("saveConfig then loadConfig roundtrips via localStorage", () => {
    const cfg: LlmConfig = { format: "openai", baseUrl: "u", model: "m", apiKey: "k" };
    saveConfig(cfg);
    expect(loadConfig({})).toEqual(cfg);
  });
  it("falls back to injected env when storage is empty", () => {
    const cfg = loadConfig({
      VITE_LLM_FORMAT: "anthropic", VITE_LLM_BASE_URL: "b", VITE_LLM_MODEL: "m", VITE_LLM_API_KEY: "k",
    });
    expect(cfg).toMatchObject({ format: "anthropic", baseUrl: "b", model: "m", apiKey: "k" });
  });
  it("isConfigured requires all four core fields", () => {
    expect(isConfigured({ format: "openai", baseUrl: "u", model: "m", apiKey: "k" })).toBe(true);
    expect(isConfigured({ format: "openai", baseUrl: "u", model: "m" })).toBe(false);
  });
});
```

- [ ] **Step 3: Run test to verify it fails**

Run: `pnpm --filter web test -- llm/config`
Expected: FAIL — cannot resolve `./config`.

- [ ] **Step 4: Write the implementation**

`apps/web/src/llm/config.ts`:
```ts
import type { LlmConfig, LlmFormat } from "./types";

const STORAGE_KEY = "mk.llmConfig";
type EnvSource = Record<string, string | undefined>;

export function loadConfig(env: EnvSource = import.meta.env as EnvSource): Partial<LlmConfig> {
  const stored = typeof localStorage !== "undefined" ? localStorage.getItem(STORAGE_KEY) : null;
  if (stored) {
    try {
      return JSON.parse(stored) as LlmConfig;
    } catch {
      /* corrupt storage — fall through to env defaults */
    }
  }
  const fmt = env.VITE_LLM_FORMAT;
  return {
    format: fmt === "anthropic" || fmt === "openai" ? (fmt as LlmFormat) : undefined,
    baseUrl: env.VITE_LLM_BASE_URL,
    model: env.VITE_LLM_MODEL,
    apiKey: env.VITE_LLM_API_KEY,
    evalModel: env.VITE_LLM_EVAL_MODEL || undefined,
  };
}

export function saveConfig(cfg: LlmConfig): void {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(cfg));
}

export function isConfigured(cfg: Partial<LlmConfig>): cfg is LlmConfig {
  return Boolean(cfg.format && cfg.baseUrl && cfg.model && cfg.apiKey);
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `pnpm --filter web test -- llm/config`
Expected: PASS (3 tests).

- [ ] **Step 6: Commit**

```bash
git add apps/web/src/llm/config.ts apps/web/src/vite-env.d.ts apps/web/src/llm/config.test.ts
git commit -m "feat(llm): config load/save/isConfigured + vite env typing"
```

---

### Task 5: Unified client + barrel

**Files:**
- Create: `apps/web/src/llm/client.ts`
- Create: `apps/web/src/llm/index.ts`
- Test: `apps/web/src/llm/client.test.ts`

**Interfaces:**
- Consumes: `isConfigured` (config), `openaiAdapter`, `anthropicAdapter`, `LlmError`, types.
- Produces: `chat(config: Partial<LlmConfig>, req: ChatRequest): Promise<ChatResult>`; barrel `index.ts` re-exporting types, `LlmError`, config fns, and `chat`.

- [ ] **Step 1: Write the failing test**

`apps/web/src/llm/client.test.ts`:
```ts
import { describe, it, expect, vi, afterEach } from "vitest";
import { chat } from "./client";

function mockFetch(body: unknown) {
  const fn = vi.fn().mockResolvedValue({ ok: true, status: 200, json: () => Promise.resolve(body) });
  vi.stubGlobal("fetch", fn);
  return fn;
}
afterEach(() => vi.unstubAllGlobals());

describe("chat (dispatch by format)", () => {
  it("routes openai format to /chat/completions", async () => {
    const fetchMock = mockFetch({ choices: [{ message: { content: "x" } }] });
    await chat({ format: "openai", baseUrl: "http://h/v1", model: "m", apiKey: "k" }, { messages: [] });
    expect(fetchMock.mock.calls[0]![0]).toBe("http://h/v1/chat/completions");
  });
  it("routes anthropic format to /messages", async () => {
    const fetchMock = mockFetch({ content: [{ type: "text", text: "x" }] });
    await chat({ format: "anthropic", baseUrl: "http://h/v1", model: "m", apiKey: "k" }, { messages: [] });
    expect(fetchMock.mock.calls[0]![0]).toBe("http://h/v1/messages");
  });
  it("throws LlmError when not configured", async () => {
    await expect(chat({ format: "openai", baseUrl: "", model: "", apiKey: "" }, { messages: [] }))
      .rejects.toMatchObject({ name: "LlmError" });
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm --filter web test -- llm/client`
Expected: FAIL — cannot resolve `./client`.

- [ ] **Step 3: Write the implementation + barrel**

`apps/web/src/llm/client.ts`:
```ts
import type { LlmConfig, ChatRequest, ChatResult } from "./types";
import { isConfigured } from "./config";
import { LlmError } from "./LlmError";
import { openaiAdapter } from "./openaiAdapter";
import { anthropicAdapter } from "./anthropicAdapter";

export async function chat(config: Partial<LlmConfig>, req: ChatRequest): Promise<ChatResult> {
  if (!isConfigured(config)) {
    throw new LlmError("LLM 未配置（缺 format / baseUrl / model / apiKey）");
  }
  return config.format === "anthropic" ? anthropicAdapter(config, req) : openaiAdapter(config, req);
}
```

`apps/web/src/llm/index.ts`:
```ts
export * from "./types";
export * from "./LlmError";
export * from "./config";
export * from "./client";
```

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm --filter web test -- llm/client`
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/llm/client.ts apps/web/src/llm/index.ts apps/web/src/llm/client.test.ts
git commit -m "feat(llm): unified chat() dispatch + module barrel"
```

---

### Task 6: SettingsPanel

**Files:**
- Create: `apps/web/src/dev/SettingsPanel.tsx`
- Test: `apps/web/src/dev/SettingsPanel.test.tsx`

**Interfaces:**
- Consumes: `loadConfig`, `saveConfig`, `chat`, `LlmError`, types from `../llm`.
- Produces: `<SettingsPanel />` — a form (format select + baseUrl/model/apiKey/evalModel inputs) with `保存` and `测试连接` buttons and a status line.

- [ ] **Step 1: Write the failing test**

`apps/web/src/dev/SettingsPanel.test.tsx`:
```tsx
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SettingsPanel } from "./SettingsPanel";
import { loadConfig } from "../llm/config";

beforeEach(() => localStorage.clear());
afterEach(() => vi.unstubAllGlobals());

async function fillCore() {
  await userEvent.clear(screen.getByLabelText("Base URL"));
  await userEvent.type(screen.getByLabelText("Base URL"), "https://api.openai.com/v1");
  await userEvent.clear(screen.getByLabelText("Model"));
  await userEvent.type(screen.getByLabelText("Model"), "gpt-4o-mini");
  await userEvent.clear(screen.getByLabelText("API Key"));
  await userEvent.type(screen.getByLabelText("API Key"), "sk-demo");
}

describe("SettingsPanel", () => {
  it("saves entered config to storage", async () => {
    render(<SettingsPanel />);
    await fillCore();
    await userEvent.click(screen.getByRole("button", { name: "保存" }));
    expect(loadConfig({})).toMatchObject({ format: "openai", baseUrl: "https://api.openai.com/v1", model: "gpt-4o-mini", apiKey: "sk-demo" });
  });
  it("test connection shows the model reply on success", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true, status: 200, json: () => Promise.resolve({ choices: [{ message: { content: "OK!" } }] }) }));
    render(<SettingsPanel />);
    await fillCore();
    await userEvent.click(screen.getByRole("button", { name: "测试连接" }));
    expect(await screen.findByText(/OK!/)).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm --filter web test -- SettingsPanel`
Expected: FAIL — cannot resolve `./SettingsPanel`.

- [ ] **Step 3: Write the implementation**

`apps/web/src/dev/SettingsPanel.tsx`:
```tsx
import { useState } from "react";
import type { LlmConfig, LlmFormat } from "../llm/types";
import { loadConfig, saveConfig } from "../llm/config";
import { chat } from "../llm/client";
import { LlmError } from "../llm/LlmError";

type Status = { kind: "idle" | "testing" | "ok" | "err"; text?: string };

const inputCls = "mt-1 w-full rounded-[10px] border border-mk-input bg-mk-input-bg px-3 py-2.5 text-sm text-mk-ink outline-none";
const labelCls = "block text-[13px] font-semibold text-[#3A4256] mt-3";

export function SettingsPanel() {
  const initial = loadConfig();
  const [format, setFormat] = useState<LlmFormat>(initial.format ?? "openai");
  const [baseUrl, setBaseUrl] = useState(initial.baseUrl ?? "");
  const [model, setModel] = useState(initial.model ?? "");
  const [apiKey, setApiKey] = useState(initial.apiKey ?? "");
  const [evalModel, setEvalModel] = useState(initial.evalModel ?? "");
  const [status, setStatus] = useState<Status>({ kind: "idle" });

  const current = (): LlmConfig => ({ format, baseUrl, model, apiKey, evalModel: evalModel || undefined });

  const onSave = () => { saveConfig(current()); setStatus({ kind: "idle", text: "已保存" }); };
  const onTest = async () => {
    setStatus({ kind: "testing", text: "测试中…" });
    try {
      const r = await chat(current(), { messages: [{ role: "user", content: "回复 OK" }], maxTokens: 16 });
      setStatus({ kind: "ok", text: r.text || "(空回复)" });
    } catch (e) {
      setStatus({ kind: "err", text: e instanceof LlmError ? e.message : "未知错误" });
    }
  };

  return (
    <div className="mx-auto max-w-[560px] p-8 font-sans text-mk-ink">
      <h2 className="text-lg font-bold">LLM 设置（BYO-key）</h2>
      <p className="mt-1 text-[13px] text-mk-muted-2">密钥只存在你的浏览器，不上传、不入库。也可用 apps/web/.env.local 预填。</p>

      <label className={labelCls}>格式
        <select aria-label="格式" value={format} onChange={(e) => setFormat(e.target.value as LlmFormat)} className={inputCls}>
          <option value="openai">openai</option>
          <option value="anthropic">anthropic</option>
        </select>
      </label>
      <label className={labelCls}>Base URL
        <input aria-label="Base URL" value={baseUrl} onChange={(e) => setBaseUrl(e.target.value)} placeholder="https://api.openai.com/v1" className={inputCls} />
      </label>
      <label className={labelCls}>Model
        <input aria-label="Model" value={model} onChange={(e) => setModel(e.target.value)} className={inputCls} />
      </label>
      <label className={labelCls}>API Key
        <input aria-label="API Key" type="password" value={apiKey} onChange={(e) => setApiKey(e.target.value)} className={inputCls} />
      </label>
      <label className={labelCls}>Eval Model（可选）
        <input aria-label="Eval Model" value={evalModel} onChange={(e) => setEvalModel(e.target.value)} className={inputCls} />
      </label>

      <div className="mt-5 flex items-center gap-3">
        <button type="button" onClick={onSave} className="rounded-[11px] bg-mk-primary px-5 py-2.5 text-sm font-bold text-white">保存</button>
        <button type="button" onClick={onTest} className="rounded-[11px] bg-mk-accent px-5 py-2.5 text-sm font-bold text-white">测试连接</button>
        {status.text && (
          <span className={`text-[13px] font-semibold ${status.kind === "err" ? "text-[#C0552F]" : status.kind === "ok" ? "text-mk-green" : "text-mk-muted-2"}`}>
            {status.kind === "ok" ? "✅ " : status.kind === "err" ? "❌ " : ""}{status.text}
          </span>
        )}
      </div>
    </div>
  );
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm --filter web test -- SettingsPanel`
Expected: PASS (2 tests).

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/dev/SettingsPanel.tsx apps/web/src/dev/SettingsPanel.test.tsx
git commit -m "feat(web): LLM SettingsPanel (save + test connection)"
```

---

### Task 7: DevApp tabs + mount

**Files:**
- Create: `apps/web/src/dev/DevApp.tsx`
- Modify: `apps/web/src/main.tsx`
- Test: `apps/web/src/dev/DevApp.test.tsx`

**Interfaces:**
- Consumes: `Harness` (`./Harness`), `SettingsPanel` (`./SettingsPanel`).
- Produces: `<DevApp />` — two tabs (`卡片` ｜ `LLM 设置`), defaulting to the cards harness.

- [ ] **Step 1: Write the failing test**

`apps/web/src/dev/DevApp.test.tsx`:
```tsx
import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DevApp } from "./DevApp";

describe("DevApp", () => {
  it("defaults to the cards harness and switches to LLM settings", async () => {
    render(<DevApp />);
    expect(screen.queryByRole("button", { name: "测试连接" })).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "LLM 设置" }));
    expect(screen.getByRole("button", { name: "测试连接" })).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm --filter web test -- DevApp`
Expected: FAIL — cannot resolve `./DevApp`.

- [ ] **Step 3: Write DevApp + repoint main**

`apps/web/src/dev/DevApp.tsx`:
```tsx
import { useState } from "react";
import { Harness } from "./Harness";
import { SettingsPanel } from "./SettingsPanel";

type Tab = "cards" | "llm";

export function DevApp() {
  const [tab, setTab] = useState<Tab>("cards");
  const tabCls = (t: Tab) =>
    `rounded-full px-3 py-1.5 text-[13px] font-semibold ${tab === t ? "bg-mk-primary text-white" : "bg-white text-mk-muted-2"}`;
  return (
    <div className="min-h-screen bg-mk-bg font-sans text-mk-ink">
      <div className="flex gap-2 border-b border-mk-border bg-white px-6 py-3">
        <button type="button" onClick={() => setTab("cards")} className={tabCls("cards")}>卡片</button>
        <button type="button" onClick={() => setTab("llm")} className={tabCls("llm")}>LLM 设置</button>
      </div>
      {tab === "cards" ? <Harness /> : <SettingsPanel />}
    </div>
  );
}
```

`apps/web/src/main.tsx` — replace the body so it renders `DevApp`:
```tsx
import React from "react";
import { createRoot } from "react-dom/client";
import { DevApp } from "./dev/DevApp";
import "./index.css";

createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <DevApp />
  </React.StrictMode>,
);
```

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm --filter web test -- DevApp`
Expected: PASS (1 test).

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/dev/DevApp.tsx apps/web/src/dev/DevApp.test.tsx apps/web/src/main.tsx
git commit -m "feat(web): DevApp tabs (cards | LLM settings); mount in main"
```

---

### Task 8: Gated live smoke test + full slice verification

**Files:**
- Create: `apps/web/src/llm/live.smoke.test.ts`

**Interfaces:**
- Consumes: `loadConfig`, `isConfigured`, `chat`.
- Produces: a live test that runs only when a real config is present (via `.env.local`), and skips otherwise.

- [ ] **Step 1: Write the gated live smoke test**

`apps/web/src/llm/live.smoke.test.ts`:
```ts
import { describe, it, expect } from "vitest";
import { loadConfig, isConfigured } from "./config";
import { chat } from "./client";

const cfg = loadConfig();
const runnable = isConfigured(cfg);

// Skips automatically unless apps/web/.env.local provides a full VITE_LLM_* config.
describe.skipIf(!runnable)("LLM live smoke (needs apps/web/.env.local)", () => {
  it("returns a non-empty reply from the configured provider", async () => {
    const r = await chat(cfg, { messages: [{ role: "user", content: "Reply with the single word: OK" }], maxTokens: 16 });
    expect(r.text.length).toBeGreaterThan(0);
  });
});
```

- [ ] **Step 2: Run it and confirm it SKIPS (no key in this environment)**

Run: `pnpm --filter web test -- live.smoke`
Expected: PASS with the suite reported as **skipped** (0 failures; the `describe.skipIf` triggers because no `.env.local` config is present).

- [ ] **Step 3: Full slice verification**

Run: `pnpm -r typecheck`
Expected: PASS — contracts and web both typecheck (incl. the new `llm/` module and `import.meta.env` typing).

Run: `pnpm -r test`
Expected: PASS — all contracts + web suites green; the live smoke suite shows as skipped.

Run: `git check-ignore apps/web/.env.local`
Expected: prints `apps/web/.env.local` (confirming it is git-ignored). Note: the file does not exist yet — `check-ignore` reports the path as ignored regardless.

- [ ] **Step 4: Commit**

```bash
git add apps/web/src/llm/live.smoke.test.ts
git commit -m "test(llm): gated live smoke test (skips without .env.local)"
```

---

## Self-Review

**Spec coverage** (against `docs/superpowers/specs/2026-06-20-slice2-llm-client-design.md`):
- §4 types → Task 1. §5.1 openai adapter → Task 2. §5.2 anthropic adapter (system extraction, headers) → Task 3. §5.3 `chat` dispatch + not-configured guard → Task 5. §6 config load/save/isConfigured + env fallback → Task 4. §6.1 `.env.example` → Task 1. §7 SettingsPanel → Task 6; DevApp tabs + main → Task 7. §9 tests 1–11 → Tasks 2–7; test 12 (gated live smoke) → Task 8. §10 DoD (typecheck+test green, `.env.local` ignored, no key leak) → Task 8 Step 3 + the per-adapter no-leak tests.
- §2 out-of-scope items (tools, persistence, streaming, metering, real settings page, summon_card) — none implemented; `ChatRequest.tools` only reserved as a comment (Task 1).

**Placeholder scan:** no TBD/TODO; every code step has complete code; every test step has real assertions.

**Type consistency:** `LlmConfig`/`ChatRequest`/`ChatResult`/`ChatUsage` (Task 1) used identically in Tasks 2–8; `LlmError` constructor signature `(message, {status, provider, body})` consistent across adapters and client; `openaiAdapter`/`anthropicAdapter` signatures match what `chat` calls (Task 5); `loadConfig`/`saveConfig`/`isConfigured` names match across Tasks 4–8; `chat(config, req)` signature consistent in SettingsPanel (Task 6) and the live smoke test (Task 8).

**Note on `import.meta.env` typing:** `apps/web/tsconfig.json` sets explicit `types`, which would normally exclude Vite's ambient `import.meta.env` typing; Task 4 Step 1 adds `apps/web/src/vite-env.d.ts` with a triple-slash `vite/client` reference, which is honored independent of the `types` option.
