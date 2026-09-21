import { StrictMode } from "react";
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
    [key("GET", base("/comments"))]: { body: { comments: [] } },
    // 印记 speaks first, in both faces of the room.
    [key("POST", base("/opening"))]: { body: { reply: "先说说你自己更倾向哪一边？", generated: true } },
    // FinishedWritingPage's for-atom lookup: `{assignment: null}` is the
    // real "not homework" 200, not a stand-in for an unregistered route —
    // without it a real fetch failure and "no homework" look identical, and
    // the chip stays hidden (Task 10 fix round 1: a thrown error must never
    // be folded into "not homework").
    [key("GET", `/api/v1/lite/assignments/for-atom/${WID}`)]: { body: { assignment: null } },
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
      // 🚨 kind 必须跟着回传（0182）：服务端按它算深度，漏掉就等于把这一行
      // 交还给关键词猜测，她刚做的编辑会被静默改形。
      expect(put.body).toEqual({
        outline: [
          { text: "不该一刀切", role: "中心论点", kind: "thesis", depth: 0 },
          { text: "第二条理由", role: "一条理由", kind: "point", depth: 1 },
        ],
      });
    });
  });

  // 🚨 2026-09-20：「去写」现在先去**行文**（想清楚怎么组织），再去段落。
  // 同事的意见 4：「要在开始写之前先想好整个文章组织框架」。
  // 它仍然不是关卡 —— 顶上那条导航一直点得动，行文那一屏也一步就能过去。
  it("lets her leave for the page at any time — planning is never a gate", async () => {
    routes[key("POST", base("/stage"))] = { body: writing({ ...inRoom(), stage: "flow" }) };
    render(<WritingRoomHost writingId={WID} />);
    await screen.findByPlaceholderText("说说你的想法");

    fireEvent.click(screen.getByRole("button", { name: /去写/ }));

    await waitFor(() => expect(screen.getByRole("heading", { name: "行文" })).toBeTruthy());
    const post = calls.find((c) => c.method === "POST" && c.url === base("/stage"))!;
    expect(post.body).toEqual({ stage: "flow" });
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
    expect(screen.queryByRole("navigation", { name: "写作三步" })).toBeNull();
    expect(calls.some((c) => c.url.endsWith("/outline"))).toBe(false);
    // The piece is no longer duplicated here in a bordered 成稿 card: it is the
    // first page `ReportPanel` shows, set as an article ("don't use card for
    // articles"). This panel is down to the breadcrumb row plus that panel.
    expect(screen.queryByText("这是我的成稿。")).toBeNull();
  });

  // Renaming a finished piece moved with the 2026-09-15 wide finished page
  // (Task 10, FinishedWritingPage): the title there is a plain heading, not
  // an inline EditableTitle — she renames it by pressing 修改, which reopens
  // the room (see WritingRoomHost's `onRevise`), where the title IS
  // EditableTitle again (covered by "the title is hers" below).
  //
  // There is deliberately no component test here for the finished → 修改 →
  // rename path itself (fix round 1 removed one: a heading-present /
  // no-textbox assertion is a negative rendering check, not a behaviour
  // test, and AGENTS.md bans that class of test). The server already
  // refuses a rename on a finished, non-revising writing (Task 4's Go gate
  // tests); the full finished → 修改 → rename flow is checked by Task 13's
  // browser walk instead.
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

    expect(await screen.findByText(/印记暂时没接上，请重试。/)).toBeTruthy();
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
  it("guides a block with questions only, and never touches what she wrote", async () => {
    routes = {
      ...emptyRoutes(inRoom()),
      [key("GET", base("/outline"))]: {
        // 🚨 用例要给 kind，而且要给真的那一个（0182）：一份只有一个「反方
        // 会说的话」的提纲，现在第一张卡是虚拟的开头，那张卡上没有引导按钮 ——
        // 而那不是产品缺陷，是这份用例不像真数据。
        body: {
          outline: [
            { id: "t1", text: "不该一刀切禁手机", role: "中心论点", kind: "thesis", depth: 0, position: 0 },
            { id: "o1", text: "老师会说影响上课", role: "反方观点", kind: "counter", depth: 1, position: 1 },
          ],
        },
      },
      // 第一张卡是开头，它绑在中心论点那个节点上 —— 引导按下去打到的是 t1。
      [key("POST", base("/outline/t1/guide"))]: {
        body: { questions: ["支持禁手机的老师最常说的一句话是什么？", "你身边有没有哪件事正好证明了那句话？"] },
      },
    };
    render(<WritingRoomHost writingId={WID} />);
    await screen.findByRole("heading", { name: "段落" });

    const draftBox = screen.getByPlaceholderText("写这一段……") as HTMLTextAreaElement;
    fireEvent.click(screen.getByRole("button", { name: "获取引导" }));

    expect(await screen.findByText("支持禁手机的老师最常说的一句话是什么？")).toBeTruthy();
    // Guidance must not become content.
    expect(draftBox.value).toBe("");
  });
});

describe("成稿 stage", () => {
  it("lands her on her own assembled text, edits, requests feedback, and finishes", async () => {
    routes = { ...emptyRoutes({ stage: "draft" }) };
    routes[key("GET", base("/snippets"))] = {
      body: {
        snippets: [
          { id: "s1", outlineId: "o1", outlineHeading: "开头", position: 0, text: "夏天路上晒得受不了。", updatedAt: "" },
        ],
      },
    };
    routes[key("POST", base("/compose"))] = { body: { body: "拼合出的初稿。", updatedAt: "2026-08-26T00:00:00Z" } };
    routes[key("PUT", base("/draft"))] = { body: { body: "拼合出的初稿，改过。", updatedAt: "2026-08-26T00:01:00Z" } };
    // POST /review now answers a structured Comment (Task 5/10), not
    // {feedback: prose} — {"comment": Comment} with scope="draft".
    routes[key("POST", base("/review"))] = {
      body: {
        comment: {
          id: "cm1",
          scope: "draft",
          snippetId: null,
          summary: "论证的第二段证据略薄，可以再补一个例子。",
          points: [],
          createdAt: "2026-08-26T00:02:00Z",
        },
      },
    };
    routes[key("POST", base("/finish"))] = {
      body: writing({ stage: "draft", status: "finished", finishedAt: "2026-08-26T01:00:00Z" }),
    };
    render(<WritingRoomHost writingId={WID} />);
    await screen.findByRole("heading", { name: "成稿" });

    // 从段落拼出初稿 is no longer a prerequisite she has to find and press:
    // the draft is assembled the moment she opens 成稿.
    const textarea = await screen.findByDisplayValue("拼合出的初稿。");
    expect(screen.queryByRole("button", { name: "从段落拼出初稿" })).toBeNull();
    fireEvent.change(textarea, { target: { value: "拼合出的初稿，改过。" } });
    fireEvent.blur(textarea);
    await waitFor(() => expect(calls.some((c) => c.method === "PUT" && c.url === base("/draft"))).toBe(true));

    fireEvent.click(screen.getByRole("button", { name: "AI审阅" }));
    expect(await screen.findByText("论证的第二段证据略薄，可以再补一个例子。")).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "完成这篇" }));
    expect(await screen.findByText("已完成")).toBeTruthy();
  });
});

describe("the title is hers", () => {
  /**
   * What sits in the h1 today is her raw idea sentence, carried from the
   * landing box — a note to self, not a title, and until now the one line of
   * the room she could not change.
   */
  it("edits on click, saves on blur, and keeps the new name on screen", async () => {
    routes = { ...emptyRoutes(inRoom()) };
    routes[key("PATCH", base(""))] = { body: writing(inRoom({ title: "行道树该谁来养" })) };
    render(<WritingRoomHost writingId={WID} />);
    await screen.findByRole("heading", { name: "段落" });

    fireEvent.click(screen.getByRole("button", { name: "该不该把上学时间往后推？" }));
    const box = screen.getByLabelText("标题");
    fireEvent.change(box, { target: { value: "行道树该谁来养" } });
    fireEvent.blur(box);

    await waitFor(() => {
      const patch = calls.find((c) => c.method === "PATCH" && c.url === base(""));
      expect(patch?.body).toEqual({ title: "行道树该谁来养" });
    });
    expect(await screen.findByRole("heading", { name: "行道树该谁来养" })).toBeTruthy();
  });

  it("is editable while planning too, not only in the room", async () => {
    render(<WritingRoomHost writingId={WID} />);
    await screen.findByPlaceholderText("说说你的想法");

    fireEvent.click(screen.getByRole("button", { name: "该不该把上学时间往后推？" }));
    expect(screen.getByLabelText("标题")).toBeTruthy();
  });

  it("does not send a blank title the server would reject", async () => {
    routes = { ...emptyRoutes(inRoom()) };
    render(<WritingRoomHost writingId={WID} />);
    await screen.findByRole("heading", { name: "段落" });

    fireEvent.click(screen.getByRole("button", { name: "该不该把上学时间往后推？" }));
    const box = screen.getByLabelText("标题");
    fireEvent.change(box, { target: { value: "   " } });
    fireEvent.blur(box);

    await waitFor(() => expect(screen.getByRole("heading", { name: "该不该把上学时间往后推？" })).toBeTruthy());
    expect(calls.some((c) => c.method === "PATCH")).toBe(false);
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

/**
 * StrictMode — 印记's opening must arrive exactly once, and must arrive.
 *
 * `main.tsx` renders the app inside `<React.StrictMode>`, so in dev React
 * mounts every effect, runs its cleanup, and mounts it again. Two failures
 * sit on either side of that:
 *
 *  - No once-latch → TWO concurrent `POST /opening` calls. The server's
 *    idempotency gate is a read-then-write with no lock, so both bill a model
 *    call and both append an 'ai' row: charged twice, greeted twice on her
 *    next load.
 *  - A once-latch paired with a per-invocation `let cancelled = false` cleanup
 *    flag → the cleanup cancels the closure that fired the call, the latch
 *    skips the remount, and the single real reply is discarded. The room then
 *    waits forever on a request the server already answered. That is what
 *    hung 段落's guide box and reddened writing-walk.spec.ts on 2026-08-28;
 *    write-up in `src/shared/useAlive.ts`.
 *
 * Every other test in this file renders WITHOUT StrictMode — which is exactly
 * why they all stayed green while the live room hung. These two render
 * through it on purpose.
 */
describe("StrictMode — the opening fires once and its reply always lands", () => {
  it("greets her once while planning, on exactly one POST /opening", async () => {
    render(
      <StrictMode>
        <WritingRoomHost writingId={WID} />
      </StrictMode>,
    );

    expect(await screen.findByText("先说说你自己更倾向哪一边？")).toBeTruthy();
    expect(screen.getAllByText("先说说你自己更倾向哪一边？")).toHaveLength(1);
    await waitFor(() =>
      expect(calls.filter((c) => c.method === "POST" && c.url === base("/opening"))).toHaveLength(1),
    );
  });

  it("greets her once in the room proper, on exactly one POST /opening", async () => {
    routes = { ...emptyRoutes(inRoom()) };
    render(
      <StrictMode>
        <WritingRoomHost writingId={WID} />
      </StrictMode>,
    );

    expect(await screen.findByText("先说说你自己更倾向哪一边？")).toBeTruthy();
    await waitFor(() =>
      expect(calls.filter((c) => c.method === "POST" && c.url === base("/opening"))).toHaveLength(1),
    );
  });
});
