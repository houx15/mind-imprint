import { render, screen, fireEvent, waitFor, cleanup, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ReadingCoachSlot } from "@lite/readings/ReadingRoom";
import type { LiteMessage } from "../src/api/readingRoom";

/**
 * Task 8 — the wiring: a card 印记 wrote into its reply has to APPEAR in the
 * conversation, a tap has to become a real turn, and a refresh has to bring
 * back the card she already answered rather than a blank one.
 *
 * Everything here is about `ReadingCoachPanel`, not about `CoachCard` (Task 7
 * tested the component on its own). The four things being pinned down are the
 * four places this could quietly not work:
 *
 *   1. the card renders INSIDE the chat log (via `ChatMessage.node`), so it
 *      sits in the conversation where 印记 asked it, not in a rail beside it;
 *   2. a pure tap — nothing typed, nothing quoted — still sends, i.e. it is
 *      not eaten by send()'s own "nothing to send" guard;
 *   3. answering scrolls the log, even though the number of visible messages
 *      does not change (the card grows in place);
 *   4. a reload rebuilds both halves out of `payload`.
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

function baseSlot(overrides: Partial<ReadingCoachSlot> = {}): ReadingCoachSlot {
  return {
    locked: false,
    quotes: [],
    removeQuote: () => {},
    clearQuotes: vi.fn(),
    ...overrides,
  };
}

function panel(props: {
  initialMessages: LiteMessage[];
  slot?: ReadingCoachSlot;
}) {
  return (
    <ReadingCoachPanel
      readingId="r1"
      tasks={[]}
      initialMessages={props.initialMessages}
      slot={props.slot ?? baseSlot()}
      onTasks={() => {}}
      onFocusBlock={() => {}}
      lensDone={null}
      onLensDoneSent={() => {}}
      onFinish={() => {}}
    />
  );
}

let scrolled: Element[];

beforeEach(() => {
  postTurn.mockReset();
  postTurn.mockResolvedValue({
    reply: "好，接着说。",
    tasks: [],
    currentTaskId: "",
    focusBlock: "",
    tool: "",
    finished: false,
    card: null,
    nudge: "",
    coachCard: null,
  });
  scrolled = [];
  // jsdom has no scrollIntoView at all; recording WHICH element scrolled is
  // what lets the scroll assertion below be about this panel's own anchor
  // rather than about ChatLog's (which moves for its own reasons).
  Element.prototype.scrollIntoView = function scrollIntoView(this: Element) {
    scrolled.push(this);
  };
});

afterEach(cleanup);

describe("ReadingCoachPanel — the card in the conversation", () => {
  it("renders the card 印记 wrote into a turn, in the log, right under its words", async () => {
    postTurn.mockResolvedValue({
      reply: "读完这一段，来回答我一个问题。",
      tasks: [],
      currentTaskId: "",
      focusBlock: "",
      tool: "",
      finished: false,
      card: null,
      nudge: "",
      coachCard: CARD,
    });
    const { container } = render(
      panel({ initialMessages: [{ seq: 1, role: "ai", content: "开始吧。", createdAt: "" }] }),
    );

    fireEvent.change(screen.getByPlaceholderText("请输入你的回答或问题"), { target: { value: "好" } });
    fireEvent.click(screen.getByLabelText("发送"));

    expect(await screen.findByText("哪一句最能说明作者的态度？")).toBeTruthy();

    // 🚨 位置就是 Task 8 的核心设计主张：卡片长在对话里 印记 问它的那个地方，
    // 不是渲染在日志上方的某条 rail 里。断言「prompt 和按钮存在于某处」对后者
    // 一样成立——所以这里查的是 (a) 卡片在日志容器内，(b) 它就排在说那句话的
    // 那条 印记 消息**的下一行**。
    const log = container.querySelector("[data-coach-log]") as HTMLElement;
    expect(log).toBeTruthy();
    const card = log.querySelector(".mk-coachcard") as HTMLElement;
    expect(card).toBeTruthy();
    expect(within(log).getByRole("button", { name: "中国的碳排放总量位居世界第一。" })).toBeTruthy();
    expect(within(log).getByRole("button", { name: "但人均排放仍低于多数发达国家。" })).toBeTruthy();

    // ChatBubble renders a card-carrying message with `data-role="system"` ON
    // the row itself, so that element IS the card's row inside ChatLog.
    const cardRow = card.closest("[data-role='system']")!;
    const rows = [...cardRow.parentElement!.children];
    const spokenRow = rows.find((r) => r !== cardRow && r.textContent?.includes("读完这一段，来回答我一个问题。"));
    expect(spokenRow, "印记 说的那句话应该和卡片在同一个日志里").toBeTruthy();
    expect(rows.indexOf(cardRow)).toBe(rows.indexOf(spokenRow!) + 1);
  });

  it("restores a card that arrived in an earlier session out of the message payload", () => {
    render(panel({ initialMessages: OPENED }));
    expect(screen.getByText("哪一句最能说明作者的态度？")).toBeTruthy();
    expect(screen.getByRole("button", { name: "中国的碳排放总量位居世界第一。" })).toBeTruthy();
  });

  it("a tap sends a turn carrying cardAnswer — with nothing typed and nothing quoted", async () => {
    render(panel({ initialMessages: OPENED }));

    fireEvent.click(screen.getByRole("button", { name: "但人均排放仍低于多数发达国家。" }));

    await waitFor(() => expect(postTurn).toHaveBeenCalled());
    const [id, text, picks, cardAnswer] = postTurn.mock.calls[0] as [string, string, unknown, unknown];
    expect(id).toBe("r1");
    // The composer was empty and no sentence was quoted: without the guard
    // knowing about cards, this turn never leaves the browser.
    expect(text).toBe("");
    expect(picks).toEqual([]);
    // Handed over EXACTLY as CoachCard produced it — the server checks
    // `choice` against the article as a literal substring, so a trim or a
    // reshape here is what makes 「她指了」 stop counting.
    expect(cardAnswer).toEqual({
      type: "choose_span",
      prompt: "哪一句最能说明作者的态度？",
      choice: "但人均排放仍低于多数发达国家。",
      blockId: "b2",
    });
  });

  // 🚨 2026-09-17 反了一半：答完之后**选项仍然看得见**（她回头得知道自己是在
  // 什么里面选的，而且 印记 下一轮讲的往往就是几个选项之间的差别），但它们
  // 不再是按钮。见 CoachCard 的 AnsweredOptions。
  it("shows her choice on the card once she has tapped, and stops the options being tappable", async () => {
    render(panel({ initialMessages: OPENED }));
    fireEvent.click(screen.getByRole("button", { name: "但人均排放仍低于多数发达国家。" }));

    await waitFor(() => expect(screen.getByText("你选的")).toBeTruthy());
    // 没被选的那一句还在屏幕上……
    expect(screen.getByText("中国的碳排放总量位居世界第一。")).toBeTruthy();
    // ……但点不动了。
    expect(screen.queryByRole("button", { name: "中国的碳排放总量位居世界第一。" })).toBeNull();
    expect(screen.getByText("但人均排放仍低于多数发达国家。")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "但人均排放仍低于多数发达国家。" })).toBeNull();
  });

  it("scrolls the log when the card is answered, even though no new message is visible", async () => {
    // 🚨 The turn is deliberately left IN FLIGHT. Once 印记's reply lands it
    // adds a row and everything scrolls for that reason instead — which would
    // make this test pass while the thing it is about stays broken. The window
    // being pinned down is exactly the one where her answer is on screen and
    // the reply is not: the card has grown in place, the number of visible
    // messages has not changed, and nothing else is going to scroll.
    postTurn.mockReturnValue(new Promise(() => {}));
    render(panel({ initialMessages: OPENED }));
    scrolled = [];

    fireEvent.click(screen.getByRole("button", { name: "但人均排放仍低于多数发达国家。" }));

    await waitFor(() => expect(screen.getByText("你选的")).toBeTruthy());
    expect(scrolled.some((el) => el.getAttribute("data-scroll-anchor") === "coach-end")).toBe(true);
  });

  it("a reload shows the ANSWERED card, not a blank turn and not the composed transcript line", () => {
    // Exactly what the server stores: the card on 印记's message, her answer
    // on hers, and her message's content already composed with every quoted
    // line carrying "> " (composeCardAnswerMessage).
    const answered: LiteMessage[] = [
      ...OPENED,
      {
        seq: 3,
        role: "student",
        content: "> 【印记问】哪一句最能说明作者的态度？\n> 但人均排放仍低于多数发达国家。",
        createdAt: "",
        payload: {
          answer: {
            type: "choose_span",
            prompt: "哪一句最能说明作者的态度？",
            choice: "但人均排放仍低于多数发达国家。",
            blockId: "b2",
          },
        },
      },
      { seq: 4, role: "ai", content: "好，接着说。", createdAt: "" },
    ];
    render(panel({ initialMessages: answered }));

    // The card is there AND it is answered.
    expect(screen.getByText("哪一句最能说明作者的态度？")).toBeTruthy();
    expect(screen.getByText("你选的")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "但人均排放仍低于多数发达国家。" })).toBeNull();
    // …and her stored message is NOT also dumped into the log as raw
    // markdown-quoted text: the card above IS that message.
    expect(screen.queryByText(/【印记问】/)).toBeNull();
  });

  it("keeps her own words visible when she typed alongside a tap", () => {
    const answered: LiteMessage[] = [
      ...OPENED,
      {
        seq: 3,
        role: "student",
        content: "> 【印记问】哪一句最能说明作者的态度？\n> 但人均排放仍低于多数发达国家。\n\n我觉得他在替中国说话。",
        createdAt: "",
        payload: {
          answer: {
            type: "choose_span",
            prompt: "哪一句最能说明作者的态度？",
            choice: "但人均排放仍低于多数发达国家。",
            blockId: "b2",
          },
        },
      },
    ];
    render(panel({ initialMessages: answered }));
    expect(screen.getByText("我觉得他在替中国说话。")).toBeTruthy();
    expect(screen.queryByText(/【印记问】/)).toBeNull();
  });

  /**
   * 🚨 `short_text` 的 choice 就是她自己写的那句话（服务端 `case choice != "":
   * own = choice`——那些字必须裸着进她的语料）。卡片已经在显示它了，日志再把
   * `ownWords()` 活下来的东西渲染一遍，她刷新之后就会看见同一句话两次。
   *
   * 在一个主张「你的想法很珍贵」的房间里，这是最不该出现的那种 glitch。
   */
  describe("a short_text answer survives a reload exactly ONCE", () => {
    const HERS = "他只算了成本，没算住在那儿的人。";
    const SHORT_CARD = { type: "short_text" as const, prompt: "用你自己的话说说，作者漏掉了什么？" };

    function reloaded(content: string): LiteMessage[] {
      return [
        { seq: 1, role: "ai", content: "说说你的看法。", createdAt: "", payload: { card: SHORT_CARD } },
        {
          seq: 2,
          role: "student",
          content,
          createdAt: "",
          payload: { answer: { type: "short_text", prompt: SHORT_CARD.prompt, choice: HERS } },
        },
      ];
    }

    it("她写的那一句只出现一次，不是两次", () => {
      // 服务端存下来的原样：问题在 `> ` 行里，她的句子裸着。
      render(panel({ initialMessages: reloaded(`> 【印记问】${SHORT_CARD.prompt}\n\n${HERS}`) }));

      expect(screen.getAllByText(HERS)).toHaveLength(1);
      expect(screen.getByText("你写的")).toBeTruthy();
    });

    it("她一边写卡片一边又打了字：句子一次，她补的话一次", () => {
      const typed = "而且他只看了一个城市。";
      render(panel({ initialMessages: reloaded(`> 【印记问】${SHORT_CARD.prompt}\n\n${HERS}\n\n${typed}`) }));

      expect(screen.getAllByText(HERS)).toHaveLength(1);
      expect(screen.getAllByText(typed)).toHaveLength(1);
    });
  });

  /**
   * 铁律③ 一次只问一个。触发路径毫不刁钻：印记 给一张卡 → 她不理，直接打字 →
   * 印记 又给一张。两个同时敞开的提问，她得先猜房间到底要哪一个。
   *
   * 裁定是**折叠**旧的，不是置灰：一排死掉的灰色 UI 读起来就是「这是你没做完的
   * 所有事」，计分板的情绪从后门溜进来了。
   */
  describe("only one card is open at a time", () => {
    const SECOND = {
      type: "choose_span" as const,
      prompt: "那你觉得他最想让你相信哪一句？",
      options: [
        { blockId: "b3", quote: "减排的速度已经超过了多数人的预期。" },
        { blockId: "b4", quote: "但代价落在了谁头上，文章没有说。" },
      ],
    };
    const TWO_OPEN: LiteMessage[] = [
      ...OPENED,
      { seq: 3, role: "student", content: "我先说点别的。", createdAt: "" },
      { seq: 4, role: "ai", content: "行，那换一个问题。", createdAt: "", payload: { card: SECOND } },
    ];

    it("older unanswered cards collapse to their question; only the newest is answerable", () => {
      render(panel({ initialMessages: TWO_OPEN }));

      // 最新那张完整敞开
      expect(screen.getByRole("button", { name: "减排的速度已经超过了多数人的预期。" })).toBeTruthy();
      // 旧的那张只剩问题文字——它的选项不在屏幕上
      expect(screen.queryByRole("button", { name: "中国的碳排放总量位居世界第一。" })).toBeNull();
      expect(screen.queryByRole("button", { name: "但人均排放仍低于多数发达国家。" })).toBeNull();
      // 但 印记 确实问过这句话，记录还在
      expect(screen.getByText(new RegExp("哪一句最能说明作者的态度？"))).toBeTruthy();
    });

    it("she can tap the collapsed card open again and answer it — the room never closes it FOR her", async () => {
      render(panel({ initialMessages: TWO_OPEN }));

      fireEvent.click(screen.getByRole("button", { name: /哪一句最能说明作者的态度？/ }));
      fireEvent.click(screen.getByRole("button", { name: "但人均排放仍低于多数发达国家。" }));

      await waitFor(() => expect(postTurn).toHaveBeenCalled());
      const [, , , cardAnswer] = postTurn.mock.calls[0] as [string, string, unknown, unknown];
      // 配对循环按 prompt 认卡：答的是旧那张，不是最新那张。
      expect(cardAnswer).toEqual({
        type: "choose_span",
        prompt: "哪一句最能说明作者的态度？",
        choice: "但人均排放仍低于多数发达国家。",
        blockId: "b2",
      });
    });
  });

  it("routes a pick_in_article card: the sentence she pointed at in the article becomes the answer", async () => {
    const opened: LiteMessage[] = [
      {
        seq: 1,
        role: "ai",
        content: "回文章里找一句。",
        createdAt: "",
        payload: { card: { type: "pick_in_article", prompt: "哪一句让你改了主意？" } },
      },
    ];
    const slot = baseSlot({ quotes: [{ key: "q1", quote: "但人均排放仍低于多数发达国家。", blockId: "b2" }] });
    render(panel({ initialMessages: opened, slot }));

    fireEvent.click(screen.getByLabelText("发送"));

    await waitFor(() => expect(postTurn).toHaveBeenCalled());
    const [, text, picks, cardAnswer] = postTurn.mock.calls[0] as [string, string, unknown, unknown];
    expect(cardAnswer).toEqual({
      type: "pick_in_article",
      prompt: "哪一句让你改了主意？",
      choice: "但人均排放仍低于多数发达国家。",
      blockId: "b2",
    });
    // The server re-derives both the transcript line and the pick from
    // `choice`; sending them again would put the sentence in twice.
    expect(text).toBe("");
    expect(picks).toEqual([]);
  });

  it("still refuses a turn with nothing in it at all, card or no card", () => {
    render(panel({ initialMessages: OPENED }));
    fireEvent.click(screen.getByLabelText("发送"));
    expect(postTurn).not.toHaveBeenCalled();
  });

  /**
   * `coachCardOf` 声明的不变式是「半张卡片不许当成真卡片渲染」。一个
   * `choose_span` 如果选项被过滤光了，它就是一个**没法回答**的问题：屏幕上一个
   * 提问，没有任何作答的路，也没有出口。这不是难看，是死路。
   */
  it("a choose_span whose options were all filtered away is not rendered as a card at all", () => {
    render(
      panel({
        initialMessages: [
          {
            seq: 1,
            role: "ai",
            content: "读完这一段，来回答我一个问题。",
            createdAt: "",
            payload: {
              card: {
                type: "choose_span",
                prompt: "哪一句最能说明作者的态度？",
                // 服务端发来的选项全是空串 → 过滤之后什么都不剩。
                options: [
                  { blockId: "b1", quote: "" },
                  { blockId: "b2", quote: "" },
                ],
              },
            },
          },
        ],
      }),
    );

    // 印记 说的话还在，但没有卡片——一个没法回答的提问不许上屏。
    expect(screen.getByText("读完这一段，来回答我一个问题。")).toBeTruthy();
    expect(screen.queryAllByText("哪一句最能说明作者的态度？")).toHaveLength(0);
    expect(document.querySelector(".mk-coachcard")).toBeNull();
  });

  it("does not answer a card while a lens holds the article", () => {
    render(panel({ initialMessages: OPENED, slot: baseSlot({ locked: true }) }));
    fireEvent.click(screen.getByRole("button", { name: "但人均排放仍低于多数发达国家。" }));
    expect(postTurn).not.toHaveBeenCalled();
  });
});

/**
 * R2 — 对着真实走查的截图（`.deploy-local/card-eyeball-2026-08-29/`）逐条改的
 * 三件事。全都不是「代码读起来不对」，而是「屏幕上看起来不对」。
 */
describe("R2 — 屏幕上看得见的三件事", () => {
  const WITH_HER: LiteMessage[] = [
    { seq: 1, role: "ai", content: "我们看第二段。", createdAt: "" },
    { seq: 2, role: "student", content: "我读完了。", createdAt: "" },
    { seq: 3, role: "ai", content: "读完这一段，来回答我一个问题。", createdAt: "", payload: { card: CARD } },
  ];

  /**
   * 产品负责人对着截图说的：*"currently in the box ai's chat box and task card
   * is not very clear. task card. AI avatar is necessary."*
   *
   * 印记 说的话要一眼认得出是**它在说话**，卡片要一眼认得出是**要她动手的
   * 东西**。前者靠头像，后者靠 DOM 上一个稳定的钩子撑起来的另一种外观。
   */
  it("印记 的每条消息都挂着它的头像，她自己的不挂", () => {
    const { container } = render(panel({ initialMessages: WITH_HER }));
    const log = container.querySelector("[data-coach-log]") as HTMLElement;

    const his = [...log.querySelectorAll('[data-chat-row="ai"]')];
    expect(his.length).toBeGreaterThan(0);
    for (const row of his) expect(row.querySelector("svg.mk-pebble")).toBeTruthy();

    const hers = [...log.querySelectorAll('[data-chat-row="student"]')];
    expect(hers.length).toBeGreaterThan(0);
    for (const row of hers) expect(row.querySelector("svg.mk-pebble")).toBeNull();
  });

  it("卡片和聊天气泡在 DOM 上分得开——卡片不是一个气泡", () => {
    const { container } = render(panel({ initialMessages: WITH_HER }));
    const log = container.querySelector("[data-coach-log]") as HTMLElement;

    const card = log.querySelector("[data-coach-card]") as HTMLElement;
    expect(card).toBeTruthy();
    // 卡片没有长在任何一个气泡里：气泡是「谁在说话」，卡片是「该你动手了」。
    expect(card.closest("[data-role='assistant']")).toBeNull();
    expect(card.closest("[data-role='student']")).toBeNull();
    for (const bubble of log.querySelectorAll("[data-role='assistant'],[data-role='student']")) {
      expect(bubble.querySelector("[data-coach-card]")).toBeNull();
    }
  });

  /**
   * 截图 `05-turn1-card-AFTER-tap.png`：她答掉当前这张，原本折起来的旧卡片
   * **自己弹开了**（因为它成了「最新的未答卡片」），下一条回复到达时又折回去。
   *
   * 折叠状态要按「她是不是已经往下走了」定，不按「是不是最新的未答卡片」定。
   */
  it("答掉当前这张之后，早就折起来的旧卡片不会自己弹开", async () => {
    const SECOND = {
      type: "choose_span" as const,
      prompt: "那你觉得他最想让你相信哪一句？",
      options: [
        { blockId: "b3", quote: "减排的速度已经超过了多数人的预期。" },
        { blockId: "b4", quote: "但代价落在了谁头上，文章没有说。" },
      ],
    };
    const two: LiteMessage[] = [
      ...OPENED,
      { seq: 3, role: "student", content: "我先说点别的。", createdAt: "" },
      { seq: 4, role: "ai", content: "行，那换一个问题。", createdAt: "", payload: { card: SECOND } },
    ];
    render(panel({ initialMessages: two }));

    expect(screen.queryByRole("button", { name: "中国的碳排放总量位居世界第一。" })).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "减排的速度已经超过了多数人的预期。" }));
    await waitFor(() => expect(screen.getByText("你选的")).toBeTruthy());

    // 旧那张还是折着的——它没有因为「现在轮到它是最新的未答卡片」就跳出来。
    expect(screen.queryByRole("button", { name: "中国的碳排放总量位居世界第一。" })).toBeNull();
    expect(screen.queryByRole("button", { name: "但人均排放仍低于多数发达国家。" })).toBeNull();

    // 🚨 但门始终开着：她点一下就能回去答（铁律②，房间不替她关死）。
    fireEvent.click(screen.getByRole("button", { name: /哪一句最能说明作者的态度？/ }));
    expect(screen.getByRole("button", { name: "但人均排放仍低于多数发达国家。" })).toBeTruthy();
  });

  /**
   * 真实走查里中过一次：coach 返回不可解析 → 502 → catch 把乐观消息移除，而那条
   * 消息**带着她的 `payload.answer`**。卡片恢复成未答，选项全部回来，没有任何
   * 地方记得她刚才点的是哪一句——她得自己猜。
   */
  describe("一次 502 不许把她点过的答案偷偷取消", () => {
    it("她选的那一句还在卡片上，并且能一键重试", async () => {
      postTurn.mockRejectedValueOnce(new Error("boom"));
      render(panel({ initialMessages: OPENED }));

      fireEvent.click(screen.getByRole("button", { name: "但人均排放仍低于多数发达国家。" }));

      const again = await screen.findByRole("button", { name: "重试" });
      // 她的选择还在、还标着——不用她重新回忆点过什么。卡片没有恢复成未答：
      // 一条都不再是按钮。
      expect(screen.getByText("你选的")).toBeTruthy();
      expect(screen.getByText("但人均排放仍低于多数发达国家。")).toBeTruthy();
      expect(screen.queryByRole("button", { name: "但人均排放仍低于多数发达国家。" })).toBeNull();
      expect(screen.queryByRole("button", { name: "中国的碳排放总量位居世界第一。" })).toBeNull();

      fireEvent.click(again);

      await waitFor(() => expect(postTurn).toHaveBeenCalledTimes(2));
      const [, , , cardAnswer] = postTurn.mock.calls[1] as [string, string, unknown, unknown];
      expect(cardAnswer).toEqual({
        type: "choose_span",
        prompt: "哪一句最能说明作者的态度？",
        choice: "但人均排放仍低于多数发达国家。",
        blockId: "b2",
      });
      await waitFor(() => expect(screen.queryByRole("button", { name: "重试" })).toBeNull());
      expect(screen.getByText("你选的")).toBeTruthy();
    });

    it("只是打了字的那一轮失败了，字回到输入框里（她还能改）", async () => {
      postTurn.mockRejectedValueOnce(new Error("boom"));
      render(panel({ initialMessages: OPENED }));

      const box = screen.getByPlaceholderText("请输入你的回答或问题") as HTMLTextAreaElement;
      fireEvent.change(box, { target: { value: "我觉得他在替中国说话。" } });
      fireEvent.click(screen.getByLabelText("发送"));

      await waitFor(() => expect(box.value).toBe("我觉得他在替中国说话。"));
    });
  });
});
