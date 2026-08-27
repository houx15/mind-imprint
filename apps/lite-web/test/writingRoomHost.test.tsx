import { render, screen, waitFor, cleanup, fireEvent, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { WritingRoomHost } from "@lite/writings/WritingRoomHost";

/**
 * WritingRoomHost — driven through a stubbed `fetch`, same harness shape as
 * readingRoomHost.test.tsx.
 *
 * The room has two faces since 2026-08-27, and these tests split the same way:
 *   - `stage: "outline"` (结构) is a FULL-SCREEN planning conversation with a
 *     mind map. No stage bar, no length counter — just thinking.
 *   - every other stage is the room: stage map, length counter, coach rail.
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
    // 结构 — where a new writing genuinely starts (the 0100 column default).
    stage: "outline",
    targetWords: null,
    structureKey: "",
    // Non-null so the setup dialog does NOT gate these tests. Its own
    // behaviour is covered by "the setup dialog", which sets it null.
    setupAt: "2026-08-26T00:00:00Z",
    status: "active",
    createdAt: "2026-08-26T00:00:00Z",
    updatedAt: "2026-08-26T00:00:00Z",
    finishedAt: null,
    ...over,
  };
}

/** The room proper begins at 段落 — 结构 is the full-screen planning view. */
const inRoom = (over: Partial<Record<string, unknown>> = {}) => ({ stage: "snippets", ...over });

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
    // 印记 speaks first, in both faces of the room.
    [key("POST", base("/opening"))]: { body: { reply: "先说说你自己更倾向哪一边？", generated: true } },
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

// ---------------------------------------------------------------------------
// 结构 — the planning conversation
// ---------------------------------------------------------------------------

describe("结构 — a planning conversation, not a template to fill", () => {
  /**
   * The regression this describe prevents, in two layers. The room first
   * shipped a 「帮我拟一份候选」 button that sent her transcript to the model
   * and got a finished outline back. That was replaced by a picker of fixed
   * skeletons with block names like 「你承认它哪一部分是对的」 — rigid,
   * unreadable for a middle-schooler, and still a form. Both are gone.
   */
  it("shows no outline generator and no template picker", async () => {
    render(<WritingRoomHost writingId={WID} />);
    await screen.findByPlaceholderText("说说你的想法");

    expect(screen.queryByRole("button", { name: "帮我拟一份候选" })).toBeNull();
    expect(screen.queryByRole("button", { name: "帮我挑一副" })).toBeNull();
    expect(screen.queryByText("全部结构")).toBeNull();
    expect(screen.queryByText("你承认它哪一部分是对的")).toBeNull();
    expect(calls.some((c) => c.url.includes("/outline/generate"))).toBe(false);
    expect(calls.some((c) => c.url.includes("/structure"))).toBe(false);
  });

  it("takes the whole screen: no stage bar and no length counter while planning", async () => {
    routes = { ...emptyRoutes({ targetWords: 800 }) };
    render(<WritingRoomHost writingId={WID} />);
    await screen.findByPlaceholderText("说说你的想法");

    expect(screen.queryByRole("navigation", { name: "写作三步" })).toBeNull();
    expect(screen.queryByText(/已写/)).toBeNull();
  });

  it("印记 opens the conversation without her saying anything first", async () => {
    render(<WritingRoomHost writingId={WID} />);
    expect(await screen.findByText("先说说你自己更倾向哪一边？")).toBeTruthy();
    expect(calls.some((c) => c.method === "POST" && c.url === base("/opening"))).toBe(true);
  });

  it("grows the map from what she says, and only shows the panel once it has something", async () => {
    routes[key("POST", base("/plan/turn"))] = {
      body: {
        reply: "你打算用哪几件事来说明？",
        outline: [{ id: "n1", text: "不该一刀切禁手机", role: "中心论点", depth: 0, position: 0 }],
        addedIds: ["n1"],
      },
    };
    render(<WritingRoomHost writingId={WID} />);
    await screen.findByPlaceholderText("说说你的想法");

    // Before she speaks the map has nothing on it, so the panel is not there
    // at all — an empty panel would be a promise the screen hasn't kept.
    expect(screen.queryByText("你的思路")).toBeNull();

    fireEvent.change(screen.getByPlaceholderText("说说你的想法"), {
      target: { value: "我觉得不该一刀切禁手机。" },
    });
    fireEvent.click(screen.getByLabelText("发送"));

    expect(await screen.findByText("你打算用哪几件事来说明？")).toBeTruthy();
    expect(await screen.findByText("你的思路")).toBeTruthy();
    expect(screen.getByText("不该一刀切禁手机")).toBeTruthy();
    expect(screen.getByText("中心论点")).toBeTruthy();

    const post = calls.find((c) => c.method === "POST" && c.url === base("/plan/turn"))!;
    expect(post.body).toEqual({ text: "我觉得不该一刀切禁手机。" });
  });

  it("renders the map as a tree, nesting each node under the one above it", async () => {
    routes = {
      ...emptyRoutes(),
      [key("GET", base("/outline"))]: {
        body: {
          outline: [
            { id: "n1", text: "不该一刀切", role: "中心论点", depth: 0, position: 0 },
            { id: "n2", text: "课间需要社交", role: "一条理由", depth: 1, position: 1 },
            { id: "n3", text: "上学期收手机那件事", role: "她自己的经历", depth: 2, position: 2 },
          ],
        },
      },
    };
    render(<WritingRoomHost writingId={WID} />);

    // All three levels are on screen, and the deepest is a DESCENDANT of the
    // shallowest — a flat list would render the text but lose the shape,
    // which is the entire point of the map.
    const root = (await screen.findByText("不该一刀切")).closest("li")!;
    expect(within(root).getByText("课间需要社交")).toBeTruthy();
    expect(within(root).getByText("上学期收手机那件事")).toBeTruthy();
  });

  it("lets her delete a node, and takes its children with it", async () => {
    routes = {
      ...emptyRoutes(),
      [key("GET", base("/outline"))]: {
        body: {
          outline: [
            { id: "n1", text: "不该一刀切", role: "中心论点", depth: 0, position: 0 },
            { id: "n2", text: "课间需要社交", role: "一条理由", depth: 1, position: 1 },
            { id: "n3", text: "上学期那件事", role: "经历", depth: 2, position: 2 },
            { id: "n4", text: "第二条理由", role: "一条理由", depth: 1, position: 3 },
          ],
        },
      },
      [key("PUT", base("/outline"))]: { body: { outline: [] } },
    };
    render(<WritingRoomHost writingId={WID} />);
    fireEvent.click(await screen.findByLabelText("删掉「课间需要社交」"));

    await waitFor(() => {
      const put = calls.find((c) => c.method === "PUT" && c.url === base("/outline"))!;
      // n2 goes, n3 (its child) goes with it, and the unrelated n4 survives.
      expect(put.body).toEqual({
        outline: [
          { text: "不该一刀切", role: "中心论点", depth: 0 },
          { text: "第二条理由", role: "一条理由", depth: 1 },
        ],
      });
    });
  });

  it("lets her leave for the page at any time — planning is never a gate", async () => {
    routes[key("POST", base("/stage"))] = { body: writing(inRoom()) };
    render(<WritingRoomHost writingId={WID} />);
    await screen.findByPlaceholderText("说说你的想法");

    fireEvent.click(screen.getByRole("button", { name: /去写/ }));

    await waitFor(() => expect(screen.getByRole("heading", { name: "段落" })).toBeTruthy());
    const post = calls.find((c) => c.method === "POST" && c.url === base("/stage"))!;
    expect(post.body).toEqual({ stage: "snippets" });
  });
});

// ---------------------------------------------------------------------------
// the room (段落 / 成稿)
// ---------------------------------------------------------------------------

describe("loading the room", () => {
  it("restores the transcript and shows the stage map", async () => {
    routes = { ...emptyRoutes(inRoom()) };
    routes[key("GET", base("/messages"))] = {
      body: { messages: [{ seq: 1, role: "student", content: "我想论证短视频有没有让人变笨。", createdAt: "" }] },
    };
    render(<WritingRoomHost writingId={WID} />);

    expect(await screen.findByText("我想论证短视频有没有让人变笨。")).toBeTruthy();
    expect(screen.getByRole("button", { name: /段落/ })).toHaveAttribute("aria-current", "step");
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
    expect(screen.queryByRole("navigation", { name: "写作三步" })).toBeNull();
    expect(screen.queryByRole("textbox")).toBeNull();
    expect(calls.some((c) => c.url.endsWith("/outline"))).toBe(false);
  });
});

describe("the stage map — a MAP, not a gate", () => {
  it("jumps straight from 段落 to 成稿 with one click, no gating", async () => {
    routes = { ...emptyRoutes(inRoom()) };
    routes[key("POST", base("/stage"))] = { body: writing({ stage: "draft" }) };
    render(<WritingRoomHost writingId={WID} />);
    await screen.findByRole("heading", { name: "段落" });

    fireEvent.click(screen.getByRole("button", { name: /成稿/ }));

    await waitFor(() => expect(screen.getByRole("heading", { name: "成稿" })).toBeTruthy());
    expect(calls.find((c) => c.method === "POST" && c.url === base("/stage"))!.body).toEqual({ stage: "draft" });
  });
});

describe("talk first", () => {
  it("sends a coach turn and shows the reply", async () => {
    routes = { ...emptyRoutes(inRoom()) };
    routes[key("POST", base("/turn"))] = {
      body: { reply: "你想说服谁读这段论证？", decision: "respond", card: null, nudge: "", hintCardId: null },
    };
    render(<WritingRoomHost writingId={WID} />);
    await screen.findByRole("heading", { name: "段落" });

    fireEvent.change(screen.getByPlaceholderText("想到什么，跟印记说说"), { target: { value: "我想写打工的利弊。" } });
    fireEvent.click(screen.getByLabelText("发送"));

    expect(await screen.findByText("你想说服谁读这段论证？")).toBeTruthy();
    expect(calls.find((c) => c.method === "POST" && c.url === base("/turn"))!.body).toEqual({
      text: "我想写打工的利弊。",
    });
  });

  it("surfaces a failed turn instead of a fabricated reply", async () => {
    routes = { ...emptyRoutes(inRoom()) };
    routes[key("POST", base("/turn"))] = {
      status: 502,
      body: { error: { code: "ai_dialogue_failed", message: "印记暂时没接上，请重试。" } },
    };
    render(<WritingRoomHost writingId={WID} />);
    await screen.findByRole("heading", { name: "段落" });

    fireEvent.change(screen.getByPlaceholderText("想到什么，跟印记说说"), { target: { value: "在吗" } });
    fireEvent.click(screen.getByLabelText("发送"));

    expect(await screen.findByText("印记暂时没接上，请重试。")).toBeTruthy();
  });
});

describe("the setup dialog", () => {
  it("asks for language and length once, and stamps setupAt so it never asks again", async () => {
    routes = { ...emptyRoutes({ setupAt: null }) };
    routes[key("PUT", base("/setup"))] = { body: writing({ lang: "en", targetWords: 500 }) };
    render(<WritingRoomHost writingId={WID} />);

    const dialog = await screen.findByRole("dialog", { name: "开始之前" });
    // No 文体 selector: a lite student may not know the word, so the third
    // field is an open box and the model infers genre from it.
    expect(within(dialog).queryByText(/文体/)).toBeNull();

    fireEvent.click(within(dialog).getByRole("button", { name: /English/ }));
    fireEvent.change(within(dialog).getByLabelText("目标字数"), { target: { value: "500" } });
    fireEvent.change(within(dialog).getByLabelText("还想说点什么"), { target: { value: "这是老师布置的作业。" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "开始" }));

    await waitFor(() => {
      const put = calls.find((c) => c.method === "PUT" && c.url === base("/setup"));
      expect(put?.body).toEqual({ lang: "en", targetWords: 500, note: "这是老师布置的作业。" });
    });
    await waitFor(() => expect(screen.queryByRole("dialog", { name: "开始之前" })).toBeNull());
  });

  it("lets her skip it entirely — length is never a precondition", async () => {
    routes = { ...emptyRoutes({ setupAt: null }) };
    routes[key("PUT", base("/setup"))] = { body: writing({}) };
    render(<WritingRoomHost writingId={WID} />);

    const dialog = await screen.findByRole("dialog", { name: "开始之前" });
    fireEvent.click(within(dialog).getByRole("button", { name: "跳过" }));

    await waitFor(() => {
      const put = calls.find((c) => c.method === "PUT" && c.url === base("/setup"));
      expect(put?.body).toEqual({ lang: "zh", targetWords: null, note: "" });
    });
  });
});

describe("目标字数 — visible, or it may as well not exist", () => {
  it("shows the target in the header next to a live count", async () => {
    routes = { ...emptyRoutes(inRoom({ targetWords: 800 })) };
    routes[key("GET", base("/draft"))] = { body: { body: "一二三四五", updatedAt: null } };
    render(<WritingRoomHost writingId={WID} />);
    await screen.findByRole("heading", { name: "段落" });
    expect(screen.getByText(/800/)).toBeTruthy();
    expect(screen.getByText("5")).toBeTruthy();
  });
});

describe("段落 stage — the 铁律 pressure point", () => {
  it("shows a labelled 示范 with NO way to move it into her draft", async () => {
    routes = {
      ...emptyRoutes(inRoom({ lang: "en" })),
      [key("GET", base("/outline"))]: {
        body: { outline: [{ id: "o1", text: "The upside", role: "", depth: 0, position: 0 }] },
      },
    };
    routes[key("PUT", base("/snippets"))] = {
      body: {
        snippets: [{ id: "s1", outlineId: "o1", outlineHeading: "The upside", position: 0, text: "", updatedAt: "" }],
      },
    };
    routes[key("POST", base("/snippets/s1/exemplar"))] = {
      body: {
        exemplar: "Working part-time teaches real accountability.",
        prompts: ["What is your strongest example?"],
      },
    };
    render(<WritingRoomHost writingId={WID} />);
    await screen.findByRole("heading", { name: "段落" });

    fireEvent.click(screen.getByRole("button", { name: "示范段落" }));
    expect(await screen.findByText("Working part-time teaches real accountability.")).toBeTruthy();
    expect(screen.getByText("示范")).toBeTruthy();

    // 铁律, asserted by OUTCOME rather than by inventory: click EVERY node
    // inside the exemplar box and assert her textarea never changes. A
    // queryAllByRole("button") check would miss a <div onClick> or an <a>,
    // which are the two most natural ways someone would later add an
    // "insert this" affordance.
    const box = screen.getByText("Working part-time teaches real accountability.").closest("div")!;
    const draftBox = screen.getByPlaceholderText("写这一段……") as HTMLTextAreaElement;
    const before = draftBox.value;
    for (const node of Array.from(box.querySelectorAll("*"))) fireEvent.click(node);
    fireEvent.click(box);
    expect(draftBox.value).toBe(before);
    expect(draftBox.value).not.toContain("Working part-time teaches real accountability.");
  });

  it("offers no 示范 affordance at all for a Chinese writing", async () => {
    routes = {
      ...emptyRoutes(inRoom({ lang: "zh" })),
      [key("GET", base("/outline"))]: {
        body: { outline: [{ id: "o1", text: "打工的好处", role: "", depth: 0, position: 0 }] },
      },
    };
    render(<WritingRoomHost writingId={WID} />);
    await screen.findByRole("heading", { name: "段落" });
    expect(screen.queryByRole("button", { name: "示范段落" })).toBeNull();
  });

  it("guides a block with questions only, and never touches what she wrote", async () => {
    routes = {
      ...emptyRoutes(inRoom()),
      [key("GET", base("/outline"))]: {
        body: { outline: [{ id: "o1", text: "老师会说影响上课", role: "反方会说的话", depth: 0, position: 0 }] },
      },
      [key("POST", base("/outline/o1/guide"))]: {
        body: { questions: ["支持禁手机的老师最常说的一句话是什么？", "你身边有没有哪件事正好证明了那句话？"] },
      },
    };
    render(<WritingRoomHost writingId={WID} />);
    await screen.findByRole("heading", { name: "段落" });

    const draftBox = screen.getByPlaceholderText("写这一段……") as HTMLTextAreaElement;
    fireEvent.click(screen.getByRole("button", { name: "卡住了？" }));

    expect(await screen.findByText("支持禁手机的老师最常说的一句话是什么？")).toBeTruthy();
    // Guidance must not become content.
    expect(draftBox.value).toBe("");
  });
});

describe("成稿 stage", () => {
  it("composes from her snippets, edits, requests feedback, and finishes", async () => {
    routes = { ...emptyRoutes({ stage: "draft" }) };
    routes[key("POST", base("/compose"))] = { body: { body: "拼合出的初稿。", updatedAt: "2026-08-26T00:00:00Z" } };
    routes[key("PUT", base("/draft"))] = { body: { body: "拼合出的初稿，改过。", updatedAt: "2026-08-26T00:01:00Z" } };
    routes[key("POST", base("/review"))] = { body: { feedback: "论证的第二段证据略薄，可以再补一个例子。" } };
    routes[key("POST", base("/finish"))] = {
      body: writing({ stage: "draft", status: "finished", finishedAt: "2026-08-26T01:00:00Z" }),
    };
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

describe("工具卡 — there are none in this room", () => {
  /**
   * pro's own writing surface barely used the tool cards, and a student stuck
   * on a paragraph wants a question, not a form. The whole surface is gone —
   * deck, shelf, summon, and the guide's nomination.
   */
  it("offers no card surface and never asks the server for one", async () => {
    routes = { ...emptyRoutes(inRoom()) };
    render(<WritingRoomHost writingId={WID} />);
    await screen.findByRole("heading", { name: "段落" });

    expect(screen.queryByRole("button", { name: "工具卡" })).toBeNull();
    expect(screen.queryByText("让步段 · 以退为进")).toBeNull();
    expect(calls.some((c) => c.url.includes("/cards"))).toBe(false);
    expect(calls.some((c) => c.url.includes("/summon"))).toBe(false);
  });
});
