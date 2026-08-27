import { render, screen, waitFor, cleanup, fireEvent, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { WritingRoomHost } from "@lite/writings/WritingRoomHost";

/**
 * WritingRoomHost — driven through a stubbed `fetch`, same harness shape as
 * readingRoomHost.test.tsx. There is no pro room to mount here (see the task
 * brief's 复用边界), so these tests exercise the assembled room directly:
 * the stage map, the setup dialog, the coach's opening line, each stage's own
 * panel, and the guiding box.
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
    // 结构, the first of the three steps since 2026-08-27 (构思 was retired
    // into the setup dialog + the coach's opening line).
    stage: "outline",
    targetWords: null,
    structureKey: "",
    // Non-null so the setup dialog does NOT gate these tests. Its own
    // behaviour is covered by "the setup dialog" below, which sets it null.
    setupAt: "2026-08-26T00:00:00Z",
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
    // The coach speaks first now. Every room load with no AI turn yet fires
    // this once, so it belongs in the baseline rather than in each test.
    [key("POST", base("/opening"))]: { body: { reply: "先说说你自己更倾向哪一边？", generated: true } },
    [key("GET", "/api/v1/writings/structures?lang=zh")]: { body: { structures: STRUCTURES_ZH } },
    [key("GET", "/api/v1/writings/structures?lang=en")]: { body: { structures: [] } },
  };
}

/** A trimmed stand-in for the server's fixed library — labels only, exactly
 *  as the real one is: no block carries content. */
const STRUCTURES_ZH = [
  {
    key: "zh-argument-concession",
    lang: "zh",
    name: "让步式议论",
    blurb: "对方也有道理，硬顶反而站不住。",
    blocks: [
      { role: "你的立场", hint: "一句话说清楚你站哪一边。" },
      { role: "反方最强的说法", hint: "找对方最难反驳的那条。" },
    ],
  },
  {
    key: "zh-narrative",
    lang: "zh",
    name: "记叙文",
    blurb: "你想讲一件真实发生过的事。",
    blocks: [{ role: "事情发生前", hint: "当时的你在意什么？" }],
  },
];

beforeEach(() => {
  routes = emptyRoutes();
  stubFetch();
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("loading the room", () => {
  it("restores the transcript and shows the stage map on 结构", async () => {
    routes[key("GET", base("/messages"))] = {
      body: { messages: [{ seq: 1, role: "student", content: "我想论证短视频有没有让人变笨。", createdAt: "" }] },
    };
    render(<WritingRoomHost writingId={WID} />);

    expect(await screen.findByText("我想论证短视频有没有让人变笨。")).toBeTruthy();
    expect(screen.getByRole("button", { name: /结构/ })).toHaveAttribute("aria-current", "step");
    expect(screen.getByRole("heading", { name: "结构" })).toBeTruthy();
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
    // The room's own loads are never even attempted for a finished writing.
    expect(calls.some((c) => c.url.endsWith("/outline"))).toBe(false);
  });
});

describe("the stage map — a MAP, not a gate", () => {
  it("jumps straight from 结构 to 成稿 with one click, no gating", async () => {
    routes[key("POST", base("/stage"))] = { body: writing({ stage: "draft" }) };
    render(<WritingRoomHost writingId={WID} />);
    await screen.findByRole("heading", { name: "结构" });

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
    await screen.findByRole("heading", { name: "结构" });

    fireEvent.change(screen.getByPlaceholderText("想到什么，跟印记说说"), { target: { value: "我想写打工的利弊。" } });
    fireEvent.click(screen.getByLabelText("发送"));

    expect(await screen.findByText("你想说服谁读这段论证？")).toBeTruthy();
    const post = calls.find((c) => c.method === "POST" && c.url === base("/turn"))!;
    expect(post.body).toEqual({ text: "我想写打工的利弊。" });
  });

  it("surfaces a failed turn instead of a fabricated reply", async () => {
    routes[key("POST", base("/turn"))] = { status: 502, body: { error: { code: "ai_dialogue_failed", message: "印记暂时没接上，请重试。" } } };
    render(<WritingRoomHost writingId={WID} />);
    await screen.findByRole("heading", { name: "结构" });

    fireEvent.change(screen.getByPlaceholderText("想到什么，跟印记说说"), { target: { value: "在吗" } });
    fireEvent.click(screen.getByLabelText("发送"));

    expect(await screen.findByText("印记暂时没接上，请重试。")).toBeTruthy();
  });
});

describe("结构 stage — the AI picks a skeleton, it never writes an outline", () => {
  /**
   * The regression this whole describe exists to prevent. Until 2026-08-27
   * this stage had 「帮我拟一份候选」: her transcript went to the model and a
   * finished outline came back, which she then "edited". That is the AI doing
   * the thinking. It is gone, and these tests fail if it returns.
   */
  it("has no outline-generation affordance anywhere on the page", async () => {
    render(<WritingRoomHost writingId={WID} />);
    await screen.findByRole("heading", { name: "结构" });

    expect(screen.queryByRole("button", { name: "帮我拟一份候选" })).toBeNull();
    expect(screen.queryByText(/拟一份/)).toBeNull();
    expect(calls.some((c) => c.url.includes("/outline/generate"))).toBe(false);
  });

  it("recommends a skeleton from the library, persisting nothing until she accepts", async () => {
    routes[key("POST", base("/structure/recommend"))] = {
      body: { structureKey: "zh-argument-concession", reason: "你两边都有理由，让步式放得下反方。" },
    };
    routes[key("POST", base("/structure"))] = {
      body: {
        structureKey: "zh-argument-concession",
        outline: [
          { id: "o1", text: "", role: "你的立场", depth: 0, position: 0 },
          { id: "o2", text: "", role: "反方最强的说法", depth: 0, position: 1 },
        ],
      },
    };
    render(<WritingRoomHost writingId={WID} />);
    await screen.findByRole("heading", { name: "结构" });

    fireEvent.click(screen.getByRole("button", { name: "帮我挑一副" }));
    expect(await screen.findByText("你两边都有理由，让步式放得下反方。")).toBeTruthy();

    // Recommending must not lay out an outline behind her back.
    expect(calls.some((c) => c.method === "POST" && c.url === base("/structure"))).toBe(false);

    fireEvent.click(screen.getByRole("button", { name: "就用这一副" }));

    await waitFor(() => {
      const post = calls.find((c) => c.method === "POST" && c.url === base("/structure"));
      expect(post?.body).toEqual({ structureKey: "zh-argument-concession", force: false });
    });
  });

  it("lays out labelled blocks whose text fields are EMPTY — 铁律①", async () => {
    routes = {
      ...emptyRoutes({ stage: "outline", structureKey: "zh-argument-concession" }),
      [key("GET", base("/outline"))]: {
        body: {
          outline: [
            { id: "o1", text: "", role: "你的立场", depth: 0, position: 0 },
            { id: "o2", text: "", role: "反方最强的说法", depth: 0, position: 1 },
          ],
        },
      },
    };
    render(<WritingRoomHost writingId={WID} />);
    await screen.findByRole("heading", { name: "结构" });

    // The label shows up more than once on purpose — on her block, and again
    // in the shelf's preview chips for the skeleton she is using.
    expect((await screen.findAllByText("反方最强的说法")).length).toBeGreaterThan(0);

    // The label is present as TEXT and NOT as an editable field: if she could
    // type over it, the template and her thinking would stop being
    // distinguishable in the data too, and that distinction is what the
    // process report reads.
    const hers = screen.getByLabelText("反方最强的说法") as HTMLInputElement;
    expect(hers.value).toBe("");
    expect(screen.queryByDisplayValue("反方最强的说法")).toBeNull();
  });

  it("echoes role back on save so editing her text cannot wipe the block labels", async () => {
    routes = {
      ...emptyRoutes({ stage: "outline", structureKey: "zh-argument-concession" }),
      [key("GET", base("/outline"))]: {
        body: { outline: [{ id: "o1", text: "", role: "你的立场", depth: 0, position: 0 }] },
      },
      [key("PUT", base("/outline"))]: {
        body: { outline: [{ id: "o1", text: "我反对一刀切", role: "你的立场", depth: 0, position: 0 }] },
      },
    };
    render(<WritingRoomHost writingId={WID} />);
    const field = (await screen.findByLabelText("你的立场")) as HTMLInputElement;

    fireEvent.change(field, { target: { value: "我反对一刀切" } });
    fireEvent.blur(field);

    await waitFor(() => {
      const put = calls.find((c) => c.method === "PUT" && c.url === base("/outline"));
      expect(put?.body).toEqual({ outline: [{ text: "我反对一刀切", role: "你的立场", depth: 0 }] });
    });
  });

  it("still shows her blocks when the structure library cannot be read", async () => {
    // Silent data loss, the class of bug this codebase keeps re-learning: her
    // rows exist and carry their own `role`, so a failed library lookup must
    // cost her the skeleton's NAME and HINTS — never the sight of her own
    // sentences.
    routes = {
      ...emptyRoutes({ stage: "outline", structureKey: "zh-argument-concession" }),
      [key("GET", base("/outline"))]: {
        body: { outline: [{ id: "o1", text: "我反对一刀切", role: "你的立场", depth: 0, position: 0 }] },
      },
    };
    delete routes[key("GET", "/api/v1/writings/structures?lang=zh")];

    render(<WritingRoomHost writingId={WID} />);
    await screen.findByRole("heading", { name: "结构" });

    expect(await screen.findByDisplayValue("我反对一刀切")).toBeTruthy();
    expect(screen.getByLabelText("你的立场")).toBeTruthy();
  });

  it("guides her on a BLOCK too, with questions and never a sentence", async () => {
    // "AI should guide me to think about the outlines" — the guiding box has
    // to reach the outline, not only the paragraphs. Working out what
    // 「反方最强的说法」 means for her topic is where she stalls, and it is
    // upstream of every paragraph she writes after it.
    routes = {
      ...emptyRoutes({ stage: "outline", structureKey: "zh-argument-concession" }),
      [key("GET", base("/outline"))]: {
        body: { outline: [{ id: "o2", text: "", role: "反方最强的说法", depth: 0, position: 0 }] },
      },
      [key("POST", base("/outline/o2/guide"))]: {
        body: {
          questions: ["支持禁手机的老师最常说的一句话是什么？", "你身边有没有哪件事，正好证明了他们那句话？"],
          cardId: "",
          cardReason: "",
        },
      },
    };
    render(<WritingRoomHost writingId={WID} />);
    await screen.findByRole("heading", { name: "结构" });

    fireEvent.click(screen.getByRole("button", { name: "想不出来？" }));

    expect(await screen.findByText("支持禁手机的老师最常说的一句话是什么？")).toBeTruthy();
    expect(screen.getByText("你身边有没有哪件事，正好证明了他们那句话？")).toBeTruthy();

    // Guidance must not become content: her field is still empty, and nothing
    // in the guide box can put anything into it.
    expect((screen.getByLabelText("反方最强的说法") as HTMLInputElement).value).toBe("");
  });
});

describe("the coach speaks first", () => {
  it("greets her on open without her having to say anything", async () => {
    render(<WritingRoomHost writingId={WID} />);
    expect(await screen.findByText("先说说你自己更倾向哪一边？")).toBeTruthy();
    expect(calls.some((c) => c.method === "POST" && c.url === base("/opening"))).toBe(true);
  });

  it("does not ask for an opening when 印记 has already spoken", async () => {
    routes[key("GET", base("/messages"))] = {
      body: {
        messages: [
          { seq: 1, role: "student", content: "我想写手机。", createdAt: "" },
          { seq: 2, role: "ai", content: "你更倾向哪一边？", createdAt: "" },
        ],
      },
    };
    render(<WritingRoomHost writingId={WID} />);
    await screen.findByText("你更倾向哪一边？");
    expect(calls.some((c) => c.method === "POST" && c.url === base("/opening"))).toBe(false);
  });
});

describe("the setup dialog", () => {
  it("asks for language and length once, and stamps setupAt so it never asks again", async () => {
    routes = { ...emptyRoutes({ setupAt: null }) };
    routes[key("PUT", base("/setup"))] = { body: writing({ lang: "en", targetWords: 500 }) };
    render(<WritingRoomHost writingId={WID} />);

    const dialog = await screen.findByRole("dialog", { name: "开始之前" });
    // No 文体 selector: a lite student may not know the word, so the third
    // field is an open box instead and the model infers genre from it.
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
    routes = { ...emptyRoutes({ targetWords: 800 }) };
    routes[key("GET", base("/draft"))] = { body: { body: "一二三四五", updatedAt: null } };
    render(<WritingRoomHost writingId={WID} />);
    await screen.findByRole("heading", { name: "结构" });
    expect(screen.getByText(/800/)).toBeTruthy();
    expect(screen.getByText("5")).toBeTruthy();
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

describe("工具卡 — there are none in this room", () => {
  /**
   * 2026-08-27 product call: pro's own writing surface barely used the tool
   * cards, and a student stuck on a paragraph wants a question, not a form.
   * The whole surface is gone — deck, shelf, summon, and the guide's
   * nomination. These assertions are what stop it drifting back in.
   */
  it("offers no card surface anywhere in the room", async () => {
    render(<WritingRoomHost writingId={WID} />);
    await screen.findByRole("heading", { name: "结构" });

    expect(screen.queryByRole("button", { name: "工具卡" })).toBeNull();
    expect(screen.queryByText("让步段 · 以退为进")).toBeNull();
    expect(screen.queryByText(/要不要用《/)).toBeNull();
  });

  it("never asks the server for cards", async () => {
    render(<WritingRoomHost writingId={WID} />);
    await screen.findByRole("heading", { name: "结构" });
    expect(calls.some((c) => c.url.includes("/cards"))).toBe(false);
    expect(calls.some((c) => c.url.includes("/summon"))).toBe(false);
  });

  it("renders a guide as questions only, with nothing to open", async () => {
    routes = {
      ...emptyRoutes({ stage: "outline", structureKey: "zh-argument-concession" }),
      [key("GET", base("/outline"))]: {
        body: { outline: [{ id: "o1", text: "", role: "你的立场", depth: 0, position: 0 }] },
      },
      [key("POST", base("/outline/o1/guide"))]: {
        body: { questions: ["你自己更倾向哪一边？"] },
      },
    };
    render(<WritingRoomHost writingId={WID} />);
    await screen.findByRole("heading", { name: "结构" });

    fireEvent.click(screen.getByRole("button", { name: "想不出来？" }));
    expect(await screen.findByText("你自己更倾向哪一边？")).toBeTruthy();

    // No card offer can appear alongside the questions.
    expect(screen.queryByText(/这张卡/)).toBeNull();
  });
});
