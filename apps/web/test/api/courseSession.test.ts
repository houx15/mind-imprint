import { describe, it, expect, vi, afterEach } from "vitest";
import { startCourseSession, getCourseSession, submitCourseCard, skipCourseCard, courseAsk, courseAdvance } from "@/api/courseSession";

afterEach(() => { vi.restoreAllMocks(); });

function sseBody(frames: string): Response {
  const stream = new ReadableStream<Uint8Array>({
    start(c) { c.enqueue(new TextEncoder().encode(frames)); c.close(); },
  });
  return new Response(stream, { status: 200, headers: { "Content-Type": "text/event-stream" } });
}

const sessionFixture = {
  id: "s1", courseId: "c1", phase: "demonstrate", phaseTitle: "演示", status: "active",
  messages: [{ id: "m1", phase: "demonstrate", role: "student", content: "你好", createdAt: "2026-07-17T12:00:00Z" }],
};

describe("startCourseSession", () => {
  it("POSTs the get-or-create endpoint and parses the session", async () => {
    const spy = vi.fn(async () => new Response(
      JSON.stringify(sessionFixture),
      { status: 201, headers: { "Content-Type": "application/json" } },
    ));
    vi.stubGlobal("fetch", spy);

    const result = await startCourseSession("c1");

    const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toContain("/api/v1/courses/c1/session");
    expect(init.method).toBe("POST");
    expect(result).toEqual(sessionFixture);
  });
});

describe("getCourseSession", () => {
  it("GETs the session and parses it", async () => {
    const spy = vi.fn(async () => new Response(
      JSON.stringify(sessionFixture),
      { status: 200, headers: { "Content-Type": "application/json" } },
    ));
    vi.stubGlobal("fetch", spy);

    const result = await getCourseSession("c1");

    const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toContain("/api/v1/courses/c1/session");
    expect(init?.method ?? "GET").toBe("GET");
    expect(result).toEqual(sessionFixture);
  });
});

describe("submitCourseCard", () => {
  it("POSTs field_values/event_trace/anchors to the card submit endpoint", async () => {
    const spy = vi.fn(async () => new Response(
      JSON.stringify({ card_status: "completed" }),
      { status: 200, headers: { "Content-Type": "application/json" } },
    ));
    vi.stubGlobal("fetch", spy);

    const payload = { field_values: { q1: "a" }, event_trace: [], anchors: [] };
    await submitCourseCard("c1", "ci1", payload);

    const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toContain("/api/v1/courses/c1/session/cards/ci1/submit");
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body as string)).toEqual(payload);
  });
});

describe("skipCourseCard", () => {
  it("POSTs to the card skip endpoint with no body", async () => {
    const spy = vi.fn(async () => new Response(JSON.stringify({ card_status: "skipped" }), { status: 200 }));
    vi.stubGlobal("fetch", spy);

    await skipCourseCard("c1", "ci1");

    const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toContain("/api/v1/courses/c1/session/cards/ci1/skip");
    expect(init.method).toBe("POST");
  });
});

describe("courseAsk", () => {
  it("accumulates text deltas into one reply event", async () => {
    vi.spyOn(global, "fetch").mockResolvedValue(sseBody(
      `event: text\ndata: {"delta":"你好"}\n\n` +
      `event: text\ndata: {"delta":"，同学"}\n\n` +
      `event: done\ndata: {}\n\n`,
    ));
    const events = [];
    for await (const e of courseAsk("c1", "hi")) events.push(e);
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
    for await (const e of courseAsk("c1", "hi")) events.push(e);
    expect(events[0]).toEqual({ type: "card", cardInstanceId: "ci1", cardId: "craap", materialId: "mat1" });
  });

  it("yields an error event on a non-ok response", async () => {
    vi.spyOn(global, "fetch").mockResolvedValue(new Response(
      JSON.stringify({ error: { code: "internal_error", message: "服务器出错了" } }),
      { status: 500, headers: { "Content-Type": "application/json" } },
    ));
    const events = [];
    for await (const e of courseAsk("c1", "hi")) events.push(e);
    expect(events).toEqual([{ type: "error", code: "internal_error", message: "服务器出错了" }]);
  });
});

describe("courseAdvance", () => {
  it("posts with no body and surfaces a phase event", async () => {
    const spy = vi.fn(async () => sseBody(
      `event: phase\ndata: {"to":"guided"}\n\n` +
      `event: done\ndata: {}\n\n`,
    ));
    vi.stubGlobal("fetch", spy);

    const events = [];
    for await (const e of courseAdvance("c1")) events.push(e);
    expect(events[0]).toEqual({ type: "phase", to: "guided" });
    expect(events.at(-1)).toEqual({ type: "done" });

    const [, init] = spy.mock.calls[0] as unknown as [string, RequestInit];
    expect(init.body).toBeUndefined();
  });
});
