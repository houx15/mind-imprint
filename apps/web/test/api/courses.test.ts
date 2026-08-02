import { describe, it, expect, vi, afterEach } from "vitest";
import { listCourses, getCourse, getCourseProgress, saveCourseProgress, answerCourseQuiz, getCourseReport, courseAsk } from "@/api/courses";

afterEach(() => { vi.restoreAllMocks(); });

function sseBody(frames: string): Response {
  const stream = new ReadableStream<Uint8Array>({
    start(c) { c.enqueue(new TextEncoder().encode(frames)); c.close(); },
  });
  return new Response(stream, { status: 200, headers: { "Content-Type": "text/event-stream" } });
}

describe("listCourses", () => {
  it("GETs /api/v1/courses and unwraps the courses array", async () => {
    const courses = [{ slug: "sustainability", branch: "reading", title: "可持续", blurb: "…", time_label: "10 分钟", card_ids: ["craap"], step_count: 3 }];
    const spy = vi.fn(async () => new Response(
      JSON.stringify({ courses }),
      { status: 200, headers: { "Content-Type": "application/json" } },
    ));
    vi.stubGlobal("fetch", spy);

    const result = await listCourses();

    const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toContain("/api/v1/courses");
    expect(init?.method ?? "GET").toBe("GET");
    expect(result).toEqual(courses);
  });
});

describe("getCourse", () => {
  it("GETs /api/v1/courses/{slug} and unwraps the course payload", async () => {
    const course = { slug: "sustainability", title: "可持续", branch: "reading", cardIds: ["craap"], structure: { id: "s1", steps: [] }, renderCache: {} };
    const spy = vi.fn(async () => new Response(
      JSON.stringify({ course }),
      { status: 200, headers: { "Content-Type": "application/json" } },
    ));
    vi.stubGlobal("fetch", spy);

    const result = await getCourse("sustainability");

    const [url] = spy.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toContain("/api/v1/courses/sustainability");
    expect(result).toEqual(course);
  });
});

describe("getCourseProgress", () => {
  it("GETs the progress endpoint and unwraps it", async () => {
    const progress = { course_slug: "sustainability", current_ordinal: 1, completed_ordinals: [0], started_at: null, completed_at: null, updated_at: "2026-08-01T00:00:00Z" };
    const spy = vi.fn(async () => new Response(
      JSON.stringify({ progress }),
      { status: 200, headers: { "Content-Type": "application/json" } },
    ));
    vi.stubGlobal("fetch", spy);

    const result = await getCourseProgress("sustainability");

    const [url] = spy.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toContain("/api/v1/courses/sustainability/progress");
    expect(result).toEqual(progress);
  });
});

describe("saveCourseProgress", () => {
  it("PUTs current_ordinal and unwraps the returned progress", async () => {
    const progress = { course_slug: "sustainability", current_ordinal: 2, completed_ordinals: [0, 1], started_at: null, completed_at: null, updated_at: "2026-08-01T00:00:00Z" };
    const spy = vi.fn(async () => new Response(
      JSON.stringify({ progress }),
      { status: 200, headers: { "Content-Type": "application/json" } },
    ));
    vi.stubGlobal("fetch", spy);

    const result = await saveCourseProgress("sustainability", { current_ordinal: 2 });

    const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toContain("/api/v1/courses/sustainability/progress");
    expect(init.method).toBe("PUT");
    expect(JSON.parse(init.body as string)).toEqual({ current_ordinal: 2 });
    expect(result).toEqual(progress);
  });
});

describe("answerCourseQuiz", () => {
  it("POSTs stepId/interactionId/selected/correct to the quiz-answer endpoint", async () => {
    const spy = vi.fn(async () => new Response(JSON.stringify({ ok: true }), { status: 200 }));
    vi.stubGlobal("fetch", spy);

    const body = { stepId: "step1", interactionId: "q1", selected: ["a"], correct: true };
    await answerCourseQuiz("sustainability", body);

    const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toContain("/api/v1/courses/sustainability/quiz-answer");
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body as string)).toEqual(body);
  });
});

describe("getCourseReport", () => {
  it("GETs the report endpoint and unwraps it", async () => {
    const report = { title: "可持续", goal: "…", teaching_thread: "…", completedStepTitles: ["step1"], cardIds: ["craap"], secondsSpent: 120, quiz: { total: 2, correct: 1 } };
    const spy = vi.fn(async () => new Response(
      JSON.stringify({ report }),
      { status: 200, headers: { "Content-Type": "application/json" } },
    ));
    vi.stubGlobal("fetch", spy);

    const result = await getCourseReport("sustainability");

    const [url] = spy.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toContain("/api/v1/courses/sustainability/report");
    expect(result).toEqual(report);
  });
});

describe("courseAsk", () => {
  it("POSTs input/ordinal and accumulates text deltas into one reply event", async () => {
    const spy = vi.fn(async () => sseBody(
      `event: text\ndata: {"delta":"你好"}\n\n` +
      `event: text\ndata: {"delta":"，同学"}\n\n` +
      `event: done\ndata: {}\n\n`,
    ));
    vi.stubGlobal("fetch", spy);

    const events = [];
    for await (const e of courseAsk("sustainability", "这个来源可信吗", 1)) events.push(e);

    expect(events).toEqual([
      { type: "reply", body: "你好" },
      { type: "reply", body: "你好，同学" },
    ]);

    const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toContain("/api/v1/courses/sustainability/ask");
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body as string)).toEqual({ input: "这个来源可信吗", ordinal: 1 });
  });

  it("maps an error frame to an error event", async () => {
    vi.spyOn(global, "fetch").mockResolvedValue(sseBody(
      `event: error\ndata: {"error":{"code":"internal_error","message":"出错了"}}\n\n`,
    ));

    const events = [];
    for await (const e of courseAsk("sustainability", "hi", 0)) events.push(e);

    expect(events).toEqual([{ type: "error", code: "internal_error", message: "出错了" }]);
  });

  it("yields an error event on a non-ok response", async () => {
    vi.spyOn(global, "fetch").mockResolvedValue(new Response(
      JSON.stringify({ error: { code: "internal_error", message: "服务器出错了" } }),
      { status: 500, headers: { "Content-Type": "application/json" } },
    ));

    const events = [];
    for await (const e of courseAsk("sustainability", "hi", 0)) events.push(e);

    expect(events).toEqual([{ type: "error", code: "internal_error", message: "服务器出错了" }]);
  });
});
