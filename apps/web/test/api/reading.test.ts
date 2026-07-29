import { describe, it, expect, vi, afterEach } from "vitest";
import { readTurn, evaluateCardSelection, getOpenCard, putReadingBrief, getTakeawayDraft, postFinalizeReading } from "@/api/reading";

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

describe("getOpenCard", () => {
  it("GETs .../materials/{mid}/open-card and maps snake_case to camelCase", async () => {
    const spy = vi.fn(async () => new Response(JSON.stringify({
      card_instance_id: "ci1",
      card_id: "sift",
      status: "active",
      anchors: [{ id: "a0", material_id: "m1", block_id: "b0", start: 0, end: 3, quote: "x", dimension: "sift", author: "ai", question: "为什么？", answer: "" }],
    }), { status: 200, headers: { "Content-Type": "application/json" } }));
    vi.stubGlobal("fetch", spy);

    const result = await getOpenCard("p1", "m1");

    expect(result).toEqual({
      cardInstanceId: "ci1",
      cardId: "sift",
      status: "active",
      anchors: [{ id: "a0", material_id: "m1", block_id: "b0", start: 0, end: 3, quote: "x", dimension: "sift", author: "ai", question: "为什么？", answer: "" }],
    });

    const [url] = spy.mock.calls[0] as unknown as [string];
    expect(url).toContain("/api/v1/projects/p1/materials/m1/open-card");
  });

  it("returns null when the server reports no open card", async () => {
    const spy = vi.fn(async () => new Response(JSON.stringify({
      card_instance_id: "", card_id: "", status: "", anchors: [],
    }), { status: 200, headers: { "Content-Type": "application/json" } }));
    vi.stubGlobal("fetch", spy);

    const result = await getOpenCard("p1", "m1");

    expect(result).toBeNull();
  });
});

// S2 (Task 9) wire-level tests for the 3 client fns the ReadingRoom brief/
// finalize UI drives directly (not through useReadingLoop) — guards the
// snake_case wire against silent regression, same convention as readTurn/
// evaluateCardSelection above.
describe("putReadingBrief", () => {
  it("PUTs to .../references/{rid}/reading-brief with a snake_case full-replace body", async () => {
    const spy = vi.fn(async () => new Response(JSON.stringify({
      reference: {}, readingReason: "验证碳排放反例", readingFocus: "看引用来源", phaseTag: "反例检验",
    }), { status: 200, headers: { "Content-Type": "application/json" } }));
    vi.stubGlobal("fetch", spy);

    await putReadingBrief("p1", "r1", {
      readingReason: "验证碳排放反例",
      readingFocus: "看引用来源",
      phaseTag: "反例检验",
    });

    const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit & { body: string }];
    expect(url).toContain("/api/v1/projects/p1/references/r1/reading-brief");
    expect(init.method).toBe("PUT");
    expect(JSON.parse(init.body)).toEqual({
      reading_reason: "验证碳排放反例",
      reading_focus: "看引用来源",
      phase_tag: "反例检验",
    });
  });
});

describe("getTakeawayDraft", () => {
  it("GETs .../references/{rid}/takeaway-draft and Zod-parses TakeawayDraft", async () => {
    const response = {
      record: {
        findings: ["政策目标模糊。"],
        credibility: { verdict: "中等可信", why: "官方数据但样本有限。" },
        keyQuotes: [{ quote: "植被覆盖上升。", why: "支持结论。" }],
      },
      suggestedNewLeads: ["查一下具体地区数据"],
      suggestedProposalImpact: "这篇支持我的论点，但需要补充地区细节。",
    };
    const spy = vi.fn(async () => new Response(JSON.stringify(response), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    }));
    vi.stubGlobal("fetch", spy);

    const result = await getTakeawayDraft("p1", "r1");

    expect(result).toEqual(response);
    const [url] = spy.mock.calls[0] as unknown as [string];
    expect(url).toContain("/api/v1/projects/p1/references/r1/takeaway-draft");
  });
});

describe("postFinalizeReading", () => {
  it("POSTs {new_leads, proposal_impact} and Zod-parses the returned Reference", async () => {
    const reference = {
      id: "r1",
      title: "China's carbon trajectory",
      classification: "期刊论文",
      author: "Nature Sustainability",
      credentials: "同行评议",
      year: "2024",
      url: "https://www.nature.com/",
      tags: ["气候"],
      collectionId: null,
      credibility: "strong" as const,
      evaluation: "权威且新近",
      decision: "use" as const,
      pending: false,
      searchHints: [],
      materialId: "m1",
      notes: [],
      phaseTag: "反例检验",
      readingReason: "验证碳排放反例",
      readingFocus: "看引用来源",
      takeaway: {
        findings: ["中国碳排放总量全球第一"],
        credibility: { verdict: "strong", why: "NASA 一手数据" },
        keyQuotes: [{ quote: "China emits the most", why: "直接反例" }],
        newLeads: ["核实人均口径"],
        proposalImpact: "作为让步段的反例证据",
      },
    };
    const spy = vi.fn(async () => new Response(JSON.stringify({ reference }), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    }));
    vi.stubGlobal("fetch", spy);

    const result = await postFinalizeReading("p1", "r1", {
      newLeads: ["核实人均口径"],
      proposalImpact: "作为让步段的反例证据",
    });

    expect(result.takeaway?.proposalImpact).toBe("作为让步段的反例证据");
    const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit & { body: string }];
    expect(url).toContain("/api/v1/projects/p1/references/r1/finalize-reading");
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body)).toEqual({
      new_leads: ["核实人均口径"],
      proposal_impact: "作为让步段的反例证据",
    });
  });
});
