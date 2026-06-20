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
    await expect(openaiAdapter(cfg, { messages: [] })).rejects.toMatchObject({ name: "LlmError", status: 401, provider: "openai", message: "invalid key" });
  });
  it("never leaks the apiKey in the error message", async () => {
    mockFetch(500, { error: { message: "boom" } });
    const err = await openaiAdapter(cfg, { messages: [] }).catch((e) => e as Error);
    expect(err.message).not.toContain("sk-test-123");
  });
  it("wraps a network/fetch failure as LlmError without leaking the key", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("Failed to fetch")));
    const err = await openaiAdapter(cfg, { messages: [] }).catch((e) => e as Error);
    expect(err).toMatchObject({ name: "LlmError", provider: "openai" });
    expect((err as { status?: number }).status).toBeUndefined();
    expect(err.message).not.toContain("sk-test-123");
  });
});
