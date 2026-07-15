import { describe, it, expect, vi, afterEach } from "vitest";
import { putBuffer, commitSnapshot } from "./writing";

afterEach(() => { vi.restoreAllMocks(); });

describe("putBuffer", () => {
  it("PUTs the content to .../buffer and resolves on 204", async () => {
    const spy = vi.fn(async () => new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", spy);

    await putBuffer("p1", "草稿正文……");

    const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit & { body: string }];
    expect(url).toContain("/api/v1/projects/p1/buffer");
    expect(init.method).toBe("PUT");
    expect(JSON.parse(init.body)).toEqual({ content: "草稿正文……" });
  });

  it("throws when the save fails", async () => {
    const spy = vi.fn(async () => new Response(
      JSON.stringify({ error: { code: "internal_error", message: "保存失败，请重试。" } }),
      { status: 500, headers: { "Content-Type": "application/json" } },
    ));
    vi.stubGlobal("fetch", spy);

    await expect(putBuffer("p1", "x")).rejects.toThrow("保存失败，请重试。");
  });
});

describe("commitSnapshot", () => {
  it("POSTs the content to .../snapshots and parses the server shape", async () => {
    const spy = vi.fn(async () => new Response(
      JSON.stringify({ id: "snap-1", seq: 3, word_count: 420, in_band: true }),
      { status: 201, headers: { "Content-Type": "application/json" } },
    ));
    vi.stubGlobal("fetch", spy);

    const result = await commitSnapshot("p1", "定稿正文……");

    const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit & { body: string }];
    expect(url).toContain("/api/v1/projects/p1/snapshots");
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body)).toEqual({ content: "定稿正文……" });
    expect(result).toEqual({ id: "snap-1", seq: 3, wordCount: 420, inBand: true });
  });

  it("throws on a malformed snapshot response (schema drift)", async () => {
    const spy = vi.fn(async () => new Response(
      JSON.stringify({ id: "snap-1", seq: "not-a-number", word_count: 420, in_band: true }),
      { status: 201, headers: { "Content-Type": "application/json" } },
    ));
    vi.stubGlobal("fetch", spy);

    await expect(commitSnapshot("p1", "x")).rejects.toThrow();
  });

  it("surfaces the server's Chinese error message on failure, verbatim", async () => {
    const spy = vi.fn(async () => new Response(
      JSON.stringify({ error: { code: "validation_failed", message: "草稿是空的，先写点东西再提交。" } }),
      { status: 400, headers: { "Content-Type": "application/json" } },
    ));
    vi.stubGlobal("fetch", spy);

    await expect(commitSnapshot("p1", "")).rejects.toThrow("草稿是空的，先写点东西再提交。");
  });
});
