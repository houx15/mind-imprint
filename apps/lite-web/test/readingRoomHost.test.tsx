import { render, screen, waitFor, cleanup, fireEvent } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { StudioTurnEvent } from "@/api/studioTurn";
import { ReadingRoomHost } from "@lite/readings/ReadingRoomHost";
import { createReadingRoomApi } from "@lite/api/readingRoom";

/**
 * ReadingRoomHost — the mount point, driven through a stubbed `fetch` so the
 * assertions are about the REAL wiring (which endpoint, which id, which
 * props) rather than about a hand-rolled double.
 */

const READING_ID = "atom-1";

type Route = { status?: number; body?: unknown };
let routes: Record<string, Route>;
let calls: { method: string; url: string; body: unknown }[];

function key(method: string, url: string) {
  return `${method} ${url}`;
}

function stubFetch() {
  calls = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init?: RequestInit) => {
      const method = (init?.method ?? "GET").toUpperCase();
      const body = init?.body ? JSON.parse(init.body as string) : undefined;
      calls.push({ method, url, body });
      const route = routes[key(method, url)];
      if (!route) {
        return jsonResponse(404, { error: { code: "not_found", message: "资源不存在" } });
      }
      return jsonResponse(route.status ?? 200, route.body ?? {});
    }),
  );
}

function jsonResponse(status: number, body: unknown): Response {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: async () => body,
  } as Response;
}

const READING = {
  id: READING_ID,
  title: "中国的可持续转型",
  lang: "zh",
  status: "reading",
  hasSource: true,
  createdAt: "2026-08-26T00:00:00Z",
  updatedAt: "2026-08-26T00:00:00Z",
  finishedAt: null,
};

const SOURCE = {
  title: "中国的可持续转型",
  sourceUrl: "",
  blocks: [
    { id: "b1", text: "中国的太阳能装机量在过去十年增长了十倍。" },
    { id: "b2", text: "但同一时期，中国的碳排放总量仍居全球第一。" },
  ],
};

function sourcedRoutes(): Record<string, Route> {
  return {
    [key("GET", `/api/v1/readings/${READING_ID}`)]: { body: READING },
    [key("GET", `/api/v1/readings/${READING_ID}/source`)]: { body: SOURCE },
    [key("GET", `/api/v1/readings/${READING_ID}/brief`)]: {
      body: { phaseTag: null, readingReason: "我想弄清这篇有没有回避排放总量", readingFocus: "" },
    },
    [key("GET", `/api/v1/readings/${READING_ID}/annotations`)]: { body: { annotations: [] } },
    [key("GET", `/api/v1/readings/${READING_ID}/messages`)]: { body: { messages: [] } },
    [key("GET", `/api/v1/readings/${READING_ID}/cards`)]: { body: { cards: [] } },
  };
}

beforeEach(() => {
  routes = sourcedRoutes();
  stubFetch();
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("ReadingRoomHost", () => {
  it("mounts the real reading room on the article, with the lite surfaces only", async () => {
    render(<ReadingRoomHost readingId={READING_ID} />);

    expect(await screen.findByText("中国的太阳能装机量在过去十年增长了十倍。")).toBeTruthy();
    // The room's own chrome, not a lite re-implementation.
    expect(screen.getByText(/透镜库/)).toBeTruthy();
    expect(screen.getByRole("tab", { name: /阅读成果/ })).toBeTruthy();
    // The persisted brief seeds the banner rather than an empty template.
    expect(screen.getByText(/我想弄清这篇有没有回避排放总量/)).toBeTruthy();
    // Gated off by LITE_READING_CAPABILITIES.
    expect(screen.queryByText("证据笔记")).toBeNull();
    expect(screen.queryByRole("button", { name: "追来源" })).toBeNull();
  });

  it("restores the persisted transcript instead of greeting her again", async () => {
    routes[key("GET", `/api/v1/readings/${READING_ID}/messages`)] = {
      body: {
        messages: [
          { seq: 1, role: "student", content: "这个十倍是跟哪一年比的？", createdAt: "" },
          { seq: 2, role: "ai", content: "文章没说基准年，你打算怎么查？", createdAt: "" },
        ],
      },
    };
    render(<ReadingRoomHost readingId={READING_ID} />);

    expect(await screen.findByText("这个十倍是跟哪一年比的？")).toBeTruthy();
    expect(screen.getByText("文章没说基准年，你打算怎么查？")).toBeTruthy();
  });

  it("offers the paste box for a reading whose article never landed — and repairs THAT reading", async () => {
    // The orphan case: createReading succeeded, putReadingSource did not. The
    // reading exists and is listed in 过往的阅读, so opening it must not
    // dead-end on an error.
    delete routes[key("GET", `/api/v1/readings/${READING_ID}/source`)];
    routes[key("GET", `/api/v1/readings/${READING_ID}`)] = { body: { ...READING, hasSource: false } };
    routes[key("PUT", `/api/v1/readings/${READING_ID}/source`)] = { body: SOURCE };

    render(<ReadingRoomHost readingId={READING_ID} />);

    expect(await screen.findByText("这次阅读还没有正文")).toBeTruthy();
    // Nothing that looks like a failure.
    expect(screen.queryByText(/打不开/)).toBeNull();

    fireEvent.change(screen.getByPlaceholderText("把文章正文粘贴到这里…"), {
      target: { value: "中国的太阳能装机量在过去十年增长了十倍。" },
    });
    // Once the source is in, the reload finds it.
    routes[key("GET", `/api/v1/readings/${READING_ID}/source`)] = { body: SOURCE };
    fireEvent.click(screen.getByRole("button", { name: "开始阅读" }));

    await waitFor(() => {
      expect(calls.some((c) => c.method === "PUT" && c.url === `/api/v1/readings/${READING_ID}/source`)).toBe(true);
    });
    // The crucial half: it PUT onto the EXISTING reading and never minted a
    // second one.
    expect(calls.some((c) => c.method === "POST" && c.url === "/api/v1/readings")).toBe(false);
    expect(await screen.findByText("中国的太阳能装机量在过去十年增长了十倍。")).toBeTruthy();
  });

  it("shows a real error when the reading itself cannot be loaded", async () => {
    delete routes[key("GET", `/api/v1/readings/${READING_ID}`)];
    render(<ReadingRoomHost readingId={READING_ID} />);
    expect(await screen.findByText(/资源不存在|打不开/)).toBeTruthy();
  });
});

describe("createReadingRoomApi", () => {
  async function drain(gen: AsyncGenerator<StudioTurnEvent>): Promise<StudioTurnEvent[]> {
    const out: StudioTurnEvent[] = [];
    for await (const ev of gen) out.push(ev);
    return out;
  }

  it("addresses every call by the ATOM id, never a project id", async () => {
    routes[key("POST", `/api/v1/readings/${READING_ID}/turn`)] = {
      body: { reply: "好问题。", decision: "respond", card: null, nudge: "", hintCardId: null },
    };
    const api = createReadingRoomApi(READING_ID);
    // The pro parameter positions are kept and deliberately ignored.
    await drain(api.readTurn("project-that-does-not-exist", "material-x", { student_text: "在吗", focused_spans: [] }));
    expect(calls[0].url).toBe(`/api/v1/readings/${READING_ID}/turn`);
    expect(calls[0].body).toEqual({ text: "在吗", focusedSpans: [] });
  });

  it("turns a summon into a card event carrying the example anchor", async () => {
    const anchor = {
      id: "ex0",
      material_id: READING_ID,
      block_id: "b1",
      start: 0,
      end: 10,
      quote: "中国的太阳能装机量在过去十年增长了十倍。",
      dimension: "craap",
      author: "ai",
      question: "这句给了一个没有出处的数字。",
      answer: "",
    };
    routes[key("POST", `/api/v1/readings/${READING_ID}/summon`)] = {
      body: {
        reply: "",
        decision: "summon",
        nudge: "这句给了一个没有出处的数字。",
        hintCardId: null,
        card: { id: "card-1", cardId: "craap", blockId: "b1", status: "proposed", anchors: [anchor] },
      },
    };
    const api = createReadingRoomApi(READING_ID);
    const events = await drain(api.summonCard("p", "m", "craap"));
    const card = events.find((e) => e.type === "card");
    expect(card).toMatchObject({
      type: "card",
      cardInstanceId: "card-1",
      cardId: "craap",
      nudgeText: "这句给了一个没有出处的数字。",
      materialId: READING_ID,
    });
    // The anchor is what hangs the card under b1 and highlights the example
    // sentence — dropping it would silently move the card to paragraph 1.
    expect((card as { anchors: unknown[] }).anchors).toEqual([anchor]);
  });

  it("forwards hintCardId as the criterion so a 克制 hint stays tappable", async () => {
    routes[key("POST", `/api/v1/readings/${READING_ID}/turn`)] = {
      body: {
        reply: "这个数字的出处值得查一下。",
        decision: "hint",
        card: null,
        nudge: "",
        hintCardId: "craap",
      },
    };
    const api = createReadingRoomApi(READING_ID);
    const events = await drain(api.readTurn("p", "m", { student_text: "x", focused_spans: [] }));
    expect(events[0]).toMatchObject({ type: "intervention", level: "hint", criterion: "craap" });
  });

  it("translates the lite 'submitted' status into the loop's 'completed'", async () => {
    routes[key("POST", `/api/v1/readings/${READING_ID}/cards/card-1/submit`)] = {
      body: { id: "card-1", cardId: "craap", status: "submitted", anchors: [] },
    };
    const api = createReadingRoomApi(READING_ID);
    const events = await drain(
      api.submitProjectCard("p", "card-1", { field_values: {}, event_trace: [], anchors: [] }),
    );
    // The loop retires the card ONLY on "completed" — an untranslated
    // "submitted" would leave the card mounted forever.
    expect(events).toEqual([{ type: "done", cardStatus: "completed" }]);
  });

  it("surfaces an AI failure instead of masking it as a coach reply", async () => {
    routes[key("POST", `/api/v1/readings/${READING_ID}/turn`)] = {
      status: 502,
      body: { error: { code: "ai_dialogue_failed", message: "AI 暂时没接上，请重试。" } },
    };
    const seen: string[] = [];
    const api = createReadingRoomApi(READING_ID, { onAiError: (m) => seen.push(m) });
    const events = await drain(api.readTurn("p", "m", { student_text: "x", focused_spans: [] }));
    expect(seen).toEqual(["AI 暂时没接上，请重试。"]);
    // Never an `intervention` — a fabricated sentence would read as a real
    // answer and she would keep talking to nothing.
    expect(events.some((e) => e.type === "intervention")).toBe(false);
    expect(events[0]).toMatchObject({ type: "error", code: "ai_dialogue_failed" });
  });

  it("resumes an open card after a reload, from the card list", async () => {
    routes[key("GET", `/api/v1/readings/${READING_ID}/cards`)] = {
      body: {
        cards: [
          { id: "c-old", cardId: "craap", status: "submitted", anchors: [] },
          { id: "c-open", cardId: "sift", status: "active", anchors: [{ block_id: "b2" }] },
        ],
      },
    };
    const api = createReadingRoomApi(READING_ID);
    const open = await api.getOpenCard!("p", "m");
    expect(open).toMatchObject({ cardInstanceId: "c-open", cardId: "sift", status: "active" });
  });

  it("writes her 收获 before finishing, because finish refuses an empty takeaway", async () => {
    routes[key("PUT", `/api/v1/readings/${READING_ID}/takeaway`)] = { body: { text: "总量与人均是两件事。" } };
    routes[key("POST", `/api/v1/readings/${READING_ID}/finish`)] = { body: { ...READING, status: "finished" } };
    const api = createReadingRoomApi(READING_ID);
    await api.postFinalizeReading("p", "r", { newLeads: [], proposalImpact: "总量与人均是两件事。" });
    expect(calls.map((c) => `${c.method} ${c.url}`)).toEqual([
      `PUT /api/v1/readings/${READING_ID}/takeaway`,
      `POST /api/v1/readings/${READING_ID}/finish`,
    ]);
  });

  it("assembles the takeaway draft from her own confirmed cards, with no model call", async () => {
    routes[key("GET", `/api/v1/readings/${READING_ID}/cards`)] = {
      body: {
        cards: [
          {
            id: "c1",
            cardId: "craap",
            status: "submitted",
            anchors: [{ quote: "碳排放总量仍居全球第一" }],
            framework: { finding: "这句给出了与全文乐观基调相反的事实。" },
          },
          { id: "c2", cardId: "sift", status: "skipped", anchors: [], framework: {} },
        ],
      },
    };
    routes[key("GET", `/api/v1/readings/${READING_ID}/takeaway`)] = { body: { text: "先记到这里。" } };
    const api = createReadingRoomApi(READING_ID);
    const draft = await api.getTakeawayDraft("p", "r");
    expect(draft.record.findings).toEqual(["这句给出了与全文乐观基调相反的事实。"]);
    expect(draft.record.keyQuotes).toEqual([
      { quote: "碳排放总量仍居全球第一", why: "这句给出了与全文乐观基调相反的事实。" },
    ]);
    // No verdict is invented on her behalf, and lite has no proposal to feed.
    expect(draft.record.credibility).toEqual({ verdict: "", why: "" });
    expect(draft.suggestedNewLeads).toEqual([]);
    expect(draft.suggestedProposalImpact).toBe("先记到这里。");
  });
});
