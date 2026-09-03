import { render, screen, waitFor, cleanup, fireEvent } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { StudioTurnEvent } from "@/api/studioTurn";
import { ReadingRoomHost } from "@lite/readings/ReadingRoomHost";
import { createReadingRoomApi, putReadingRating } from "@lite/api/readingRoom";

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
  lastActivityAt: "2026-08-26T00:00:00Z",
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
    // The room's own chrome — rendered by lite's own ReadingRoom since the fork.
    expect(screen.getByRole("tab", { name: /阅读成果/ })).toBeTruthy();
    // ONE 印记. The room's coach column carries the 带读 invitation, and the
    // room's own chat log and composer are not ALSO on the page — two AI chat
    // boxes side by side is what this replaced.
    expect(screen.getByText("让我来带你详细读一遍这篇文章。")).toBeTruthy();
    expect(screen.queryByPlaceholderText(/说说你对哪一句有疑问/)).toBeNull();
    expect(screen.queryByRole("button", { name: "这条来源可信吗？" })).toBeNull();
    // 「这篇用在哪个阶段」 names PROJECT phases; a lite reading has no project.
    expect(screen.queryByLabelText("这篇材料用在哪个阶段")).toBeNull();
    // 「你读这篇是为了」 was write-only in lite — it fed only the room
    // composer's own readTurn prompt, which 带读 replaced — so lite's own room
    // never carried it over. (Pro's bar is untouched; it is live there.)
    expect(screen.queryByText(/你读这篇是为了/)).toBeNull();
    // …and with nothing rendering it, the host no longer FETCHES it either.
    // The brief outlived its bar by one commit as a round trip on every open
    // whose answer nothing read; this is what keeps it dead.
    expect(calls.filter((c) => c.url.endsWith("/brief"))).toHaveLength(0);
    // Never rendered by lite's room at all — these were `caps` branches
    // before it forked away from pro's.
    expect(screen.queryByText("证据笔记")).toBeNull();
    expect(screen.queryByRole("button", { name: "追来源" })).toBeNull();
  });

  it("shows where she is on BOTH step surfaces, and they agree", async () => {
    // Two surfaces on purpose, with different jobs: the DIAL is the plan (every
    // step, folded into a corner, one hover away) and the ROW at the top of
    // 印记's column is the present tense (the one step she is on, always
    // visible). What must never happen is the two pointing at different steps —
    // they derive from the same first-`pending` rule, and this is what holds
    // that together.
    routes[key("GET", `/api/v1/readings/${READING_ID}/plan`)] = {
      body: {
        routineKey: "close_read",
        routineName: "精读",
        tasks: [
          { id: "t1", position: 1, kind: "read", label: "先通读一遍", detail: "", blockId: "", status: "done", completedAt: "2026-08-29T00:00:00Z" },
          { id: "t2", position: 2, kind: "lens", label: "找出作者最想让你信的那一句", detail: "", blockId: "", status: "pending", completedAt: null },
          { id: "t3", position: 3, kind: "reflect", label: "这个证据够吗？", detail: "", blockId: "", status: "pending", completedAt: null },
        ],
      },
    };

    render(<ReadingRoomHost readingId={READING_ID} />);

    // The ROW, in the coach column, without touching anything: the step's own
    // label so she knows what 印记 is asking of her right now.
    const row = await screen.findByText("第 2 步 / 共 3 步");
    expect(row.closest(".mk-reading-room__coach")).toBeTruthy();
    expect(screen.getByText("找出作者最想让你信的那一句")).toBeTruthy();

    // The DIAL, folded: the same position, carried as the disc's accessible
    // name — the numbers are on screen as a shape (2/3 inside the ring) and as
    // a sentence to anyone who cannot see the shape.
    const disc = screen.getByRole("button", { name: "带读进度 · 第 2 步 / 共 3 步" });
    expect(disc.closest("aside")).toBeNull();
    expect(disc.closest(".mk-reading-room")).toBeTruthy();

    // Expanded, the whole plan.
    // `pointerover`, not `pointerenter`: React synthesizes enter/leave from
    // the over/out pair and never subscribes to the non-bubbling events.
    fireEvent.pointerOver(disc.parentElement!);
    // Two now — the row and the panel — and that is the point: both say the
    // same sentence because both derive it the same way.
    expect(screen.getAllByText("第 2 步 / 共 3 步")).toHaveLength(2);
    expect(screen.getAllByText("这个证据够吗？")).toHaveLength(1);
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

  it("restores her confirmed 阅读成果 after a reload, from the persisted cards", async () => {
    // Everything below was already in the database — the sentence she picked
    // (anchors) and the review that produced the finding (framework). Without
    // rebuilding them the tab resets to 0 and the article highlight vanishes,
    // which reads as "my work is gone".
    routes[key("GET", `/api/v1/readings/${READING_ID}/cards`)] = {
      body: {
        cards: [
          {
            id: "c1",
            cardId: "craap",
            status: "submitted",
            anchors: [
              {
                id: "sel0",
                material_id: READING_ID,
                block_id: "b2",
                start: 0,
                end: 8,
                quote: "但同一时期",
                dimension: "craap",
                author: "student",
                question: "",
                answer: "",
              },
            ],
            framework: {
              verdict: "strong",
              verdictLabel: "高度匹配",
              verdictReason: "",
              checks: [],
              finding: "这句给出了与全文乐观基调相反的事实。",
              judgment: "",
              support: "",
              caveat: "",
              nextStep: "",
              spanIds: ["sel0"],
            },
          },
          // A skipped card has no finding — it must not become a half-outcome.
          { id: "c2", cardId: "sift", status: "skipped", anchors: [], framework: {} },
        ],
      },
    };
    render(<ReadingRoomHost readingId={READING_ID} />);

    expect(await screen.findByRole("tab", { name: /阅读成果 1/ })).toBeTruthy();
  });

  // Fix round 1: onCardSummoned used to bump reloadKey, which set
  // state.phase to "loading" SYNCHRONOUSLY — wiping the whole screen (article,
  // plan rail, conversation) back to "正在打开这次阅读…" at the exact moment
  // 带读 hands her a lens, then remounting everything from scratch. This test
  // proves that no longer happens: the room stays mounted (no loading text, the
  // article never disappears, no full reload of /source), and the newly
  // summoned lens still surfaces — via a narrow re-check (GET /cards), not a
  // room-wide reload.
  it("keeps the room mounted — no reload, no torn-down screen — when 带读 hands her a lens mid-conversation", async () => {
    const cardsUrl = `/api/v1/readings/${READING_ID}/cards`;
    const sourceUrl = `/api/v1/readings/${READING_ID}/source`;
    const coachUrl = `/api/v1/readings/${READING_ID}/coach`;

    render(<ReadingRoomHost readingId={READING_ID} />);
    expect(await screen.findByText("中国的太阳能装机量在过去十年增长了十倍。")).toBeTruthy();

    const sourceCallsBefore = calls.filter((c) => c.method === "GET" && c.url === sourceUrl).length;

    // The coach turn's own lens summon just persisted a card row
    // server-side — mirrored here by making the NEXT /cards read see it,
    // same as a real POST would have.
    routes[key("GET", cardsUrl)] = {
      body: {
        cards: [
          {
            id: "card-1",
            cardId: "craap",
            status: "proposed",
            anchors: [
              {
                id: "ex0",
                material_id: READING_ID,
                block_id: "b1",
                start: 0,
                end: 5,
                quote: "中国的",
                dimension: "craap",
                author: "ai",
                question: "这句缺一个出处。",
                answer: "",
              },
            ],
          },
        ],
      },
    };
    routes[key("POST", coachUrl)] = {
      body: {
        reply: "试着自己做一遍：这一句站得住吗？",
        tasks: [],
        currentTaskId: "",
        focusBlock: "",
        tool: "",
        finished: false,
        card: { id: "card-1", cardId: "craap", blockId: "b1", status: "proposed", anchors: [] },
        nudge: "这句话缺一个出处。",
      },
    };

    fireEvent.click(screen.getByRole("button", { name: "开始" }));

    expect(await screen.findByText("试着自己做一遍：这一句站得住吗？")).toBeTruthy();

    // The room never wiped and reloaded. (b2's text, untouched by the new
    // lens's example highlight, stays a single readable text node — b1's own
    // text is now legitimately split by a <mark>, which is the mechanism
    // working, not the room disappearing.)
    expect(screen.queryByText("正在打开这次阅读…")).toBeNull();
    expect(screen.getByText("但同一时期，中国的碳排放总量仍居全球第一。")).toBeTruthy();
    const sourceCallsAfter = calls.filter((c) => c.method === "GET" && c.url === sourceUrl).length;
    expect(sourceCallsAfter).toBe(sourceCallsBefore);

    // And the mechanism actually works: the newly-summoned lens surfaces,
    // fetched through the narrow re-check rather than a reload.
    expect(await screen.findByText("看懂示范，开始选句")).toBeTruthy();
  });

  it("shows a real error when the reading itself cannot be loaded", async () => {
    delete routes[key("GET", `/api/v1/readings/${READING_ID}`)];
    render(<ReadingRoomHost readingId={READING_ID} />);
    expect(await screen.findByText(/资源不存在|打不开/)).toBeTruthy();
  });

  // A finished reading is TERMINAL. Before this, /readings/:id mounted the
  // live room whatever the status was, so she could summon fresh lenses on a
  // reading she had already finished and finish it again.
  it("opens a finished reading read-only, never the live room", async () => {
    routes[key("GET", `/api/v1/readings/${READING_ID}`)] = {
      body: { ...READING, status: "finished", finishedAt: "2026-08-25T00:00:00Z" },
    };
    routes[key("GET", `/api/v1/readings/${READING_ID}/takeaway`)] = {
      body: { text: "作者把「装机量」当成了「实际发电量」。", updatedAt: "2026-08-25T00:00:00Z" },
    };
    render(<ReadingRoomHost readingId={READING_ID} />);

    expect(await screen.findByText("已完成")).toBeTruthy();
    // `find`, not `get`: 我的收获 is no longer rendered by the panel itself. It
    // is `ReportPanel`'s `fallback`, shown only once the report fetch has
    // resolved to "no report" — so it lands a tick after 已完成, and a
    // synchronous get here raced that resolution.
    expect(await screen.findByText("作者把「装机量」当成了「实际发电量」。")).toBeTruthy();
    // Nothing that could change the reading is on the page.
    expect(screen.queryByRole("tab")).toBeNull();
    expect(screen.queryByText(/完成这次阅读/)).toBeNull();
    // And the room's own loads are never even attempted.
    expect(calls.some((c) => c.url.endsWith("/source"))).toBe(false);
  });

  // The drawer's 「看报告」 label (ReadingHistoryPanel.tsx) routes to this
  // same finished panel — this pins the half of that promise the routing
  // test can't: that the panel she lands on actually mounts the report, not
  // the old 「报告还在路上」 placeholder.
  it("mounts the report panel on a finished reading — 看报告 is no longer a lie", async () => {
    routes[key("GET", `/api/v1/readings/${READING_ID}`)] = {
      body: { ...READING, status: "finished", finishedAt: "2026-08-25T00:00:00Z" },
    };
    routes[key("GET", `/api/v1/readings/${READING_ID}/takeaway`)] = {
      body: { text: "作者把「装机量」当成了「实际发电量」。", updatedAt: "2026-08-25T00:00:00Z" },
    };
    routes[key("GET", `/api/v1/readings/${READING_ID}/report`)] = {
      body: {
        report: {
          version: 1,
          kind: "reading",
          title: READING.title,
          studentName: "Phoebe",
          finishedAt: "2026-08-25T00:00:00Z",
          stats: [{ key: "focusMinutes", label: "专注时长", value: 9, unit: "分钟" }],
          moments: [],
          keep: null,
          gains: [],
        },
      },
    };
    render(<ReadingRoomHost readingId={READING_ID} />);

    expect(await screen.findByText("阅读时长")).toBeTruthy();
    expect(screen.getByText("9")).toBeTruthy();
    expect(screen.queryByText(/报告还在路上/)).toBeNull();
  });

  it("still opens a finished reading when its takeaway cannot be read", async () => {
    routes[key("GET", `/api/v1/readings/${READING_ID}`)] = {
      body: { ...READING, status: "finished", finishedAt: "2026-08-25T00:00:00Z" },
    };
    render(<ReadingRoomHost readingId={READING_ID} />); // no takeaway route → 404
    expect(await screen.findByText("已完成")).toBeTruthy();
    // A missing takeaway is now the NORMAL case, not a degraded one: 完成这篇
    // stopped asking for one. So the panel says nothing about it rather than
    // reporting 「这次阅读没有留下收获记录」 — that line described a form we no
    // longer ask her to fill, and it read as something having gone wrong.
    expect(screen.queryByText("我的收获")).toBeNull();
    expect(screen.queryByText(/没有留下收获记录/)).toBeNull();
    expect(screen.getByText(READING.title)).toBeTruthy();
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
    expect(calls[0]?.url).toBe(`/api/v1/readings/${READING_ID}/turn`);
    expect(calls[0]?.body).toEqual({ text: "在吗", focusedSpans: [] });
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
    // 服务端自己那句话原样带出来，外面裹一层「后台错误：」——产品负责人
    // 2026-09-02：所有接口错误都照实说。
    expect(seen).toEqual(["后台错误：AI 暂时没接上，请重试。"]);
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

  it("完成这篇 is ONE call now — no takeaway PUT, because there is no form", async () => {
    routes[key("POST", `/api/v1/readings/${READING_ID}/finish`)] = { body: { ...READING, status: "finished" } };
    const api = createReadingRoomApi(READING_ID);
    await api.finishReading();
    // It used to PUT the takeaway she had just typed into a finalize form and
    // only then finish, because finish refused an empty takeaway. The form and
    // the gate are both gone; a stray PUT here would be writing a field
    // nothing asked her for.
    expect(calls.map((c) => `${c.method} ${c.url}`)).toEqual([
      `POST /api/v1/readings/${READING_ID}/finish`,
    ]);
  });

  it("她给这次体验打的星，走她自己的端点", async () => {
    routes[key("PUT", `/api/v1/readings/${READING_ID}/rating`)] = { body: { rating: 4 } };
    expect(await putReadingRating(READING_ID, 4)).toBe(4);
    expect(calls[0]).toMatchObject({
      method: "PUT",
      url: `/api/v1/readings/${READING_ID}/rating`,
      body: { rating: 4 },
    });
  });
});
