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
    expect(err).toMatchObject({ name: "LlmError", status: 400, provider: "anthropic", message: "bad model" });
    expect(err.message).not.toContain("sk-ant-xyz");
  });
  it("returns empty text when no content block is type text", async () => {
    mockFetch(200, { content: [{ type: "tool_use", id: "x" }], usage: { input_tokens: 1, output_tokens: 0 } });
    const result = await anthropicAdapter(cfg, { messages: [{ role: "user", content: "hi" }] });
    expect(result.text).toBe("");
  });
  it("wraps a network/fetch failure as LlmError without leaking the key", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("Failed to fetch")));
    const err = await anthropicAdapter(cfg, { messages: [] }).catch((e) => e as Error);
    expect(err).toMatchObject({ name: "LlmError", provider: "anthropic" });
    expect(err.message).not.toContain("sk-ant-xyz");
  });
});
