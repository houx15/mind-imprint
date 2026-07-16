import { describe, it, expect, vi, afterEach } from "vitest";
import { listThreads, createThread, getMessages, submitChatCard, skipChatCard, chatTurn } from "./chat";

afterEach(() => { vi.restoreAllMocks(); });

function sseBody(frames: string): Response {
  const stream = new ReadableStream<Uint8Array>({
    start(c) { c.enqueue(new TextEncoder().encode(frames)); c.close(); },
  });
  return new Response(stream, { status: 200, headers: { "Content-Type": "text/event-stream" } });
}

describe("listThreads", () => {
  it("GETs the bare array of threads and parses it", async () => {
    const threads = [{ id: "t1", title: "第一个话题", createdAt: "2026-07-13T12:00:00Z" }];
    const spy = vi.fn(async () => new Response(
      JSON.stringify(threads),
      { status: 200, headers: { "Content-Type": "application/json" } },
    ));
    vi.stubGlobal("fetch", spy);

    const result = await listThreads();

    const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toContain("/api/v1/chat/threads");
    expect(init?.method ?? "GET").toBe("GET");
    expect(result).toEqual(threads);
  });
});

describe("createThread", () => {
  it("POSTs a title and parses the created thread", async () => {
    const thread = { id: "t2", title: "新话题", createdAt: "2026-07-13T12:00:00Z" };
    const spy = vi.fn(async () => new Response(
      JSON.stringify(thread),
      { status: 200, headers: { "Content-Type": "application/json" } },
    ));
    vi.stubGlobal("fetch", spy);

    const result = await createThread("新话题");

    const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toContain("/api/v1/chat/threads");
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body as string)).toEqual({ title: "新话题" });
    expect(result).toEqual(thread);
  });
});

describe("getMessages", () => {
  it("GETs the bare array of messages for a thread", async () => {
    const messages = [{ id: "m1", role: "user", content: "你好", modality: "text", createdAt: "2026-07-13T12:00:00Z" }];
    const spy = vi.fn(async () => new Response(
      JSON.stringify(messages),
      { status: 200, headers: { "Content-Type": "application/json" } },
    ));
    vi.stubGlobal("fetch", spy);

    const result = await getMessages("t1");

    const [url] = spy.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toContain("/api/v1/chat/threads/t1/messages");
    expect(result).toEqual(messages);
  });
});

describe("submitChatCard", () => {
  it("POSTs field_values/event_trace/anchors to the card submit endpoint", async () => {
    const spy = vi.fn(async () => new Response(
      JSON.stringify({ card_status: "completed" }),
      { status: 200, headers: { "Content-Type": "application/json" } },
    ));
    vi.stubGlobal("fetch", spy);

    const payload = { field_values: { q1: "a" }, event_trace: [], anchors: [] };
    await submitChatCard("t1", "ci1", payload);

    const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toContain("/api/v1/chat/threads/t1/cards/ci1/submit");
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body as string)).toEqual(payload);
  });
});

describe("skipChatCard", () => {
  it("POSTs to the card skip endpoint with no body", async () => {
    const spy = vi.fn(async () => new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", spy);

    await skipChatCard("t1", "ci1");

    const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toContain("/api/v1/chat/threads/t1/cards/ci1/skip");
    expect(init.method).toBe("POST");
  });
});

describe("chatTurn", () => {
  it("maps a text frame to a reply event accumulating the delta", async () => {
    vi.spyOn(global, "fetch").mockResolvedValue(sseBody(
      `event: text\ndata: {"delta":"你好"}\n\n` +
      `event: text\ndata: {"delta":"，同学"}\n\n` +
      `event: done\ndata: {}\n\n`,
    ));
    const events = [];
    for await (const e of chatTurn("t1", "hi")) events.push(e);
    expect(events[0]).toEqual({ type: "reply", body: "你好" });
    expect(events[1]).toEqual({ type: "reply", body: "你好，同学" });
    expect(events.at(-1)).toEqual({ type: "done" });
  });

  it("maps a card frame to a card event", async () => {
    vi.spyOn(global, "fetch").mockResolvedValue(sseBody(
      `event: card\ndata: {"card_instance_id":"ci1","card_id":"craap","material_id":"mat1"}\n\n` +
      `event: done\ndata: {}\n\n`,
    ));
    const events = [];
    for await (const e of chatTurn("t1", "hi")) events.push(e);
    expect(events[0]).toEqual({ type: "card", cardInstanceId: "ci1", cardId: "craap", materialId: "mat1" });
  });

  it("maps a done frame to a done event", async () => {
    vi.spyOn(global, "fetch").mockResolvedValue(sseBody(`event: done\ndata: {}\n\n`));
    const events = [];
    for await (const e of chatTurn("t1", "hi")) events.push(e);
    expect(events).toEqual([{ type: "done" }]);
  });

  it("yields an error event on a non-ok response", async () => {
    vi.spyOn(global, "fetch").mockResolvedValue(new Response(
      JSON.stringify({ error: { code: "internal_error", message: "服务器出错了" } }),
      { status: 500, headers: { "Content-Type": "application/json" } },
    ));
    const events = [];
    for await (const e of chatTurn("t1", "hi")) events.push(e);
    expect(events).toEqual([{ type: "error", code: "internal_error", message: "服务器出错了" }]);
  });
});
