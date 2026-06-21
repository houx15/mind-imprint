import { describe, it, expect, vi, afterEach } from "vitest";
import { anthropicAdapter } from "./anthropicAdapter";
import type { LlmConfig } from "./types";

const cfg: LlmConfig = { format: "anthropic", baseUrl: "https://api.anthropic.com/v1", model: "claude-3-5-haiku", apiKey: "sk-ant-xyz" };

function mockFetch(status: number, body: unknown) {
  const fn = vi.fn().mockResolvedValue({ ok: status >= 200 && status < 300, status, json: () => Promise.resolve(body) });
  vi.stubGlobal("fetch", fn);
  return fn;
}
afterEach(() => { vi.unstubAllGlobals(); });

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
    const err = await anthropicAdapter(cfg, { messages: [] }).catch((e: unknown) => e);
    expect(err).toMatchObject({ name: "LlmError", status: 400, provider: "anthropic", message: "bad model" });
    expect((err as Error).message).not.toContain("sk-ant-xyz");
  });
  it("returns empty text when no content block is type text", async () => {
    mockFetch(200, { content: [{ type: "tool_use", id: "x" }], usage: { input_tokens: 1, output_tokens: 0 } });
    const result = await anthropicAdapter(cfg, { messages: [{ role: "user", content: "hi" }] });
    expect(result.text).toBe("");
  });
  it("wraps a network/fetch failure as LlmError without leaking the key", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("Failed to fetch")));
    const err = await anthropicAdapter(cfg, { messages: [] }).catch((e: unknown) => e);
    expect(err).toMatchObject({ name: "LlmError", provider: "anthropic" });
    expect((err as Error).message).not.toContain("sk-ant-xyz");
  });

  // Tool support tests (Task 4)
  it("includes tools with input_schema when req.tools is provided", async () => {
    const fetchMock = mockFetch(200, { content: [{ type: "text", text: "ok" }], usage: { input_tokens: 5, output_tokens: 2 } });
    await anthropicAdapter(cfg, {
      messages: [{ role: "user", content: "go" }],
      tools: [{ name: "summon_card", description: "summons a card", parameters: { type: "object", properties: { card_id: { type: "string" } } } }],
    });
    const [, init] = fetchMock.mock.calls[0]!;
    const sent = JSON.parse(init.body as string);
    expect(sent.tools).toEqual([{
      name: "summon_card",
      description: "summons a card",
      input_schema: { type: "object", properties: { card_id: { type: "string" } } },
    }]);
  });

  it("does not include tools key when req.tools is empty/absent", async () => {
    const fetchMock = mockFetch(200, { content: [{ type: "text", text: "ok" }], usage: { input_tokens: 5, output_tokens: 2 } });
    await anthropicAdapter(cfg, { messages: [{ role: "user", content: "go" }] });
    const [, init] = fetchMock.mock.calls[0]!;
    const sent = JSON.parse(init.body as string);
    expect(sent.tools).toBeUndefined();
  });

  it("parses tool_use content blocks into toolCalls and maps stop_reason tool_use → tool_call", async () => {
    mockFetch(200, {
      content: [
        { type: "text", text: "thinking..." },
        { type: "tool_use", id: "tu1", name: "summon_card", input: { card_id: "sift_craap", reason: "r", nudge_text: "n" } },
      ],
      stop_reason: "tool_use",
      usage: { input_tokens: 10, output_tokens: 5 },
    });
    const result = await anthropicAdapter(cfg, { messages: [{ role: "user", content: "go" }] });
    expect(result.text).toBe("thinking...");
    expect(result.toolCalls).toEqual([{ id: "tu1", name: "summon_card", args: { card_id: "sift_craap", reason: "r", nudge_text: "n" } }]);
    expect(result.stopReason).toBe("tool_call");
  });

  it("maps stop_reason end_turn → stop and max_tokens → length", async () => {
    mockFetch(200, { content: [{ type: "text", text: "a" }], stop_reason: "end_turn", usage: { input_tokens: 1, output_tokens: 1 } });
    const r1 = await anthropicAdapter(cfg, { messages: [{ role: "user", content: "hi" }] });
    expect(r1.stopReason).toBe("stop");

    mockFetch(200, { content: [{ type: "text", text: "b" }], stop_reason: "max_tokens", usage: { input_tokens: 1, output_tokens: 1 } });
    const r2 = await anthropicAdapter(cfg, { messages: [{ role: "user", content: "hi" }] });
    expect(r2.stopReason).toBe("length");
  });

  it("serializes role:tool messages as user content blocks with type:tool_result", async () => {
    const fetchMock = mockFetch(200, { content: [{ type: "text", text: "done" }], usage: { input_tokens: 5, output_tokens: 2 } });
    await anthropicAdapter(cfg, {
      messages: [
        { role: "user", content: "go" },
        { role: "tool", content: "card filled", toolCallId: "tu1" },
      ],
    });
    const [, init] = fetchMock.mock.calls[0]!;
    const sent = JSON.parse(init.body as string);
    expect(sent.messages).toContainEqual({
      role: "user",
      content: [{ type: "tool_result", tool_use_id: "tu1", content: "card filled" }],
    });
  });

  it("serializes assistant messages with toolCalls as content blocks", async () => {
    const fetchMock = mockFetch(200, { content: [{ type: "text", text: "done" }], usage: { input_tokens: 5, output_tokens: 2 } });
    await anthropicAdapter(cfg, {
      messages: [
        { role: "user", content: "go" },
        {
          role: "assistant",
          content: "let me check",
          toolCalls: [{ id: "tu1", name: "summon_card", args: { card_id: "sift_craap" } }],
        },
        { role: "tool", content: "result", toolCallId: "tu1" },
      ],
    });
    const [, init] = fetchMock.mock.calls[0]!;
    const sent = JSON.parse(init.body as string);
    const assistantMsg = sent.messages.find((m: { role: string }) => m.role === "assistant");
    expect(assistantMsg.content).toEqual([
      { type: "text", text: "let me check" },
      { type: "tool_use", id: "tu1", name: "summon_card", input: { card_id: "sift_craap" } },
    ]);
  });

  it("serializes assistant messages with toolCalls but no text content (omits text block)", async () => {
    const fetchMock = mockFetch(200, { content: [{ type: "text", text: "done" }], usage: { input_tokens: 5, output_tokens: 2 } });
    await anthropicAdapter(cfg, {
      messages: [
        { role: "user", content: "go" },
        {
          role: "assistant",
          content: "",
          toolCalls: [{ id: "tu2", name: "summon_card", args: { card_id: "concession" } }],
        },
        { role: "tool", content: "result", toolCallId: "tu2" },
      ],
    });
    const [, init] = fetchMock.mock.calls[0]!;
    const sent = JSON.parse(init.body as string);
    const assistantMsg = sent.messages.find((m: { role: string }) => m.role === "assistant");
    expect(assistantMsg.content).toEqual([
      { type: "tool_use", id: "tu2", name: "summon_card", input: { card_id: "concession" } },
    ]);
  });

  it("system lift is preserved alongside tool messages", async () => {
    const fetchMock = mockFetch(200, { content: [{ type: "text", text: "ok" }], usage: { input_tokens: 5, output_tokens: 2 } });
    await anthropicAdapter(cfg, {
      messages: [
        { role: "system", content: "be helpful" },
        { role: "user", content: "go" },
        { role: "tool", content: "result", toolCallId: "tu1" },
      ],
    });
    const [, init] = fetchMock.mock.calls[0]!;
    const sent = JSON.parse(init.body as string);
    expect(sent.system).toBe("be helpful");
    // system message must NOT appear in messages array
    expect(sent.messages.every((m: { role: string }) => m.role !== "system")).toBe(true);
  });

  it("joins multiple text blocks with newline", async () => {
    mockFetch(200, {
      content: [
        { type: "text", text: "first" },
        { type: "text", text: "second" },
      ],
      stop_reason: "end_turn",
      usage: { input_tokens: 2, output_tokens: 4 },
    });
    const result = await anthropicAdapter(cfg, { messages: [{ role: "user", content: "hi" }] });
    expect(result.text).toBe("first\nsecond");
  });
});
