import { render, screen, fireEvent, waitFor, cleanup } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ReadingCoachSlot } from "@lite/readings/ReadingRoom";
import type { LiteMessage } from "../src/api/readingRoom";

/**
 * 一副透镜开着的时候，这一栏还能不能说话。
 *
 * 产品负责人 2026-09-17，逐字：
 *
 *   > 没搞懂各种透镜加入的逻辑，这个 ai 在我选的一篇论证读书态度的议论文里面
 *   > 加了伦理学的透镜，我找不到句子，句子匹配不通过 …
 *   > 找不到句子的时候，没法在聊天框打字求助 ai。
 *
 * 前半句是透镜挑得不对（那一半靠提示词和「她说没有这种句子就先信她」那条规矩），
 * 后半句是**产品把她关在了外面**：输入框在透镜开着时是锁死的，屏幕上唯一还能
 * 动的东西是「跳过」。她没有办法说出「这篇里没有这种句子」—— 而她是对的。
 *
 * 所以这里钉三件事：
 *
 *  1. 透镜开着，输入框**能用**。
 *  2. 但旧卡片仍然点不动 —— 铁律③，一次只问一个。
 *  3. 上一轮还在飞的时候仍然锁着（`locked` 收窄成这一个意思，没有被删掉）。
 */

const postTurn = vi.fn();
vi.mock("../src/api/readingRoom", async (orig) => ({
  ...(await orig<Record<string, unknown>>()),
  postReadingCoachTurn: (...args: unknown[]) => postTurn(...args),
}));

const { ReadingCoachPanel } = await import("../src/readings/ReadingCoachPanel");

const CARD = {
  type: "choose_span" as const,
  prompt: "哪一句最能说明作者的态度？",
  options: [
    { blockId: "b1", quote: "中国的碳排放总量位居世界第一。" },
    { blockId: "b2", quote: "但人均排放仍低于多数发达国家。" },
  ],
};

const OPENED: LiteMessage[] = [
  { seq: 1, role: "ai", content: "我们看第二段。", createdAt: "" },
  { seq: 2, role: "ai", content: "读完这一段，来回答我一个问题。", createdAt: "", payload: { card: CARD } },
];

function slot(overrides: Partial<ReadingCoachSlot> = {}): ReadingCoachSlot {
  return {
    locked: false,
    quotes: [],
    removeQuote: () => {},
    clearQuotes: vi.fn(),
    ...overrides,
  };
}

function panel(s: ReadingCoachSlot) {
  return (
    <ReadingCoachPanel
      readingId="r1"
      tasks={[]}
      initialMessages={OPENED}
      slot={s}
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
    reply: "这篇里确实没有那种句子，我们换一副。",
    tasks: [],
    currentTaskId: "",
    focusBlock: "",
    tool: "",
    finished: false,
    card: null,
    nudge: "",
    coachCard: null,
  });
  Element.prototype.scrollIntoView = function scrollIntoView() {};
});

afterEach(cleanup);

describe("透镜开着的时候", () => {
  it("她仍然能打字求助，而且那一轮真的发出去了", async () => {
    render(panel(slot({ lensOpen: true, locateLens: () => {} })));

    const box = screen.getByPlaceholderText("找不到合适的句子？跟印记说一声") as HTMLTextAreaElement;
    expect(box.disabled).toBe(false);
    fireEvent.change(box, { target: { value: "这篇文章里没有这种句子。" } });
    fireEvent.click(screen.getByLabelText("发送"));

    await waitFor(() => expect(postTurn).toHaveBeenCalledTimes(1));
    const [, text] = postTurn.mock.calls[0] as [string, string];
    expect(text).toBe("这篇文章里没有这种句子。");
  });

  it("横幅说得出这一步在文章里做，而且不提屏幕的方位", () => {
    render(panel(slot({ lensOpen: true, locateLens: () => {} })));
    const banner = screen.getByText(/这一步要在文章里做/);
    expect(banner.textContent).toContain("找不到合适的句子");
    // 🚨 一个方位词都不许有：文章在桌面端和手机上排的位置不一样。
    for (const word of ["左边", "右边", "上面", "下面"]) {
      expect(banner.textContent?.includes(word)).toBe(false);
    }
  });

  // 铁律③：透镜和卡片都是把这一步交回她手上，两个一起能动，她第一件要做的事
  // 就变成了「先做哪个」。服务端那一侧也拦着（reading_coach.go 的 anyOpen）。
  it("旧卡片点不动", () => {
    render(panel(slot({ lensOpen: true, locateLens: () => {} })));
    const option = screen.getByRole("button", {
      name: "但人均排放仍低于多数发达国家。",
    }) as HTMLButtonElement;
    expect(option.disabled).toBe(true);
    fireEvent.click(option);
    expect(postTurn).not.toHaveBeenCalled();
  });

  // `locked` 没有被删掉，只是收窄了：上一轮还在飞的时候照旧不让发第二轮。
  it("上一轮还在飞的时候仍然锁着", () => {
    render(panel(slot({ locked: true })));
    const box = screen.getByPlaceholderText("请输入你的回答或问题") as HTMLTextAreaElement;
    fireEvent.change(box, { target: { value: "我读完了。" } });
    fireEvent.click(screen.getByLabelText("发送"));
    expect(postTurn).not.toHaveBeenCalled();
  });
});
