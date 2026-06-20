import { describe, it, expect, vi, afterEach } from "vitest";
import { chat } from "./client";

function mockFetch(body: unknown) {
  const fn = vi.fn().mockResolvedValue({ ok: true, status: 200, json: () => Promise.resolve(body) });
  vi.stubGlobal("fetch", fn);
  return fn;
}
afterEach(() => {
  vi.unstubAllGlobals();
});

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
