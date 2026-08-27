import { render, screen, waitFor, cleanup, fireEvent, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { WritingRoomHost } from "@lite/writings/WritingRoomHost";

/**
 * WritingRoomHost — driven through a stubbed `fetch`, same harness shape as
 * readingRoomHost.test.tsx. There is no pro room to mount here (see the task
 * brief's 复用边界), so these tests exercise the assembled room directly:
 * the stage map, the coach turn, each stage's own panel, and the card
 * lifecycle through the REAL `StudioCardSheet`.
 */

const WID = "w1";

type Route = { status?: number; body?: unknown };
let routes: Record<string, Route>;
let calls: { method: string; url: string; body: unknown }[];

function key(method: string, url: string) {
  return `${method} ${url}`;
}

function jsonResponse(status: number, body: unknown): Response {
  return { ok: status >= 200 && status < 300, status, json: async () => body } as Response;
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
      if (!route) return jsonResponse(404, { error: { code: "not_found", message: "资源不存在" } });
      return jsonResponse(route.status ?? 200, route.body ?? {});
    }),
  );
}

function writing(over: Partial<Record<string, unknown>> = {}) {
  return {
    id: WID,
    title: "该不该把上学时间往后推？",
    lang: "zh",
    stage: "ideate",
    targetWords: null,
    status: "active",
    createdAt: "2026-08-26T00:00:00Z",
    updatedAt: "2026-08-26T00:00:00Z",
    finishedAt: null,
    ...over,
  };
}

function base(path: string) {
  return `/api/v1/writings/${WID}${path}`;
}

function emptyRoutes(over: Partial<Record<string, unknown>> = {}): Record<string, Route> {
  return {
    [key("GET", base(""))]: { body: writing(over) },
    [key("GET", base("/messages"))]: { body: { messages: [] } },
    [key("GET", base("/outline"))]: { body: { outline: [] } },
    [key("GET", base("/snippets"))]: { body: { snippets: [] } },
    [key("GET", base("/draft"))]: { body: { body: "", updatedAt: null } },
    [key("GET", base("/cards"))]: { body: { cards: [] } },
  };
}

beforeEach(() => {
  routes = emptyRoutes();
  stubFetch();
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("loading the room", () => {
  it("restores the transcript and shows the stage map on 构思", async () => {
    routes[key("GET", base("/messages"))] = {
      body: { messages: [{ seq: 1, role: "student", content: "我想论证短视频有没有让人变笨。", createdAt: "" }] },
    };
    render(<WritingRoomHost writingId={WID} />);

    expect(await screen.findByText("我想论证短视频有没有让人变笨。")).toBeTruthy();
    expect(screen.getByRole("button", { name: /构思/ })).toHaveAttribute("aria-current", "step");
    expect(screen.getByRole("heading", { name: "构思" })).toBeTruthy();
  });

  it("shows a real error when the writing itself cannot load", async () => {
    delete routes[key("GET", base(""))];
    render(<WritingRoomHost writingId={WID} />);
    expect(await screen.findByText(/资源不存在|打不开/)).toBeTruthy();
  });

  it("opens a finished writing read-only, with no stage map or composer", async () => {
    routes[key("GET", base(""))] = { body: writing({ status: "finished", finishedAt: "2026-08-25T00:00:00Z" }) };
    routes[key("GET", base("/draft"))] = { body: { body: "这是我的成稿。", updatedAt: "2026-08-25T00:00:00Z" } };
    render(<WritingRoomHost writingId={WID} />);

    expect(await screen.findByText("已完成")).toBeTruthy();
    expect(screen.getByText("这是我的成稿。")).toBeTruthy();
    expect(screen.queryByRole("navigation", { name: "写作四步" })).toBeNull();
    expect(screen.queryByRole("textbox")).toBeNull();
    // The room's own loads are never even attempted for a finished writing.
    expect(calls.some((c) => c.url.endsWith("/outline"))).toBe(false);
  });
});

describe("the stage map — a MAP, not a gate", () => {
  it("jumps straight from 构思 to 成稿 with one click, no gating", async () => {
    routes[key("POST", base("/stage"))] = { body: writing({ stage: "draft" }) };
    render(<WritingRoomHost writingId={WID} />);
    await screen.findByRole("heading", { name: "构思" });

    fireEvent.click(screen.getByRole("button", { name: /成稿/ }));

    await waitFor(() => expect(screen.getByRole("heading", { name: "成稿" })).toBeTruthy());
    const post = calls.find((c) => c.method === "POST" && c.url === base("/stage"))!;
    expect(post.body).toEqual({ stage: "draft" });
  });
});

describe("talk first", () => {
  it("sends a coach turn and shows the reply", async () => {
    routes[key("POST", base("/turn"))] = {
      body: { reply: "你想说服谁读这段论证？", decision: "respond", card: null, nudge: "", hintCardId: null },
    };
    render(<WritingRoomHost writingId={WID} />);
    await screen.findByRole("heading", { name: "构思" });

    fireEvent.change(screen.getByPlaceholderText("想到什么，跟印记说说"), { target: { value: "我想写打工的利弊。" } });
    fireEvent.click(screen.getByLabelText("发送"));

    expect(await screen.findByText("你想说服谁读这段论证？")).toBeTruthy();
    const post = calls.find((c) => c.method === "POST" && c.url === base("/turn"))!;
    expect(post.body).toEqual({ text: "我想写打工的利弊。" });
  });

  it("surfaces a failed turn instead of a fabricated reply", async () => {
    routes[key("POST", base("/turn"))] = { status: 502, body: { error: { code: "ai_dialogue_failed", message: "印记暂时没接上，请重试。" } } };
    render(<WritingRoomHost writingId={WID} />);
    await screen.findByRole("heading", { name: "构思" });

    fireEvent.change(screen.getByPlaceholderText("想到什么，跟印记说说"), { target: { value: "在吗" } });
    fireEvent.click(screen.getByLabelText("发送"));

    expect(await screen.findByText("印记暂时没接上，请重试。")).toBeTruthy();
  });
});

describe("大纲 stage", () => {
  it("generates a candidate, lets her edit it, and confirms it", async () => {
    routes = { ...emptyRoutes({ stage: "outline" }) };
    routes[key("POST", base("/outline/generate"))] = {
      body: { outline: [{ text: "打工能带来的收获", depth: 0 }, { text: "打工的代价", depth: 0 }] },
    };
    routes[key("PUT", base("/outline"))] = {
      body: {
        outline: [
          { id: "o1", text: "打工能带来的收获", depth: 0, position: 0 },
          { id: "o2", text: "打工的代价（改）", depth: 0, position: 1 },
        ],
      },
    };
    render(<WritingRoomHost writingId={WID} />);
    await screen.findByRole("heading", { name: "大纲" });

    fireEvent.click(screen.getByRole("button", { name: "帮我拟一份候选" }));
    const second = await screen.findByDisplayValue("打工的代价");
    fireEvent.change(second, { target: { value: "打工的代价（改）" } });

    fireEvent.click(screen.getByRole("button", { name: "确认这份提纲" }));

    await waitFor(() => {
      const put = calls.find((c) => c.method === "PUT" && c.url === base("/outline"));
      expect(put?.body).toEqual({
        outline: [
          { text: "打工能带来的收获", depth: 0 },
          { text: "打工的代价（改）", depth: 0 },
        ],
      });
    });
  });
});

describe("段落 stage — the 铁律① pressure point", () => {
  it("shows guiding-question blocks and a labelled 示范 with NO insert affordance", async () => {
    routes = {
      ...emptyRoutes({ stage: "snippets", lang: "en" }),
      [key("GET", base("/outline"))]: { body: { outline: [{ id: "o1", text: "The upside", depth: 0, position: 0 }] } },
    };
    routes[key("PUT", base("/snippets"))] = {
      body: { snippets: [{ id: "s1", outlineId: "o1", outlineHeading: "The upside", position: 0, text: "", updatedAt: "" }] },
    };
    routes[key("POST", base("/snippets/s1/exemplar"))] = {
      body: { exemplar: "Working part-time teaches real accountability.", prompts: ["What is your strongest example?", "How would you counter the obvious objection?"] },
    };
    render(<WritingRoomHost writingId={WID} />);
    await screen.findByRole("heading", { name: "段落" });

    fireEvent.click(screen.getByRole("button", { name: "示范段落" }));

    expect(await screen.findByText("Working part-time teaches real accountability.")).toBeTruthy();
    expect(screen.getByText("What is your strongest example?")).toBeTruthy();
    expect(screen.getByText("How would you counter the obvious objection?")).toBeTruthy();
    expect(screen.getByText("示范")).toBeTruthy();

    // 铁律①, asserted by OUTCOME rather than by inventory.
    //
    // The obvious check — queryAllByRole("button") is empty — is weaker than
    // it looks. A <div onClick> carries no accessible role at all and an <a>
    // carries "link", so the two most natural ways someone would later add an
    // "insert this" affordance both slip straight past it. A guard that misses
    // the exact regression it exists to catch is worse than no guard, because
    // it reads as protection.
    //
    // So click EVERY node inside the exemplar box and assert her paragraph
    // textarea never changes. That holds whatever markup a future edit reaches
    // for — button, div, anchor, span with a handler — because it tests what
    // the rule actually cares about: that no path exists from the AI's English
    // into her draft.
    const exemplarText = screen.getByText("Working part-time teaches real accountability.");
    const box = exemplarText.closest("div")!;
    expect(within(box).queryAllByRole("button")).toHaveLength(0);

    const draft = screen.getByPlaceholderText("写这一段……") as HTMLTextAreaElement;
    const draftBefore = draft.value;
    for (const node of Array.from(box.querySelectorAll("*"))) {
      fireEvent.click(node);
    }
    fireEvent.click(box);
    expect(draft.value).toBe(draftBefore);
    expect(draft.value).not.toContain("Working part-time teaches real accountability.");

    // It lazily created the snippet row before asking for the exemplar.
    expect(calls.some((c) => c.method === "PUT" && c.url === base("/snippets"))).toBe(true);
  });

  it("offers no 示范 affordance at all for a Chinese writing", async () => {
    routes = {
      ...emptyRoutes({ stage: "snippets", lang: "zh" }),
      [key("GET", base("/outline"))]: { body: { outline: [{ id: "o1", text: "打工的好处", depth: 0, position: 0 }] } },
    };
    render(<WritingRoomHost writingId={WID} />);
    await screen.findByRole("heading", { name: "段落" });
    expect(screen.queryByRole("button", { name: "示范段落" })).toBeNull();
  });
});

describe("成稿 stage", () => {
  it("composes from her snippets, edits, requests feedback, and finishes", async () => {
    routes = { ...emptyRoutes({ stage: "draft" }) };
    routes[key("POST", base("/compose"))] = { body: { body: "拼合出的初稿。", updatedAt: "2026-08-26T00:00:00Z" } };
    routes[key("PUT", base("/draft"))] = { body: { body: "拼合出的初稿，改过。", updatedAt: "2026-08-26T00:01:00Z" } };
    routes[key("POST", base("/review"))] = { body: { feedback: "论证的第二段证据略薄，可以再补一个例子。" } };
    routes[key("POST", base("/finish"))] = { body: writing({ stage: "draft", status: "finished", finishedAt: "2026-08-26T01:00:00Z" }) };
    render(<WritingRoomHost writingId={WID} />);
    await screen.findByRole("heading", { name: "成稿" });

    fireEvent.click(screen.getByRole("button", { name: "从段落拼出初稿" }));
    const textarea = await screen.findByDisplayValue("拼合出的初稿。");
    fireEvent.change(textarea, { target: { value: "拼合出的初稿，改过。" } });
    fireEvent.blur(textarea);
    await waitFor(() => expect(calls.some((c) => c.method === "PUT" && c.url === base("/draft"))).toBe(true));

    fireEvent.click(screen.getByRole("button", { name: "请印记看看" }));
    expect(await screen.findByText("论证的第二段证据略薄，可以再补一个例子。")).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "完成这篇" }));
    expect(await screen.findByText("已完成")).toBeTruthy();
  });
});

describe("工具卡", () => {
  it("summons a card, opens it on her tap, fills it, and submits without clobbering its anchors", async () => {
    routes[key("POST", base("/summon"))] = {
      body: {
        reply: "",
        decision: "summon",
        nudge: "试试从「学期中打工」这条切入。",
        hintCardId: null,
        card: { id: "c1", cardId: "concession", blockId: null, status: "proposed", origin: "student", anchors: [], fieldValues: {}, eventTrace: [], framework: {}, createdAt: "", submittedAt: null },
      },
    };
    routes[key("POST", `${base("/cards")}/c1/activate`)] = {
      body: { id: "c1", cardId: "concession", blockId: null, status: "active", origin: "student", anchors: [], fieldValues: {}, eventTrace: [], framework: {}, createdAt: "", submittedAt: null },
    };
    routes[key("POST", `${base("/cards")}/c1/submit`)] = {
      body: { id: "c1", cardId: "concession", blockId: null, status: "submitted", origin: "student", anchors: [], fieldValues: {}, eventTrace: [], framework: {}, createdAt: "", submittedAt: "2026-08-26T00:00:00Z" },
    };
    render(<WritingRoomHost writingId={WID} />);
    await screen.findByRole("heading", { name: "构思" });

    fireEvent.click(await screen.findByText("让步段 · 以退为进"));
    expect(await screen.findByText(/要不要用《让步段 · 以退为进》看看/)).toBeTruthy();
    const summonCall = calls.find((c) => c.method === "POST" && c.url === base("/summon"))!;
    expect(summonCall.body).toEqual({ cardId: "concession" });

    fireEvent.click(screen.getByRole("button", { name: "打开" }));
    await screen.findByText("提交并钉到过程树");

    fireEvent.change(screen.getByLabelText("你的中心论点是什么？"), { target: { value: "学期中打工值得。" } });
    fireEvent.change(screen.getByLabelText("反方最强的那个事实是什么？"), { target: { value: "会挤占学习时间。" } });
    fireEvent.change(screen.getByLabelText("先承认它（让步）"), { target: { value: "确实会占用一些晚上的时间。" } });
    fireEvent.change(screen.getByLabelText("再转折反驳（为什么它不足以推翻你的论点）"), { target: { value: "但打工的时间是可控的，收获的责任感更持久。" } });

    fireEvent.click(screen.getByRole("button", { name: "提交并钉到过程树" }));

    await waitFor(() => expect(screen.getByText(/记下了/)).toBeTruthy());
    const submitCall = calls.find((c) => c.method === "POST" && c.url === `${base("/cards")}/c1/submit`)!;
    expect(submitCall.body).toMatchObject({
      fieldValues: {
        thesis: "学期中打工值得。",
        counter: "会挤占学习时间。",
        concede: "确实会占用一些晚上的时间。",
        rebut: "但打工的时间是可控的，收获的责任感更持久。",
      },
    });
    // No `anchors` key at all — sending one (even `[]`) would overwrite the
    // card's AI-grounded example anchor server-side.
    expect(submitCall.body).not.toHaveProperty("anchors");
  });
});
