import { render, screen, fireEvent, waitFor, cleanup } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ReadingCoachSlot } from "@lite/readings/ReadingRoom";
import type { LiteMessage } from "../src/api/readingRoom";

/**
 * 卡片上的求助：一颗「给点提示」，最多三次，提示长在卡片里。
 *
 * 产品负责人 2026-09-20 报的第 1、3 条：
 *
 *   > 对于一个卡片上的交互也没有进行管理（比如可以就一张卡片一直点提示一下，
 *   > 使得论文阅读流程卡住，无法进行下一步）
 *   > 建议每个卡片的「给点提示」，只限定交互三轮就变灰……
 *   > 「示范一下」这个按钮很容易示范着就把答案示范出去了……不如删去。
 *
 * 这个文件盯的是那三件事：按钮只剩一颗、给满三次置灰、提示显示在卡片里而不是
 * 对话流里（题目和她写了一半的草稿因此留在视野里）。
 */

const postTurn = vi.fn();
vi.mock("../src/api/readingRoom", async (orig) => ({
  ...(await orig<Record<string, unknown>>()),
  postReadingCoachTurn: (...args: unknown[]) => postTurn(...args),
}));

const { ReadingCoachPanel } = await import("../src/readings/ReadingCoachPanel");

const CARD = {
  type: "short_text" as const,
  prompt: "用你自己的话说说，作者为什么先讲「有业」？",
};

const ASSIST = {
  type: "choose_span" as const,
  prompt: "哪一句最接近作者的理由？",
  assist: true,
  options: [
    { blockId: "b1", quote: "但必先有业，才有可敬、可乐的主体，理至易明。" },
    { blockId: "b2", quote: "所以在讲演正文以前，先要说说有业之必要。" },
  ],
};

const OPENED: LiteMessage[] = [
  { seq: 1, role: "ai", content: "我们看第二段。", createdAt: "" },
  { seq: 2, role: "ai", content: "来回答我一个问题。", createdAt: "", payload: { card: CARD } },
];

/** 她按了 n 次提示，印记 每次接了一句。 */
function withHints(n: number): LiteMessage[] {
  const out = [...OPENED];
  for (let i = 1; i <= n; i++) {
    out.push({ seq: 2 + i * 2 - 1, role: "student", content: "给点提示", createdAt: "" });
    out.push({ seq: 2 + i * 2, role: "ai", content: `这是第 ${i} 条提示。`, createdAt: "" });
  }
  return out;
}

function baseSlot(overrides: Partial<ReadingCoachSlot> = {}): ReadingCoachSlot {
  return { locked: false, quotes: [], removeQuote: () => {}, clearQuotes: vi.fn(), ...overrides };
}

function panel(initialMessages: LiteMessage[]) {
  return (
    <ReadingCoachPanel
      readingId="r1"
      tasks={[]}
      initialMessages={initialMessages}
      slot={baseSlot()}
      onTasks={() => {}}
      onFocusBlock={() => {}}
      lensDone={null}
      onLensDoneSent={() => {}}
      onFinish={() => {}}
    />
  );
}

beforeEach(() => {
  postTurn.mockReset();
  postTurn.mockResolvedValue({
    reply: "先看第 2 段的前半句。",
    tasks: [],
    currentTaskId: "",
    focusBlock: "",
    tool: "",
    finished: false,
    card: null,
    nudge: "",
    coachCard: null,
  });
  Element.prototype.scrollIntoView = vi.fn();
});
afterEach(cleanup);

describe("卡片上的求助", () => {
  it("只剩「给点提示」一颗按钮 —— 「示范一下」撤掉了", () => {
    render(panel(OPENED));
    expect(screen.getByRole("button", { name: "给点提示" })).toBeTruthy();
    expect(screen.queryByRole("button", { name: "示范一下" })).toBeNull();
  });

  it("按一下就是一轮，卡片留在原地、草稿不丢", async () => {
    render(panel(OPENED));

    const box = screen.getByRole("textbox", { name: CARD.prompt }) as HTMLTextAreaElement;
    fireEvent.change(box, { target: { value: "我写了一半的那句话" } });

    fireEvent.click(screen.getByRole("button", { name: "给点提示" }));
    await waitFor(() => expect(postTurn).toHaveBeenCalled());
    const [, text] = postTurn.mock.calls[0] as [string, string];
    expect(text).toBe("给点提示");

    // 🚨 题目还在、她写了一半的那句还在。产品负责人：「保留题目及答案草稿」。
    await waitFor(() => expect(screen.getByText("先看第 2 段的前半句。")).toBeTruthy());
    expect(screen.getByText(CARD.prompt)).toBeTruthy();
    expect((screen.getByRole("textbox", { name: CARD.prompt }) as HTMLTextAreaElement).value).toBe(
      "我写了一半的那句话",
    );
  });

  // 🚨 提示是 印记 说的话，要过和气泡同一个 Markdown 渲染器。第一版直接塞文本，
  // 线上截图里她看到的是「回到**第32段**」—— 而 system prompt 明令「每一轮都用
  // 一次加粗」，所以这不是偶发。
  it("提示里的加粗真的渲染成加粗，不是两个星号", () => {
    const bold: LiteMessage[] = [
      ...OPENED,
      { seq: 3, role: "student", content: "给点提示", createdAt: "" },
      { seq: 4, role: "ai", content: "回到**第32段**，看前半句。", createdAt: "" },
    ];
    const { container } = render(panel(bold));
    const strong = container.querySelector('[data-coach-card="open"] strong');
    expect(strong?.textContent).toBe("第32段");
    expect(container.textContent).not.toContain("**");
  });

  it("提示长在卡片里，不在对话流里", () => {
    const { container } = render(panel(withHints(2)));

    expect(screen.getByText("这是第 1 条提示。")).toBeTruthy();
    expect(screen.getByText("这是第 2 条提示。")).toBeTruthy();
    // 两条都在那张卡片里面 —— 不是对话里的两个气泡。
    const card = container.querySelector("[data-coach-card]")!;
    expect(card.textContent).toContain("这是第 1 条提示。");
    expect(card.textContent).toContain("这是第 2 条提示。");
    // 她按下的那句「给点提示」也不占一个气泡：它是一次操作，不是一句话。
    const bubbles = container.querySelectorAll("[data-role='student']");
    for (const b of bubbles) expect(b.textContent).not.toContain("给点提示");
  });

  it("给满三次就置灰", () => {
    render(panel(withHints(3)));
    const button = screen.getByRole("button", { name: "给点提示" }) as HTMLButtonElement;
    expect(button.disabled).toBe(true);
    expect(screen.getByText("提示已用完")).toBeTruthy();
  });

  it("换了一张卡，提示重新数三次", () => {
    const next: LiteMessage[] = [
      ...withHints(3),
      { seq: 40, role: "student", content: "我觉得是因为先要有事做。", createdAt: "" },
      {
        seq: 41,
        role: "ai",
        content: "接着看第 3 段。",
        createdAt: "",
        payload: { card: { type: "short_text" as const, prompt: "第 3 段又加了什么？" } },
      },
    ];
    render(panel(next));
    const buttons = screen.getAllByRole("button", { name: "给点提示" }) as HTMLButtonElement[];
    // 旧那张已替换（折起来，没有求助按钮），屏幕上只剩新卡那一颗，而且是活的。
    expect(buttons).toHaveLength(1);
    expect(buttons[0]!.disabled).toBe(false);
  });
});

describe("辅助题与回到原题", () => {
  const AFTER_ASSIST: LiteMessage[] = [
    ...withHints(3),
    { seq: 20, role: "ai", content: "换个问法。", createdAt: "", payload: { card: ASSIST } },
  ];

  it("辅助题在的时候，原题标成「已替换」，不再收答案", () => {
    render(panel(AFTER_ASSIST));

    expect(screen.getByText("已替换。点击可查看原题。")).toBeTruthy();
    // 原题的输入框不在屏幕上：一个阅读流程同时只有一张卡收答案。
    expect(screen.queryByRole("textbox", { name: CARD.prompt })).toBeNull();
    expect(screen.getByRole("button", { name: ASSIST.options[0]!.quote })).toBeTruthy();
    expect(screen.getByText("这一题换了个问法。答完它会回到上一道题。")).toBeTruthy();
  });

  it("答完辅助题、服务端说回到原题：原题重新收答案，草稿还在", async () => {
    render(panel(AFTER_ASSIST));

    // 先在原题上写一半 —— 展开它只为了拿到那个输入框。
    fireEvent.click(screen.getByRole("button", { name: new RegExp(CARD.prompt) }));
    expect(screen.getByText("已替换")).toBeTruthy();

    postTurn.mockResolvedValueOnce({
      reply: "很接近。回到上面那道题，用你自己的话写一遍。",
      tasks: [],
      currentTaskId: "",
      focusBlock: "",
      tool: "",
      finished: false,
      card: null,
      nudge: "",
      coachCard: null,
      restored: true,
    });
    fireEvent.click(screen.getByRole("button", { name: ASSIST.options[0]!.quote }));

    // 原题回到可作答：输入框回来了。
    await waitFor(() => expect(screen.getByRole("textbox", { name: CARD.prompt })).toBeTruthy());
    expect(screen.queryByText("已替换")).toBeNull();
  });
});
