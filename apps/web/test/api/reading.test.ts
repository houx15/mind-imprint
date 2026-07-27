import { describe, it, expect, vi, afterEach } from "vitest";
import { readTurn, evaluateCardSelection } from "@/api/reading";

afterEach(() => { vi.restoreAllMocks(); });

function sseBody(frames: string): Response {
  const stream = new ReadableStream<Uint8Array>({
    start(c) { c.enqueue(new TextEncoder().encode(frames)); c.close(); },
  });
  return new Response(stream, { status: 200, headers: { "Content-Type": "text/event-stream" } });
}

describe("readTurn", () => {
  it("POSTs to .../materials/{mid}/read-turn with student_text and focused_spans, streams events", async () => {
    const spy = vi.fn(async () => sseBody(
      `event: card\ndata: {"card_instance_id":"ci1","card_id":"read_card","nudge_text":"Text analysis","anchors":[],"material_id":"m1"}\n\n` +
      `event: done\ndata: {}\n\n`,
    ));
    vi.stubGlobal("fetch", spy);

    const body = { student_text: "我的理解是……", focused_spans: [{ block_id: "b1", quote: "过去二十年……" }] };
    const events = [];
    for await (const e of readTurn("p1", "m1", body)) {
      events.push(e);
    }

    expect(events[0]).toMatchObject({ type: "card", cardInstanceId: "ci1", cardId: "read_card", materialId: "m1" });
    expect(events.at(-1)).toEqual({ type: "done" });

    const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit & { body: string }];
    expect(url).toContain("/api/v1/projects/p1/materials/m1/read-turn");
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body)).toEqual(body);
  });
});

describe("evaluateCardSelection", () => {
  it("POSTs to .../cards/{cid}/evaluate and Zod-parses SelectionEval", async () => {
    const response = {
      verdict: "strong" as const,
      verdictLabel: "Strong evidence",
      verdictReason: "Clear support",
      checks: [{ key: "source", label: "Source credibility", status: "pass" as const, evidence: "NASA", explanation: "Reliable" }],
      finding: "The evidence is solid",
      judgment: "Good selection",
      support: "Cited directly",
      caveat: "Recent data only",
      nextStep: "Compare with other sources",
      spanIds: ["s1"],
    };

    const spy = vi.fn(async () => new Response(JSON.stringify(response), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    }));
    vi.stubGlobal("fetch", spy);

    const body = { block_id: "b1", start: 0, end: 20, quote: "Some text", dimension: "evidence" };
    const result = await evaluateCardSelection("p1", "c1", body);

    expect(result.verdict).toBe("strong");
    expect(result.verdictLabel).toBe("Strong evidence");

    const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit & { body: string }];
    expect(url).toContain("/api/v1/projects/p1/cards/c1/evaluate");
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body)).toEqual(body);
  });

  it("throws on schema drift", async () => {
    const spy = vi.fn(async () => new Response(JSON.stringify({ verdict: "invalid" }), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    }));
    vi.stubGlobal("fetch", spy);

    await expect(evaluateCardSelection("p1", "c1", {
      block_id: "b1",
      start: 0,
      end: 20,
      quote: "text",
      dimension: "evidence",
    })).rejects.toThrow();
  });
});
