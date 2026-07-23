import { describe, it, expect, vi, afterEach } from "vitest";
import { putBuffer, commitSnapshot, orderReview, orderSpotCheck, attestGate } from "@/api/writing";
import { API_BASE } from "@/api/client";

afterEach(() => { vi.restoreAllMocks(); });

function sseBody(frames: string): Response {
  const stream = new ReadableStream<Uint8Array>({
    start(c) { c.enqueue(new TextEncoder().encode(frames)); c.close(); },
  });
  return new Response(stream, { status: 200, headers: { "Content-Type": "text/event-stream" } });
}

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

describe("orderReview", () => {
  it("POSTs to .../snapshots/{sid}/review and yields the review event then done", async () => {
    const spy = vi.fn(async () => sseBody(
      `event: review\ndata: [{"criterion_code":"表E","criterion_name":"分析","band":"5–6 段","evidence":"e","missing":"m","fix":"f"}]\n\n` +
      `event: done\ndata: {}\n\n`,
    ));
    vi.stubGlobal("fetch", spy);

    const events = [];
    for await (const e of orderReview("p1", "snap-1")) events.push(e);

    const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toContain("/api/v1/projects/p1/snapshots/snap-1/review");
    expect(init.method).toBe("POST");
    expect(events[0]).toMatchObject({ type: "review", items: [{ criterion_code: "表E" }] });
    expect(events.at(-1)).toEqual({ type: "done" });
  });

  it("yields an error event on a non-OK response instead of throwing", async () => {
    const spy = vi.fn(async () => new Response(
      JSON.stringify({ error: { code: "review_rejected", message: "这次体检没通过内部校验，请再试一次" } }),
      { status: 400, headers: { "Content-Type": "application/json" } },
    ));
    vi.stubGlobal("fetch", spy);

    const events = [];
    for await (const e of orderReview("p1", "snap-1")) events.push(e);
    expect(events).toEqual([{ type: "error", code: "review_rejected", message: "这次体检没通过内部校验，请再试一次" }]);
  });

  it("appends ?voice= for a non-board voice", async () => {
    const spy = vi.fn(async () => sseBody(`event: done\ndata: {}\n\n`));
    vi.stubGlobal("fetch", spy);

    for await (const _ of orderReview("p1", "snap-1", "sceptic")) { /* drain */ }

    const [url] = spy.mock.calls[0] as unknown as [string];
    expect(url).toContain("/api/v1/projects/p1/snapshots/snap-1/review?voice=sceptic");
  });

  it("omits the query for board (URL identical to the keystone call)", async () => {
    const spy = vi.fn(async () => sseBody(`event: done\ndata: {}\n\n`));
    vi.stubGlobal("fetch", spy);

    for await (const _ of orderReview("p1", "snap-1", "board")) { /* drain */ }

    const [url] = spy.mock.calls[0] as unknown as [string];
    expect(url).toContain("/api/v1/projects/p1/snapshots/snap-1/review");
    expect(url).not.toContain("voice=");
  });

  it("defaults to board when voice is omitted (URL byte-identical to today's call)", async () => {
    const spy = vi.fn(async () => sseBody(`event: done\ndata: {}\n\n`));
    vi.stubGlobal("fetch", spy);

    for await (const _ of orderReview("p1", "snap-1")) { /* drain */ }

    const [url] = spy.mock.calls[0] as unknown as [string];
    expect(url).toBe(`${API_BASE}/api/v1/projects/p1/snapshots/snap-1/review`);
  });
});

describe("orderSpotCheck", () => {
  it("POSTs to .../contracts/{contractId}/spot-check and resolves on done", async () => {
    const spy = vi.fn(async () => sseBody(
      `event: review\ndata: [{"target_id":"m1","target_name":"NASA","evidence":"e","missing":"m","fix":"f"}]\n\n` +
      `event: done\ndata: {}\n\n`,
    ));
    vi.stubGlobal("fetch", spy);

    await expect(orderSpotCheck("p1", "evaluate_sources")).resolves.toBeUndefined();

    const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toContain("/api/v1/projects/p1/contracts/evaluate_sources/spot-check");
    expect(init.method).toBe("POST");
  });

  it("rejects with the server's error envelope on a non-OK response", async () => {
    const spy = vi.fn(async () => new Response(
      JSON.stringify({ error: { code: "nothing_to_check", message: "还没有可以体检的内容。" } }),
      { status: 400, headers: { "Content-Type": "application/json" } },
    ));
    vi.stubGlobal("fetch", spy);

    await expect(orderSpotCheck("p1", "evaluate_sources")).rejects.toThrow("还没有可以体检的内容。");
  });

  it("rejects when the stream itself carries an error frame", async () => {
    const spy = vi.fn(async () => sseBody(
      `event: error\ndata: {"error":{"code":"spot_check_rejected","message":"这次体检没通过内部校验，请再试一次"}}\n\n`,
    ));
    vi.stubGlobal("fetch", spy);

    await expect(orderSpotCheck("p1", "build_argument")).rejects.toThrow("这次体检没通过内部校验，请再试一次");
  });

  // M4 fix: the underlying HTTP response for this path is a 200 (the stream
  // itself succeeded; only its in-band payload reported failure) — asserting
  // `err.status` locks in that this is never mistaken for a real HTTP status
  // a future caller might branch on (e.g. `err.status === 400`).
  it("carries the sentinel status 0 (not the underlying 200) on an SSE error-frame rejection", async () => {
    const spy = vi.fn(async () => sseBody(
      `event: error\ndata: {"error":{"code":"spot_check_rejected","message":"这次体检没通过内部校验，请再试一次"}}\n\n`,
    ));
    vi.stubGlobal("fetch", spy);

    await expect(orderSpotCheck("p1", "build_argument")).rejects.toMatchObject({ status: 0, code: "spot_check_rejected" });
  });
});

describe("attestGate", () => {
  it("POSTs {item, confirmed} to .../gate/{contractId}/attest and resolves on 204", async () => {
    const spy = vi.fn(async () => new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", spy);

    await attestGate("p1", "draft_polish", "citations_matched", true);

    const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit & { body: string }];
    expect(url).toContain("/api/v1/projects/p1/gate/draft_polish/attest");
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body)).toEqual({ item: "citations_matched", confirmed: true });
  });

  it("throws on failure", async () => {
    const spy = vi.fn(async () => new Response(
      JSON.stringify({ error: { code: "validation_failed", message: "该条目不是学生自评项" } }),
      { status: 400, headers: { "Content-Type": "application/json" } },
    ));
    vi.stubGlobal("fetch", spy);

    await expect(attestGate("p1", "draft_polish", "bogus", true)).rejects.toThrow("该条目不是学生自评项");
  });
});
