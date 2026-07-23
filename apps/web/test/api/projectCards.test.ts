import { describe, it, expect, vi, afterEach } from "vitest";
import { activateProjectCard, submitProjectCard, skipProjectCard } from "@/api/projectCards";

afterEach(() => { vi.restoreAllMocks(); });

function sseBody(frames: string): Response {
  const stream = new ReadableStream<Uint8Array>({
    start(c) { c.enqueue(new TextEncoder().encode(frames)); c.close(); },
  });
  return new Response(stream, { status: 200, headers: { "Content-Type": "text/event-stream" } });
}

describe("activateProjectCard", () => {
  it("POSTs to .../cards/{cid}/activate", async () => {
    const spy = vi.fn(async () => new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", spy);
    await activateProjectCard("p1", "c1");
    const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toContain("/api/v1/projects/p1/cards/c1/activate");
    expect(init.method).toBe("POST");
  });
});

describe("skipProjectCard", () => {
  it("POSTs to .../cards/{cid}/skip with event_trace", async () => {
    const spy = vi.fn(async () => new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", spy);
    const eventTrace = [{ kind: "submit" as const, at: "z" }];
    await skipProjectCard("p1", "c1", { event_trace: eventTrace as never });
    const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit & { body: string }];
    expect(url).toContain("/api/v1/projects/p1/cards/c1/skip");
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body)).toMatchObject({ event_trace: eventTrace });
  });
});

describe("submitProjectCard", () => {
  it("POSTs to .../cards/{cid}/submit with field_values/event_trace/anchors and streams the refeed events", async () => {
    const spy = vi.fn(async () => sseBody(
      `event: intervention\ndata: {"intervention_id":"iid","body":"连到治理决心","anchor":"论证图 · 治理决心主张","criterion":"D5","level":"I2"}\n\n` +
      `event: done\ndata: {}\n\n`,
    ));
    vi.stubGlobal("fetch", spy);
    const events = [];
    for await (const e of submitProjectCard("p1", "c1", {
      field_values: { a: 1 },
      event_trace: [{ kind: "submit" as const, at: "z" }] as never,
      anchors: [],
    })) events.push(e);
    expect(events[0]).toMatchObject({ type: "intervention", interventionId: "iid", criterion: "D5" });
    expect(events.at(-1)).toEqual({ type: "done" });
    const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit & { body: string }];
    expect(url).toContain("/api/v1/projects/p1/cards/c1/submit");
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body)).toMatchObject({ field_values: { a: 1 } });
  });

  it("yields a card event", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => sseBody(
      `event: card\ndata: {"card_instance_id":"ci1","card_id":"craap","nudge_text":"CRAAP 五维核查","anchors":[]}\n\n` +
      `event: done\ndata: {}\n\n`)));
    const events = [];
    for await (const e of submitProjectCard("p1", "c1", { field_values: {}, event_trace: [], anchors: [] })) events.push(e);
    expect(events[0]).toMatchObject({ type: "card", cardInstanceId: "ci1", cardId: "craap", nudgeText: "CRAAP 五维核查" });
  });
});
