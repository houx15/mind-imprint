import { render, screen, fireEvent, waitFor, cleanup } from "@testing-library/react";
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
    render(panel({ initialMessages: [{ seq: 1, role: "ai", content: "开始吧。", createdAt: "" }] }));

    fireEvent.change(screen.getByPlaceholderText("读完这一步，跟印记说一声"), { target: { value: "好" } });
    fireEvent.click(screen.getByLabelText("发送"));

    expect(await screen.findByText("哪一句最能说明作者的态度？")).toBeTruthy();
    expect(screen.getByRole("button", { name: "中国的碳排放总量位居世界第一。" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "但人均排放仍低于多数发达国家。" })).toBeTruthy();
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

  it("shows her choice on the card once she has tapped, and stops offering the options", async () => {
    render(panel({ initialMessages: OPENED }));
    fireEvent.click(screen.getByRole("button", { name: "但人均排放仍低于多数发达国家。" }));

    await waitFor(() => expect(screen.getByText("你选的")).toBeTruthy());
    expect(screen.queryByRole("button", { name: "中国的碳排放总量位居世界第一。" })).toBeNull();
    expect(screen.getByText("“但人均排放仍低于多数发达国家。”")).toBeTruthy();
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

  it("does not answer a card while a lens holds the article", () => {
    render(panel({ initialMessages: OPENED, slot: baseSlot({ locked: true }) }));
    fireEvent.click(screen.getByRole("button", { name: "但人均排放仍低于多数发达国家。" }));
    expect(postTurn).not.toHaveBeenCalled();
  });
});
