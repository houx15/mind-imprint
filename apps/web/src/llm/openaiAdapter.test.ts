import { describe, it, expect, vi, afterEach } from "vitest";
import { openaiAdapter } from "./openaiAdapter";
import type { LlmConfig, ChatTool } from "./types";

const cfg: LlmConfig = { format: "openai", baseUrl: "https://api.openai.com/v1", model: "gpt-4o-mini", apiKey: "sk-test-123" };

function mockFetch(status: number, body: unknown) {
  const fn = vi.fn().mockResolvedValue({ ok: status >= 200 && status < 300, status, json: () => Promise.resolve(body) });
  vi.stubGlobal("fetch", fn);
  return fn;
}
afterEach(() => { vi.unstubAllGlobals(); });

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
    await expect(openaiAdapter(cfg, { messages: [] })).rejects.toMatchObject({ name: "LlmError", status: 401, provider: "openai", message: "invalid key" });
  });
  it("never leaks the apiKey in the error message", async () => {
    mockFetch(500, { error: { message: "boom" } });
    const err = await openaiAdapter(cfg, { messages: [] }).catch((e: unknown) => e);
    expect((err as Error).message).not.toContain("sk-test-123");
  });
  it("wraps a network/fetch failure as LlmError without leaking the key", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("Failed to fetch")));
    const err = await openaiAdapter(cfg, { messages: [] }).catch((e: unknown) => e);
    expect(err).toMatchObject({ name: "LlmError", provider: "openai" });
    expect((err as { status?: number }).status).toBeUndefined();
    expect((err as Error).message).not.toContain("sk-test-123");
  });

  // Tool-use tests (Step 1 — new tests added for tool support)
  it("includes tools and tool_choice:auto in the request body when tools are provided", async () => {
    const tools: ChatTool[] = [
      { name: "summon_card", description: "Summon a thinking card", parameters: { type: "object", properties: { card_id: { type: "string" } }, required: ["card_id"] } },
    ];
    const fetchMock = mockFetch(200, { choices: [{ message: { content: "OK" }, finish_reason: "stop" }], usage: {} });
    await openaiAdapter(cfg, { messages: [{ role: "user", content: "help" }], tools });
    const [, init] = fetchMock.mock.calls[0]!;
    const sent = JSON.parse(init.body as string);
    expect(sent.tools).toEqual([
      { type: "function", function: { name: "summon_card", description: "Summon a thinking card", parameters: { type: "object", properties: { card_id: { type: "string" } }, required: ["card_id"] } } },
    ]);
    expect(sent.tool_choice).toBe("auto");
  });

  it("parses tool_calls from the response into result.toolCalls", async () => {
    mockFetch(200, {
      choices: [{
        message: {
          content: null,
          tool_calls: [{ id: "c1", type: "function", function: { name: "summon_card", arguments: '{"card_id":"sift_craap","reason":"r","nudge_text":"n"}' } }],
        },
        finish_reason: "tool_calls",
      }],
      usage: { prompt_tokens: 10, completion_tokens: 3 },
    });
    const result = await openaiAdapter(cfg, { messages: [{ role: "user", content: "check this" }] });
    expect(result.toolCalls).toEqual([{ id: "c1", name: "summon_card", args: { card_id: "sift_craap", reason: "r", nudge_text: "n" } }]);
    expect(result.stopReason).toBe("tool_call");
    expect(result.text).toBe("");
  });

  it("serializes role:tool messages with tool_call_id", async () => {
    const fetchMock = mockFetch(200, { choices: [{ message: { content: "done" }, finish_reason: "stop" }], usage: {} });
    await openaiAdapter(cfg, {
      messages: [
        { role: "tool", content: '{"result":"ok"}', toolCallId: "c1" },
      ],
    });
    const [, init] = fetchMock.mock.calls[0]!;
    const sent = JSON.parse(init.body as string);
    expect(sent.messages[0]).toEqual({ role: "tool", tool_call_id: "c1", content: '{"result":"ok"}' });
  });

  it("serializes assistant messages with toolCalls as tool_calls array", async () => {
    const fetchMock = mockFetch(200, { choices: [{ message: { content: "ok" }, finish_reason: "stop" }], usage: {} });
    await openaiAdapter(cfg, {
      messages: [
        {
          role: "assistant",
          content: "",
          toolCalls: [{ id: "c1", name: "summon_card", args: { card_id: "sift_craap", reason: "r", nudge_text: "n" } }],
        },
      ],
    });
    const [, init] = fetchMock.mock.calls[0]!;
    const sent = JSON.parse(init.body as string);
    expect(sent.messages[0]).toEqual({
      role: "assistant",
      content: null,
      tool_calls: [{ id: "c1", type: "function", function: { name: "summon_card", arguments: '{"card_id":"sift_craap","reason":"r","nudge_text":"n"}' } }],
    });
  });

  it("maps finish_reason:stop to stopReason:stop", async () => {
    mockFetch(200, { choices: [{ message: { content: "hi" }, finish_reason: "stop" }], usage: {} });
    const result = await openaiAdapter(cfg, { messages: [{ role: "user", content: "hi" }] });
    expect(result.stopReason).toBe("stop");
  });

  it("maps finish_reason:length to stopReason:length", async () => {
    mockFetch(200, { choices: [{ message: { content: "cut" }, finish_reason: "length" }], usage: {} });
    const result = await openaiAdapter(cfg, { messages: [{ role: "user", content: "hi" }] });
    expect(result.stopReason).toBe("length");
  });
});
